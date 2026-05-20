package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runInit() {
	fmt.Println()
	fmt.Println("  commitdog init")
	fmt.Println()

	c := loadConfig()
	proj := loadProjectConfig()

	if proj.effectivePrimary() != "" {
		handleConfiguredRepo(proj, c)
		return
	}

	platform := proj.platform

	if platform == "" {
		fmt.Println("  which platform is this repo on?")
		fmt.Println()
		fmt.Println("  1  github")
		fmt.Println("  2  gitlab")
		fmt.Println("  3  gitea")
		fmt.Println("  4  forgejo")
		fmt.Println()
		fmt.Printf("  [1/2/3/4] pick › ")
		for {
			switch strings.TrimSpace(readLine()) {
			case "1":
				platform = "github"
			case "2":
				platform = "gitlab"
			case "3":
				platform = "gitea"
			case "4":
				platform = "forgejo"
			default:
				fmt.Printf("  [1/2/3/4] pick › ")
				continue
			}
			break
		}
		fmt.Println()
		proj.platform = platform
		_ = saveProjectConfig(proj)
	}

	token := tokenForPlatform(c, platform)
	if token == "" {
		fatal("no token found for %s. run 'commitdog setup' first.", platform)
	}

	fmt.Printf("  connecting to %s...", platform)
	username, err := platformUsername(c, platform)
	if err != nil {
		fmt.Println()
		fatal("could not connect to %s: %v\nrun 'commitdog setup' to update your token.", platform, err)
	}
	fmt.Printf("\n  ✓ connected as %s\n\n", username)

	orgs, _ := platformOrgs(c, platform)

	destinations := []string{username + " (personal)"}
	for _, o := range orgs {
		destinations = append(destinations, o)
	}

	var owner string
	if len(destinations) == 1 {
		owner = username
		fmt.Printf("  creating under %s\n\n", username)
	} else {
		fmt.Println("  where to create the repo?")
		for i, d := range destinations {
			fmt.Printf("  %d  %s\n", i+1, d)
		}
		fmt.Println()
		fmt.Printf("  [1-%d] pick › ", len(destinations))
		for {
			input := strings.TrimSpace(readLine())
			idx := 0
			for _, r := range input {
				if r >= '1' && r <= '9' {
					idx = int(r-'0') - 1
					break
				}
			}
			if idx >= 0 && idx < len(destinations) {
				if idx == 0 {
					owner = username
				} else {
					owner = orgs[idx-1]
				}
				break
			}
			fmt.Printf("  [1-%d] pick › ", len(destinations))
		}
	}

	defaultName := filepath.Base(getCurrentDir())
	fmt.Printf("  repo name [%s] › ", defaultName)
	repoName := sanitizeInput(readLine())
	if repoName == "" {
		repoName = defaultName
	}
	if !isSafeRepoName(repoName) {
		fatal("invalid repo name: %s", repoName)
	}

	fmt.Printf("  private or public? [P/u] › ")
	private := true
	for {
		input := strings.ToLower(sanitizeInput(readLine()))
		if input == "" || input == "p" {
			private = true
			break
		}
		if input == "u" {
			private = false
			break
		}
		fmt.Printf("  P or u › ")
	}

	fmt.Printf("\n  creating repo...")
	repo, err := platformCreateRepo(c, platform, owner, username, repoName, private)
	if err != nil {
		fmt.Println()
		errMsg := err.Error()
		if strings.Contains(errMsg, "already exists") {
			fatal("repo %s/%s already exists on %s.", owner, repoName, platform)
		}
		fatal("failed to create repo: %v", err)
	}
	fmt.Printf("\n  ✓ repo created: %s\n\n", repo.HTMLURL)

	if err := gitInit(); err != nil {
		fatal("git init failed: %v", err)
	}

	remoteURL := repo.SSHURL
	if !hasSSHKey(sshHostForPlatform(c, platform)) {
		remoteURL = strings.TrimSuffix(repo.HTMLURL, ".git") + ".git"
	}

	if err := gitSetRemote("origin", remoteURL); err != nil {
		fatal("failed to set remote: %v", err)
	}
	fmt.Printf("  ✓ remote set to %s\n", remoteURL)

	if err := gitAddAll(); err != nil {
		fatal("git add failed: %v", err)
	}

	diff, _ := getStagedDiff()
	if diff == "" {
		fmt.Println("  nothing to commit. add some files first.")
		return
	}

	suggestions := []string{
		"feat: initial commit",
		"chore: initial project setup",
		"init: bootstrap " + repoName,
	}

	chosen := pickSuggestion(suggestions)
	if chosen == "" {
		fmt.Println("  aborted.")
		return
	}

	if err := runCommit(chosen); err != nil {
		fatal("commit failed: %v", err)
	}
	fmt.Printf("\n  ✓ committed: %s\n", chosen)

	fmt.Printf("\n  pushing...")
	authHeader := authHeaderForPlatform(token)
	if err := runPushWithAuth("origin", "main", authHeader); err != nil {
		if err2 := runPushWithAuth("origin", "master", authHeader); err2 != nil {
			fmt.Printf("\n  push failed: %s\n", err)
			return
		}
	}

	fmt.Printf("\n  ✓ pushed\n")
	fmt.Printf("\n  live at %s\n\n", repo.HTMLURL)
}

