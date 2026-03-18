package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func verifyGitRepo() error {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func getStagedDiff() (string, error) {
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

	const maxBytes = 200 * 1024
	if len(raw) > maxBytes {
		raw = raw[:maxBytes]
	}

	return raw, nil
}

func getCurrentBranch() string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}

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

func runCommit(message string) error {
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

func runPush(remote, branch string) error {
	if !isSafeGitRef(remote) || !isSafeGitRef(branch) {
		return fmt.Errorf("invalid remote or branch name")
	}

	cmd := exec.Command("git", "push", remote, branch)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if isHTTPSAuthError(msg) {
			return tryFixHTTPSRemote(remote, branch)
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func isHTTPSAuthError(msg string) bool {
	return strings.Contains(msg, "Password authentication is not supported") ||
		strings.Contains(msg, "Invalid username or password") ||
		strings.Contains(msg, "Authentication failed") && strings.Contains(msg, "https://")
}

func tryFixHTTPSRemote(remote, branch string) error {
	cmd := exec.Command("git", "remote", "get-url", remote)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("push failed: authentication error. switch your remote to SSH manually:\n  git remote set-url origin git@github.com:user/repo.git")
	}
	currentURL := strings.TrimSpace(out.String())

	if !strings.HasPrefix(currentURL, "https://github.com/") {
		return fmt.Errorf("push failed: authentication error")
	}

	sshURL := strings.Replace(currentURL, "https://github.com/", "git@github.com:", 1)

	fmt.Println()
	fmt.Printf("  push failed: GitHub no longer supports HTTPS password auth.\n")
	fmt.Printf("  switch remote to SSH? (%s) [Y/n] › ", sshURL)

	confirm := readLine()
	if confirm == "n" || confirm == "no" {
		fmt.Println()
		fmt.Println("  to fix manually:")
		fmt.Printf("  git remote set-url %s %s\n", remote, sshURL)
		fmt.Println()
		return fmt.Errorf("push aborted")
	}

	setCmd := exec.Command("git", "remote", "set-url", remote, sshURL)
	var stderr bytes.Buffer
	setCmd.Stderr = &stderr
	if err := setCmd.Run(); err != nil {
		return fmt.Errorf("could not update remote: %s", strings.TrimSpace(stderr.String()))
	}

	fmt.Printf("  ✓ remote switched to SSH\n")
	fmt.Printf("  retrying push...\n")

	pushCmd := exec.Command("git", "push", remote, branch)
	var pushStderr bytes.Buffer
	pushCmd.Stderr = &pushStderr
	if err := pushCmd.Run(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(pushStderr.String()))
	}
	return nil
}

func sanitizeMessage(s string) string {
	s = strings.ReplaceAll(s, "\x00", "")

	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

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

func stageFiles(paths []string) error {
	for _, p := range paths {
		cmd := exec.Command("git", "add", "--", p)
		cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("could not stage %s: %s", p, strings.TrimSpace(stderr.String()))
		}
	}
	return nil
}

func isAlphanumeric(c rune) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9')
}

func stageUntrackedEmpty() {
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return
	}
	for _, f := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		info, err := os.Stat(f)
		if err != nil || info.Size() != 0 {
			continue
		}
		exec.Command("git", "add", "-N", "--", f).Run()
	}
}

func getStagedNewFiles() []string {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil
	}
	var files []string
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(line, "A ") || strings.HasPrefix(line, "AN") {
			f := strings.TrimSpace(line[2:])
			if f != "" {
				files = append(files, f)
			}
		}
	}
	return files
}

func warnEmptyDirs() {
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil || path == "." {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if strings.HasPrefix(path, ".git") {
			return filepath.SkipDir
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil
		}
		if len(entries) == 0 {
			fmt.Printf("  %s/ is an empty directory — git does not track these. Add a file inside first.\n", path)
		}
		return nil
	})
	_ = err
}
