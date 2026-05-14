package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func runInit() {
	fmt.Println()
	fmt.Println("  commitdog init")
	fmt.Println()

	c := loadConfig()
	proj := loadProjectConfig()
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