func platformUsername(c config, platform string) (string, error) {
	token := tokenForPlatform(c, platform)
	switch platform {
	case "gitlab":
		return getGitLabUsername(token, gitlabHost(c))
	case "gitea":
		return getGiteaUsername(token, c.Gitea.Host)
	case "forgejo":
		return getForgejoUsername(token, c.Forgejo.Host)
	default:
		return getGitHubUsername(token)
	}
}

func platformOrgs(c config, platform string) ([]string, error) {
	token := tokenForPlatform(c, platform)
	switch platform {
	case "gitlab":
		return getGitLabGroups(token, gitlabHost(c))
	case "gitea":
		return getGiteaOrgs(token, c.Gitea.Host)
	case "forgejo":
		return getForgejoOrgs(token, c.Forgejo.Host)
	default:
		return getGitHubOrgs(token)
	}
}

func platformCreateRepo(c config, platform, owner, username, name string, private bool) (repoResponse, error) {
	token := tokenForPlatform(c, platform)
	switch platform {
	case "gitlab":
		host := gitlabHost(c)
		if owner == username {
			return createGitLabRepo(token, host, name, private)
		}
		return createGitLabGroupRepo(token, host, owner, name, private)
	case "gitea":
		if owner == username {
			return createGiteaRepo(token, c.Gitea.Host, name, private)
		}
		return createGiteaOrgRepo(token, c.Gitea.Host, owner, name, private)
	case "forgejo":
		if owner == username {
			return createForgejoRepo(token, c.Forgejo.Host, name, private)
		}
		return createForgejoOrgRepo(token, c.Forgejo.Host, owner, name, private)
	default:
		if owner == username {
			return createPersonalRepo(token, name, private)
		}
		return createOrgRepo(token, owner, name, private)
	}
}

func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "my-project"
	}
	return dir
}

func isSafeRepoName(s string) bool {
	if s == "" || len(s) > 100 {
		return false
	}
	for _, c := range s {
		if !isAlphanumeric(c) && c != '-' && c != '_' && c != '.' {
			return false
		}
	}
	return true
}

func handleConfiguredRepo(proj projectConfig, c config) {
	primary := proj.effectivePrimary()
	fmt.Println()
	fmt.Printf("  this repo is configured for %s.\n", primary)
	if len(proj.mirrors) > 0 {
		fmt.Printf("  mirrors: %s\n", strings.Join(proj.mirrors, ", "))
	}
	fmt.Println()
	fmt.Println("  1  change platform")
	fmt.Println("  2  add mirror")
	fmt.Println("  3  cancel")
	fmt.Println()
	fmt.Printf("  [1/2/3] pick › ")

	switch strings.TrimSpace(readLine()) {
	case "1":
		changePrimaryPlatform(proj)
	case "2":
		addMirrorPlatform(proj, c)
	default:
		fmt.Println("  cancelled.")
	}
}

