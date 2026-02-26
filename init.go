package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// runInit handles 'commitdog init' — creates a GitHub repo and does the first push.
func runInit() {
	fmt.Println()
	fmt.Println("  commitdog init")
	fmt.Println()

	// load config — need token
	c := loadConfig()
	if c.Token == "" {
		fatal("no GitHub token found. run 'commitdog setup' first.")
	}

	// verify token works by fetching username
	fmt.Printf("  connecting to GitHub...")
	username, err := getGitHubUsername(c.Token)
	if err != nil {
		fmt.Println()
		fatal("could not connect to GitHub: %v\nrun 'commitdog setup' to update your token.", err)
	}
	fmt.Printf("\n  ✓ connected as %s\n\n", username)

	// --- personal or org? ---
	fmt.Printf("  push to personal or org? [P/o] › ")
	var isOrg bool
	var orgName string

	for {
		input := strings.ToLower(sanitizeInput(readLine()))
		if input == "" || input == "p" {
			isOrg = false
			break
		}
		if input == "o" {
			isOrg = true
			break
		}
		fmt.Printf("  P or o › ")
	}

	if isOrg {
		fmt.Printf("  org name › ")
		for {
			input := sanitizeInput(readLine())
			if input == "" {
				fmt.Printf("  org name cannot be empty › ")
				continue
			}
			if !isSafeGitRef(input) {
				fmt.Printf("  invalid org name › ")
				continue
			}
			orgName = input
			break
		}
	}

	// --- repo name ---
	defaultName := filepath.Base(getCurrentDir())
	fmt.Printf("  repo name [%s] › ", defaultName)
	repoName := sanitizeInput(readLine())
	if repoName == "" {
		repoName = defaultName
	}
	if !isSafeRepoName(repoName) {
		fatal("invalid repo name: %s", repoName)
	}

	// --- private or public? ---
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

	// --- create repo on GitHub ---
	fmt.Printf("\n  creating repo...")
	var repo repoResponse

	if isOrg {
		repo, err = createOrgRepo(c.Token, orgName, repoName, private)
	} else {
		repo, err = createPersonalRepo(c.Token, repoName, private)
	}

	if err != nil {
		fmt.Println()
		fatal("failed to create repo: %v", err)
	}
	fmt.Printf("\n  ✓ repo created: %s\n\n", repo.HTMLURL)

	// --- git init ---
	if err := gitInit(); err != nil {
		fatal("git init failed: %v", err)
	}

	// --- set remote ---
	// use SSH if key is available, otherwise HTTPS
	remoteURL := repo.SSHURL
	if !hasSSHKey() {
		remoteURL = repo.HTMLURL + ".git"
	}

	if err := gitSetRemote("origin", remoteURL); err != nil {
		fatal("failed to set remote: %v", err)
	}
	fmt.Printf("  ✓ remote set to %s\n", remoteURL)

	// --- stage all files ---
	if err := gitAddAll(); err != nil {
		fatal("git add failed: %v", err)
	}

	// --- check if there's anything to commit ---
	diff, _ := getStagedDiff()
	if diff == "" {
		fmt.Println("  nothing to commit. add some files first.")
		return
	}

	// --- suggest commit message ---
	analysis := analyzeDiff(diff)
	suggestions := generateSuggestions(analysis)

	// override with sensible init defaults if analysis is weak
	if len(suggestions) == 0 || suggestions[0] == "" {
		suggestions = []string{
			"feat: initial commit",
			"chore: initial project setup",
		}
	}

	chosen := pickSuggestion(suggestions)
	if chosen == "" {
		fmt.Println("  aborted.")
		return
	}

	// --- commit ---
	if err := runCommit(chosen); err != nil {
		fatal("commit failed: %v", err)
	}
	fmt.Printf("\n  ✓ committed: %s\n", chosen)

	// --- push ---
	fmt.Printf("\n  pushing to GitHub...")
	if err := runPush("origin", "main"); err != nil {
		// try master if main fails
		if err2 := runPush("origin", "master"); err2 != nil {
			fmt.Printf("\n  push failed: %s\n", err)
			return
		}
	}

	fmt.Printf("\n  ✓ pushed\n")
	fmt.Printf("\n  live at %s\n\n", repo.HTMLURL)
}

// getCurrentDir returns the current working directory name.
func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "my-project"
	}
	return dir
}

// isSafeRepoName validates a GitHub repo name.
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
