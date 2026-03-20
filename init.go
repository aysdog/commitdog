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
	if c.Token == "" {
		fatal("no GitHub token found. run 'commitdog setup' first.")
	}

	fmt.Printf("  connecting to GitHub...")
	username, err := getGitHubUsername(c.Token)
	if err != nil {
		fmt.Println()
		fatal("could not connect to GitHub: %v\nrun 'commitdog setup' to update your token.", err)
	}
	fmt.Printf("\n  ✓ connected as %s\n\n", username)

	orgs, _ := getGitHubOrgs(c.Token)

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
	var repo repoResponse

	if owner == username {
		repo, err = createPersonalRepo(c.Token, repoName, private)
	} else {
		repo, err = createOrgRepo(c.Token, owner, repoName, private)
	}

	if err != nil {
		fmt.Println()
		errMsg := err.Error()
		if strings.Contains(errMsg, "Repository creation failed") {
			fatal("failed to create repo: check that your token has 'write:org' scope for org repos.\n  get a new token at https://github.com/settings/tokens")
		}
		if strings.Contains(errMsg, "already exists") {
			fatal("repo %s/%s already exists on GitHub.", owner, repoName)
		}
		fatal("failed to create repo: %v", err)
	}
	fmt.Printf("\n  ✓ repo created: %s\n\n", repo.HTMLURL)

	if err := gitInit(); err != nil {
		fatal("git init failed: %v", err)
	}

	remoteURL := repo.SSHURL
	if !hasSSHKey() {
		remoteURL = repo.HTMLURL + ".git"
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

	fmt.Printf("\n  pushing to GitHub...")
	if err := runPush("origin", "main"); err != nil {
		if err2 := runPush("origin", "master"); err2 != nil {
			fmt.Printf("\n  push failed: %s\n", err)
			return
		}
	}

	fmt.Printf("\n  ✓ pushed\n")
	fmt.Printf("\n  live at %s\n\n", repo.HTMLURL)
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
