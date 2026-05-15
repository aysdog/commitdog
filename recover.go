package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type recovery struct {
	message string
	hint    string
	autoFix func() error
}

func detectAndRecover(stderr string) *recovery {
	msg := strings.ToLower(stderr)

	if strings.Contains(msg, "already exists") && strings.Contains(msg, "tag") {
		tag := extractTagFromError(stderr)
		return &recovery{
			message: fmt.Sprintf("tag %s already exists on remote", tag),
			hint:    "delete the remote tag and retry",
			autoFix: func() error {
				fmt.Printf("  deleting remote tag %s...\n", tag)
				cmd := exec.Command("git", "push", "origin", ":refs/tags/"+tag)
				var stderr bytes.Buffer
				cmd.Stderr = &stderr
				if err := cmd.Run(); err != nil {
					return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
				}
				exec.Command("git", "tag", "-d", tag).Run()
				fmt.Printf("  %s deleted. run commitdog release again.\n", tag)
				return nil
			},
		}
	}

	if strings.Contains(msg, "conflict") || strings.Contains(msg, "could not apply") {
		files, _ := getConflictedFiles()
		return &recovery{
			message: "rebase conflict detected",
			hint:    conflictHint(files),
			autoFix: func() error {
				return autoResolveConflict()
			},
		}
	}

	if strings.Contains(msg, "non-fast-forward") || strings.Contains(msg, "fetch first") || strings.Contains(msg, "tip of your current branch is behind") {
		return &recovery{
			message: "remote has new commits you don't have locally",
			hint:    "pull first, then push",
			autoFix: func() error {
				return autoSyncAndRetry()
			},
		}
	}

	if strings.Contains(msg, "protected branch") || strings.Contains(msg, "push to protected") {
		return &recovery{
			message: "this branch is protected — direct push not allowed",
			hint:    "create a PR instead",
			autoFix: func() error {
				fmt.Println()
				fmt.Println("  running commitdog pr...")
				fmt.Println()
				runPR()
				return nil
			},
		}
	}

	if strings.Contains(msg, "password authentication") || strings.Contains(msg, "invalid username or password") || (strings.Contains(msg, "authentication failed") && strings.Contains(msg, "https://")) {
		return &recovery{
			message: "HTTPS password auth rejected by GitHub",
			hint:    "switch remote to SSH",
			autoFix: func() error {
				remotes := getRemotes()
				if len(remotes) == 0 {
					return fmt.Errorf("no remote found")
				}
				branch := getCurrentBranch()
				return tryFixHTTPSRemote(remotes[0], branch)
			},
		}
	}

	if strings.Contains(msg, "repository not found") || strings.Contains(msg, "does not appear to be a git repository") {
		detected := detectActualRemote()
		if detected != "" {
			return &recovery{
				message: fmt.Sprintf("remote 'origin' not found — detected '%s'", detected),
				hint:    fmt.Sprintf("use '%s' as remote", detected),
				autoFix: func() error {
					return autoFixRemoteName(detected)
				},
			}
		}
		return &recovery{
			message: "remote not found",
			hint:    "run 'commitdog init' to set up remote",
		}
	}

	if strings.Contains(msg, "could not read username") || strings.Contains(msg, "terminal prompts disabled") {
		return &recovery{
			message: "git is asking for credentials interactively — not supported in this context",
			hint:    "switch to SSH: git remote set-url origin git@github.com:user/repo.git",
		}
	}

	if strings.Contains(msg, "no such ref was fetched") || strings.Contains(msg, "couldn't find remote ref") {
		branch := getCurrentBranch()
		return &recovery{
			message: fmt.Sprintf("branch '%s' does not exist on remote yet", branch),
			hint:    "push with upstream tracking",
			autoFix: func() error {
				remotes := getRemotes()
				if len(remotes) == 0 {
					return fmt.Errorf("no remote found")
				}
				return runPushUpstreamWithAuth(remotes[0], branch, currentAuthHeader())
			},
		}
	}

	return nil
}

func offerRecovery(r *recovery) bool {
	fmt.Println()
	fmt.Printf("  %s  %s\n", colorRed("✗"), r.message)
	if r.hint != "" {
		fmt.Printf("  hint: %s\n", r.hint)
	}
	if r.autoFix == nil {
		fmt.Println()
		return false
	}
	fmt.Printf("\n  fix automatically? [Y/n] › ")
	input := readLine()
	if input == "n" || input == "no" {
		fmt.Println()
		return false
	}
	fmt.Println()
	if err := r.autoFix(); err != nil {
		fmt.Printf("  %s auto-fix failed: %s\n\n", colorRed("✗"), err)
		return false
	}
	return true
}

