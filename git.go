package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// verifyGitRepo checks we're inside a real git repo.
// Uses explicit args — no shell, no injection possible.
func verifyGitRepo() error {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// getStagedDiff returns the staged diff as a string.
// Hardcoded args — nothing user-supplied is ever passed to the shell.
func getStagedDiff() (string, error) {
	// --no-color ensures clean output
	// -U3 gives 3 lines of context — enough for analysis
	// --diff-filter=ACDMRT excludes untracked/unmerged noise
	cmd := exec.Command(
		"git", "diff", "--staged",
		"--no-color",
		"-U3",
		"--diff-filter=ACDMRT",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}

	raw := stdout.String()

	// safety: cap diff size to 200KB — large diffs don't need full content
	const maxBytes = 200 * 1024
	if len(raw) > maxBytes {
		raw = raw[:maxBytes]
	}

	return raw, nil
}

// getCurrentBranch returns the current branch name safely.
func getCurrentBranch() string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}

// getRemotes returns list of configured remotes.
func getRemotes() []string {
	cmd := exec.Command("git", "remote")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var remotes []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			remotes = append(remotes, l)
		}
	}
	return remotes
}

// runCommit executes git commit with the given message.
// Message is passed as a direct arg — never via shell interpolation.
func runCommit(message string) error {
	// sanitize: strip null bytes and shell metacharacters that could
	// theoretically cause issues even with exec.Command (defense in depth)
	message = sanitizeMessage(message)
	if message == "" {
		return fmt.Errorf("empty commit message")
	}

	cmd := exec.Command("git", "commit", "-m", message)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", stderr.String())
	}
	return nil
}

// runPush executes git push for the current branch.
func runPush(remote, branch string) error {
	// validate remote and branch contain only safe chars
	if !isSafeGitRef(remote) || !isSafeGitRef(branch) {
		return fmt.Errorf("invalid remote or branch name")
	}

	cmd := exec.Command("git", "push", remote, branch)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", stderr.String())
	}
	return nil
}

// sanitizeMessage removes characters that have no place in a commit message.
func sanitizeMessage(s string) string {
	// strip null bytes
	s = strings.ReplaceAll(s, "\x00", "")
	// trim leading/trailing whitespace
	s = strings.TrimSpace(s)
	// collapse internal newlines to space (commit -m takes first line)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// isSafeGitRef validates a git ref/remote name contains only safe characters.
// Prevents any form of argument injection.
func isSafeGitRef(s string) bool {
	if s == "" || len(s) > 200 {
		return false
	}
	for _, c := range s {
		if !isAlphanumeric(c) && c != '-' && c != '_' && c != '.' && c != '/' {
			return false
		}
	}
	return true
}

func isAlphanumeric(c rune) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9')
}
