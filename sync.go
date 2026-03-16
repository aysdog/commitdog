package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func runSync() {
	if err := verifyGitRepo(); err != nil {
		fatal("not a git repository.")
	}

	remotes := getRemotes()
	if len(remotes) == 0 {
		fatal("no remote configured. run 'commitdog init' first.")
	}

	branch := getCurrentBranch()
	if branch == "" || branch == "HEAD" {
		fatal("not on a branch.")
	}

	remote := remotes[0]

	fmt.Println()
	fmt.Printf("  syncing %s/%s\n", remote, branch)
	fmt.Println()

	fmt.Printf("  fetching...")
	if err := gitFetch(remote); err != nil {
		fmt.Println()
		r := detectAndRecover(err.Error())
		if r != nil && offerRecovery(r) {
			return
		}
		fatal("fetch failed: %v", err)
	}
	fmt.Println(" done")

	fmt.Printf("  pulling (rebase)...")
	pulled, err := gitPullRebase(remote, branch)
	if err != nil {
		fmt.Println()
		r := detectAndRecover(err.Error())
		if r != nil {
			if offerRecovery(r) {
				return
			}
			fatal("sync failed: %s — %s", r.message, r.hint)
		}
		if isNothingToPull(err) {
			fmt.Println(" already up to date")
		} else {
			fatal("pull failed: %v", err)
		}
	} else {
		if strings.Contains(pulled, "Already up to date") || strings.Contains(pulled, "up to date") {
			fmt.Println(" already up to date")
		} else {
			fmt.Println(" done")
		}
	}

	fmt.Printf("  pushing...")
	var pushErr error
	if !hasUpstream(branch) {
		pushErr = runPushUpstream(remote, branch)
	} else {
		pushErr = runPush(remote, branch)
	}
	if pushErr != nil {
		fmt.Println()
		if isNothingToPush(pushErr) {
			fmt.Println(" nothing to push")
		} else {
			r := detectAndRecover(pushErr.Error())
			if r != nil {
				if offerRecovery(r) {
					return
				}
				fatal("push failed: %s — %s", r.message, r.hint)
			}
			fatal("push failed: %v", pushErr)
		}
	} else {
		fmt.Println(" done")
	}

	fmt.Printf("\n  %s %s/%s is in sync\n\n", colorGreen("✓"), remote, branch)
}

func gitFetch(remote string) error {
	if !isSafeGitRef(remote) {
		return fmt.Errorf("invalid remote name")
	}
	cmd := exec.Command("git", "fetch", remote)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func gitPullRebase(remote, branch string) (string, error) {
	if !isSafeGitRef(remote) || !isSafeGitRef(branch) {
		return "", fmt.Errorf("invalid remote or branch name")
	}
	cmd := exec.Command("git", "pull", "--rebase", remote, branch)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	return stdout.String() + stderr.String(), nil
}

func isNothingToPush(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "everything up-to-date") ||
		strings.Contains(msg, "nothing to push")
}

func isNothingToPull(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already up to date") ||
		strings.Contains(msg, "up-to-date")
}