func changePrimaryPlatform(proj projectConfig) {
	all := []string{"github", "gitlab", "gitea", "forgejo"}
	fmt.Println()
	fmt.Println("  select new primary platform:")
	fmt.Println()
	for i, p := range all {
		fmt.Printf("  %d  %s\n", i+1, p)
	}
	fmt.Println()
	fmt.Printf("  [1-%d] pick, [q] quit › ", len(all))

	for {
		input := strings.TrimSpace(readLine())
		if input == "q" || input == "" {
			fmt.Println("  cancelled.")
			return
		}
		for i, newPrimary := range all {
			if input == fmt.Sprintf("%d", i+1) {
				oldPrimary := proj.effectivePrimary()
				proj.primary = newPrimary
				proj.platform = newPrimary
				if oldPrimary != "" && oldPrimary != newPrimary {
					alreadyMirror := false
					for _, m := range proj.mirrors {
						if m == oldPrimary {
							alreadyMirror = true
							break
						}
					}
					if !alreadyMirror {
						proj.mirrors = append(proj.mirrors, oldPrimary)
					}
				}
				var filtered []string
				for _, m := range proj.mirrors {
					if m != newPrimary {
						filtered = append(filtered, m)
					}
				}
				proj.mirrors = filtered
				if err := saveProjectConfig(proj); err != nil {
					fatal("could not update .commitdog: %v", err)
				}
				newRemote := platformRemoteName(newPrimary)
				if newRemote != "origin" && remoteExists(newRemote) {
					cmd := exec.Command("git", "remote", "get-url", newRemote)
					var out strings.Builder
					cmd.Stdout = &out
					if err := cmd.Run(); err == nil {
						newURL := strings.TrimSpace(out.String())
						exec.Command("git", "remote", "set-url", "origin", newURL).Run()
						fmt.Printf("  ✓ origin updated to %s\n", newURL)
					}
				}
				fmt.Printf("  ✓ primary platform changed to %s\n\n", newPrimary)
				return
			}
		}
		fmt.Printf("  1-%d or q › ", len(all))
	}
}

