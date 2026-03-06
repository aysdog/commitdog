package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
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
		return fmt.Errorf("%s", stderr.String())
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
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		f := strings.TrimSpace(line)
		if f == "" {
			continue
		}
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		if info.Size() == 0 {
			intent := exec.Command("git", "add", "-N", "--", f)
			intent.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
			intent.Run()
		}
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
		if len(line) < 4 {
			continue
		}
		xy := line[:2]
		path := strings.TrimSpace(line[3:])
		if path == "" {
			continue
		}
		if xy == "A " || xy == "AN" {
			files = append(files, path)
		}
	}
	return files
}

func warnEmptyDirs() {
	entries, err := os.ReadDir(".")
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == ".git" {
			continue
		}
		sub, err := os.ReadDir(name)
		if err != nil {
			continue
		}
		if len(sub) == 0 {
			fmt.Printf("\n  %s/ is an empty directory — git does not track these. Add a file inside first.\n", name)
		}
	}
}
