package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type commitEntry struct {
	hash    string
	subject string
	date    string
}

func runRevert() {
	if err := verifyGitRepo(); err != nil {
		fatal("not a git repository.")
	}

	commits, err := getRecentCommits(5)
	if err != nil {
		fatal("could not read git log: %v", err)
	}
	if len(commits) == 0 {
		fatal("no commits found in this repository.")
	}

	fmt.Println()
	fmt.Println("  recent commits:")
	fmt.Println()

	for i, c := range commits {
		fmt.Printf("  %d  %s  %s  %s\n",
			i+1,
			colorDim(c.hash),
			c.subject,
			colorMuted("("+c.date+")"),
		)
	}

	fmt.Println()
	fmt.Printf("  [1-%d] pick, [e] enter hash, [q] quit › ", len(commits))

	var chosen string

	for {
		input := readLine()

		switch input {
		case "q", "quit", "exit":
			fmt.Println("  aborted.")
			return
		case "e", "edit":
			chosen = askForHash()
			if chosen == "" {
				return
			}
			goto revert
		}

		for i, c := range commits {
			if input == fmt.Sprintf("%d", i+1) {
				chosen = c.hash
				goto revert
			}
		}

		fmt.Printf("  enter 1-%d, e, or q › ", len(commits))
	}

revert:
	chosen = strings.TrimSpace(chosen)
	chosen = strings.ToLower(chosen)
	if !isSafeHash(chosen) {
		fatal("invalid commit hash: %q", chosen)
	}

	subject := subjectForHash(chosen, commits)
	fmt.Println()
	if subject != "" {
		fmt.Printf("  reverting %s — %s\n", chosen[:7], subject)
	} else {
		fmt.Printf("  reverting %s\n", chosen[:7])
	}

	fmt.Printf("  ⚠  this creates a new revert commit. continue? [Y/n] › ")
	confirm := readLine()
	if confirm == "n" || confirm == "no" {
		fmt.Println("  aborted.")
		return
	}

	if err := gitRevert(chosen); err != nil {
		fatal("revert failed: %v\n\n  tip: if there are conflicts, resolve them manually and run 'git revert --continue'", err)
	}

	fmt.Printf("\n  ✓ reverted %s\n", chosen[:7])

	askPush()
}

func getRecentCommits(n int) ([]commitEntry, error) {
	// use a unique separator that won't appear in commit messages
	sep := "|||"
	format := "%h" + sep + "%s" + sep + "%cr"

	cmd := exec.Command(
		"git", "log",
		fmt.Sprintf("-%d", n),
		"--no-color",
		"--pretty=format:"+format,
	)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	var commits []commitEntry

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, sep, 3)
		if len(parts) != 3 {
			continue
		}

		hash := strings.TrimSpace(parts[0])
		subject := strings.TrimSpace(parts[1])
		date := strings.TrimSpace(parts[2])

		if len(subject) > 60 {
			subject = subject[:57] + "..."
		}

		if !isSafeHash(hash) {
			continue
		}

		commits = append(commits, commitEntry{
			hash:    hash,
			subject: subject,
			date:    date,
		})
	}

	return commits, nil
}

func askForHash() string {
	fmt.Println()
	fmt.Printf("  enter commit hash (7-40 hex chars) › ")

	for {
		input := strings.TrimSpace(readLine())
		input = strings.ToLower(input)

		if input == "" || input == "q" {
			fmt.Println("  aborted.")
			return ""
		}
		if !isSafeHash(input) {
			fmt.Printf("  invalid hash — only hex characters (0-9, a-f), 7-40 chars › ")
			continue
		}
		return input
	}
}

func gitRevert(hash string) error {
	cmd := exec.Command("git", "revert", hash, "--no-edit")
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return fmt.Errorf("git revert failed")
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func isSafeHash(s string) bool {
	if len(s) < 7 || len(s) > 40 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

func subjectForHash(hash string, commits []commitEntry) string {
	for _, c := range commits {
		if strings.HasPrefix(c.hash, hash) || strings.HasPrefix(hash, c.hash) {
			return c.subject
		}
	}
	return ""
}

func colorDim(s string) string {
	return "\033[2m" + s + "\033[0m"
}

func colorMuted(s string) string {
	return "\033[90m" + s + "\033[0m"
}