func addMirrorPlatform(proj projectConfig, c config) {
	primary := proj.effectivePrimary()
	all := []string{"github", "gitlab", "gitea", "forgejo"}

	var available []string
	for _, p := range all {
		if p == primary {
			continue
		}
		already := false
		for _, m := range proj.mirrors {
			if m == p {
				already = true
				break
			}
		}
		if !already {
			available = append(available, p)
		}
	}

	if len(available) == 0 {
		fmt.Println("  all platforms are already configured.")
		return
	}

	fmt.Println()
	fmt.Println("  add which platform as mirror?")
	fmt.Println()
	for i, p := range available {
		fmt.Printf("  %d  %s\n", i+1, p)
	}
	fmt.Println()
	fmt.Printf("  [1-%d] pick, [q] quit › ", len(available))

	var mirrorPlatform string
	for {
		input := strings.TrimSpace(readLine())
		if input == "q" || input == "" {
			fmt.Println("  cancelled.")
			return
		}
		for i, p := range available {
			if input == fmt.Sprintf("%d", i+1) {
				mirrorPlatform = p
				break
			}
		}
		if mirrorPlatform != "" {
			break
		}
		fmt.Printf("  1-%d or q › ", len(available))
	}

	token := tokenForPlatform(c, mirrorPlatform)
	if token == "" {
		fatal("no token for %s. run 'commitdog setup' first.", mirrorPlatform)
	}

	fmt.Printf("  connecting to %s...", mirrorPlatform)
	username, err := platformUsername(c, mirrorPlatform)
	if err != nil {
		fmt.Println()
		fatal("could not connect to %s: %v", mirrorPlatform, err)
	}
	fmt.Printf("\n  ✓ connected as %s\n\n", username)

	_, currentRepo := getRepoOwnerAndName()
	fmt.Printf("  repo name [%s] › ", currentRepo)
	input := sanitizeInput(readLine())
	if input != "" {
		currentRepo = input
	}

	fmt.Printf("  private or public? [P/u] › ")
	private := true
	for {
		inp := strings.ToLower(sanitizeInput(readLine()))
		if inp == "" || inp == "p" {
			private = true
			break
		}
		if inp == "u" {
			private = false
			break
		}
		fmt.Printf("  P or u › ")
	}

	fmt.Printf("\n  creating repo on %s...", mirrorPlatform)
	repo, err := platformCreateRepo(c, mirrorPlatform, username, username, currentRepo, private)
	var remoteURL string
	if err != nil {
		fmt.Println()
		errMsg := strings.ToLower(err.Error())
		if strings.Contains(errMsg, "already exists") || strings.Contains(errMsg, "exists") || strings.Contains(errMsg, "repository creation failed") || strings.Contains(errMsg, "name already") {
			fmt.Printf("  repo '%s' already exists on %s.\n\n", currentRepo, mirrorPlatform)
			fmt.Println("  1  use existing repo as mirror")
			fmt.Println("  2  choose a different name")
			fmt.Println("  3  cancel")
			fmt.Println()
			fmt.Printf("  [1/2/3] pick › ")
			switch strings.TrimSpace(readLine()) {
			case "1":
				fmt.Printf("  fetching repo details from %s...", mirrorPlatform)
				fetched, ferr := platformCreateRepo(c, mirrorPlatform, username, username, currentRepo, private)
				if ferr == nil && fetched.HTMLURL != "" {
					fmt.Println()
					fmt.Printf("  found: %s\n", fetched.HTMLURL)
					fmt.Printf("  use this repo? [Y/n] › ")
					if ans := strings.ToLower(strings.TrimSpace(readLine())); ans != "n" && ans != "no" {
						remoteURL = fetched.SSHURL
						if !hasSSHKey(sshHostForPlatform(c, mirrorPlatform)) {
							remoteURL = strings.TrimSuffix(fetched.HTMLURL, ".git") + ".git"
						}
					} else {
						remoteURL = buildMirrorURL(c, mirrorPlatform, username, currentRepo)
					}
				} else {
					fmt.Println()
					remoteURL = buildMirrorURL(c, mirrorPlatform, username, currentRepo)
				}
				fmt.Printf("  using %s\n\n", remoteURL)
			case "2":
				fmt.Printf("  new repo name › ")
				newName := sanitizeInput(readLine())
				if newName == "" || !isSafeRepoName(newName) {
					fmt.Println("  invalid name.")
					return
				}
				currentRepo = newName
				fmt.Printf("  creating repo...")
				repo, err = platformCreateRepo(c, mirrorPlatform, username, username, currentRepo, private)
				if err != nil {
					fmt.Println()
					fatal("failed to create repo: %v", err)
				}
				fmt.Printf("\n  ✓ repo created: %s\n\n", repo.HTMLURL)
				remoteURL = repo.SSHURL
				if !hasSSHKey(sshHostForPlatform(c, mirrorPlatform)) {
					remoteURL = strings.TrimSuffix(repo.HTMLURL, ".git") + ".git"
				}
			default:
				fmt.Println("  cancelled.")
				return
			}
		} else {
			fatal("failed to create repo: %v", err)
		}
	} else {
		fmt.Printf("\n  ✓ repo created: %s\n\n", repo.HTMLURL)
		remoteURL = repo.SSHURL
		if !hasSSHKey(sshHostForPlatform(c, mirrorPlatform)) {
			remoteURL = strings.TrimSuffix(repo.HTMLURL, ".git") + ".git"
		}
	}

	remoteName := platformRemoteName(mirrorPlatform)
	if err := gitAddRemote(remoteName, remoteURL); err != nil {
		fmt.Printf("  warning: could not add remote: %v\n", err)
	} else {
		fmt.Printf("  ✓ remote '%s' added\n", remoteName)
	}

	branch := getCurrentBranch()
	authHeader := authHeaderForPlatformName(mirrorPlatform)
	fmt.Printf("  pushing to %s...", mirrorPlatform)
	if err := runPushUpstreamWithAuth(remoteName, branch, authHeader); err != nil {
		fmt.Printf("\n  push failed: %v\n", err)
	} else {
		fmt.Printf("\n  ✓ pushed\n")
	}

	proj.mirrors = append(proj.mirrors, mirrorPlatform)
	if proj.primary == "" {
		proj.primary = primary
	}
	if err := saveProjectConfig(proj); err != nil {
		fmt.Printf("  warning: could not update .commitdog: %v\n", err)
	} else {
		fmt.Printf("  ✓ .commitdog updated\n\n")
	}
}

func buildMirrorURL(c config, platform, username, repo string) string {
	switch platform {
	case "gitlab":
		host := gitlabHost(c)
		return strings.TrimRight(host, "/") + "/" + username + "/" + repo + ".git"
	case "gitea":
		return strings.TrimRight(c.Gitea.Host, "/") + "/" + username + "/" + repo + ".git"
	case "forgejo":
		return strings.TrimRight(c.Forgejo.Host, "/") + "/" + username + "/" + repo + ".git"
	default:
		return "https://github.com/" + username + "/" + repo + ".git"
	}
}