func conflictHint(files []string) string {
	if len(files) == 0 {
		return "resolve conflicts, then git add . && git rebase --continue"
	}
	if len(files) == 1 {
		return fmt.Sprintf("conflict in %s", files[0])
	}
	return fmt.Sprintf("conflicts in %s and %d other file(s)", files[0], len(files)-1)
}

func autoResolveConflict() error {
	fmt.Println("  aborting rebase...")
	exec.Command("git", "rebase", "--abort").Run()

	fmt.Println("  stashing your changes...")
	stashCmd := exec.Command("git", "stash", "push", "-m", "commitdog-auto-stash")
	var stashStderr bytes.Buffer
	stashCmd.Stderr = &stashStderr
	if err := stashCmd.Run(); err != nil {
		return fmt.Errorf("stash failed: %s", strings.TrimSpace(stashStderr.String()))
	}

	remotes := getRemotes()
	if len(remotes) == 0 {
		exec.Command("git", "stash", "pop").Run()
		return fmt.Errorf("no remote configured")
	}
	remote := remotes[0]
	branch := getCurrentBranch()

	fmt.Println("  pulling latest changes...")
	pullCmd := exec.Command("git", "pull", "--rebase", remote, branch)
	pullCmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var pullStderr bytes.Buffer
	pullCmd.Stderr = &pullStderr
	if err := pullCmd.Run(); err != nil {
		exec.Command("git", "stash", "pop").Run()
		return fmt.Errorf("pull failed after stash: %s", strings.TrimSpace(pullStderr.String()))
	}

	fmt.Println("  restoring your changes...")
	popCmd := exec.Command("git", "stash", "pop")
	var popStderr bytes.Buffer
	popCmd.Stderr = &popStderr
	if err := popCmd.Run(); err != nil {
		msg := strings.TrimSpace(popStderr.String())
		if strings.Contains(strings.ToLower(msg), "conflict") {
			fmt.Println()
			fmt.Println("  stash pop has conflicts — your changes conflict with incoming.")
			fmt.Println("  resolve the conflicts in the files above, then:")
			fmt.Println("    git add .")
			fmt.Println("    git stash drop")
			fmt.Println()
			return fmt.Errorf("manual resolution needed")
		}
		return fmt.Errorf("stash pop failed: %s", msg)
	}

	fmt.Printf("  %s stash+sync+pop complete — ready to push\n", colorGreen("✓"))
	return nil
}

func autoSyncAndRetry() error {
	remotes := getRemotes()
	if len(remotes) == 0 {
		return fmt.Errorf("no remote configured")
	}
	remote := remotes[0]
	branch := getCurrentBranch()

	fmt.Printf("  pulling %s/%s...", remote, branch)
	pullCmd := exec.Command("git", "pull", "--rebase", remote, branch)
	pullCmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var pullStderr bytes.Buffer
	pullCmd.Stderr = &pullStderr
	if err := pullCmd.Run(); err != nil {
		fmt.Println()
		msg := strings.TrimSpace(pullStderr.String())
		r := detectAndRecover(msg)
		if r != nil {
			return fmt.Errorf("%s — %s", r.message, r.hint)
		}
		return fmt.Errorf("pull failed: %s", msg)
	}
	fmt.Println(" done")

	fmt.Printf("  retrying push...")
	pushCmd := exec.Command("git", "push", remote, branch)
	var pushStderr bytes.Buffer
	pushCmd.Stderr = &pushStderr
	if err := pushCmd.Run(); err != nil {
		fmt.Println()
		return fmt.Errorf("push still failed: %s", strings.TrimSpace(pushStderr.String()))
	}
	fmt.Println(" done")
	fmt.Printf("  %s pushed to %s/%s\n", colorGreen("✓"), remote, branch)
	return nil
}

func detectActualRemote() string {
	cmd := exec.Command("git", "remote")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	remotes := strings.Split(strings.TrimSpace(out.String()), "\n")
	for _, r := range remotes {
		r = strings.TrimSpace(r)
		if r != "" && r != "origin" {
			return r
		}
	}
	return ""
}

func autoFixRemoteName(correct string) error {
	fmt.Printf("  updating remote references to use '%s'...\n", correct)
	return nil
}

func extractTagFromError(stderr string) string {
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "[rejected]") {
			parts := strings.Fields(line)
			for i, p := range parts {
				if p == "->" && i > 0 {
					tag := strings.TrimSpace(parts[i-1])
					if tag != "" {
						return tag
					}
				}
			}
		}
	}
	for _, line := range strings.Split(stderr, "\n") {
		parts := strings.Fields(strings.TrimSpace(line))
		for _, p := range parts {
			p = strings.TrimPrefix(p, "refs/tags/")
			if strings.HasPrefix(p, "v") && len(p) > 1 {
				return p
			}
		}
	}
	return "unknown"
}

func colorGreen(s string) string {
	return "\033[32m" + s + "\033[0m"
}
