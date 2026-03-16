package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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

	if isRevertInProgress() {
		handleRevertInProgress()
		return
	}

	commits, err := getRecentCommits(5)
	if err != nil {
		fatal("could not read git log: %v", err)
	}
	if len(commits) == 0 {
		fatal("no commits found in this repository.")
	}

	chosen, ok := pickCommit(commits)
	if !ok {
		return
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

	if isMergeCommit(chosen) {
		parent1, parent2 := getMergeParentBranches(chosen)
		fmt.Println()
		fmt.Println("  ⚠  this is a merge commit. which side do you want to revert to?")
		fmt.Println()
		fmt.Printf("  1  %s  (before the merge — undo it entirely)\n", parent1)
		fmt.Printf("  2  %s  (the branch that was merged in)\n", parent2)
		fmt.Println()
		fmt.Printf("  [1/2] pick › ")

		mainline := 1
		for {
			input := strings.TrimSpace(readLine())
			if input == "1" || input == "" {
				mainline = 1
				break
			} else if input == "2" {
				mainline = 2
				break
			}
			fmt.Printf("  1 or 2 › ")
		}

		if err := gitRevertMerge(chosen, mainline); err != nil {
			if isConflictError(err) {
				fmt.Println()
				fmt.Println("  ✗ revert has conflicts — resolve them manually:")
				fmt.Println()
				fmt.Println("    1. fix the conflicting files")
				fmt.Println("    2. git add .")
				fmt.Println("    3. git revert --continue")
				fmt.Println()
				fmt.Println("  or to cancel: git revert --abort")
				fmt.Println()
				os.Exit(1)
			}
			fatal("revert failed: %v", err)
		}
	} else if err := gitRevert(chosen); err != nil {
		if isConflictError(err) {
			fmt.Println()
			fmt.Println("  ✗ revert has conflicts — resolve them manually:")
			fmt.Println()
			fmt.Println("    1. fix the conflicting files")
			fmt.Println("    2. git add .")
			fmt.Println("    3. git revert --continue")
			fmt.Println()
			fmt.Println("  or to cancel: git revert --abort")
			fmt.Println()
			os.Exit(1)
		}
		fatal("revert failed: %v", err)
	}

	fmt.Printf("\n  ✓ reverted %s\n", chosen[:7])
	askPush()
}

func pickCommit(commits []commitEntry) (string, bool) {
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

	for {
		input := readLine()

		switch input {
		case "q", "quit", "exit":
			fmt.Println("  aborted.")
			return "", false
		case "e", "edit":
			hash, ok := askForHash()
			if !ok {
				return "", false
			}
			return hash, true
		}

		for i, c := range commits {
			if input == fmt.Sprintf("%d", i+1) {
				return c.hash, true
			}
		}

		fmt.Printf("  enter 1-%d, e, or q › ", len(commits))
	}
}

func isRevertInProgress() bool {
	gitDir, err := getGitDir()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(gitDir, "REVERT_HEAD"))
	return err == nil
}

func getGitDir() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func handleRevertInProgress() {
	fmt.Println()
	fmt.Println("  a revert is already in progress.")
	fmt.Println()
	fmt.Printf("  [c] continue  [a] abort  [q] quit › ")

	for {
		input := strings.ToLower(readLine())
		switch input {
		case "c", "continue":
			if err := gitRevertContinue(); err != nil {
				fmt.Printf("\n  ✗ still has conflicts — fix them first, then run 'git revert --continue'\n\n")
				os.Exit(1)
			}
			fmt.Println("\n  ✓ revert completed")
			askPush()
			return
		case "a", "abort":
			if err := gitRevertAbort(); err != nil {
				fatal("revert abort failed: %v", err)
			}
			fmt.Println("\n  ✓ revert aborted — back to previous state")
			return
		case "q", "quit", "":
			fmt.Println("  exited. revert is still in progress.")
			return
		default:
			fmt.Printf("  c, a, or q › ")
		}
	}
}

func gitRevertContinue() error {
	cmd := exec.Command("git", "revert", "--continue", "--no-edit")
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func gitRevertAbort() error {
	cmd := exec.Command("git", "revert", "--abort")
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func isConflictError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "conflict") ||
		strings.Contains(msg, "could not revert") ||
		strings.Contains(msg, "after resolving")
}

func getRecentCommits(n int) ([]commitEntry, error) {
	sep := "|||"
	format := "%h" + sep + "%s" + sep + "%cr"

	cmd := exec.Command(
		"git", "log",
		fmt.Sprintf("-%d", n),
		"--no-color",
		"--pretty=format:"+format,
	)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")

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

		if isRootCommit(hash) {
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

func isRootCommit(hash string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", hash+"^")
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	return cmd.Run() != nil
}

func askForHash() (string, bool) {
	fmt.Println()
	fmt.Printf("  enter commit hash (7-40 hex chars) › ")

	for {
		input := strings.TrimSpace(strings.ToLower(readLine()))

		if input == "" || input == "q" {
			return "", false
		}
		if !isSafeHash(input) {
			fmt.Printf("  %s › ", colorRed("invalid hash — only hex characters (0-9, a-f), 7-40 chars"))
			continue
		}
		if isRootCommit(input) {
			fmt.Printf("\n  %s\n", colorRed("can't revert the first commit — it has no parent"))
			time.Sleep(400 * time.Millisecond)
			return "", false
		}
		return input, true
	}
}

func isMergeCommit(hash string) bool {
	cmd := exec.Command("git", "rev-parse", hash+"^2")
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	return cmd.Run() == nil
}

func getMergeParentBranches(hash string) (parent1, parent2 string) {
	cmd1 := exec.Command("git", "rev-parse", "--short", hash+"^1")
	cmd1.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var out1 bytes.Buffer
	cmd1.Stdout = &out1
	cmd1.Run()
	p1 := strings.TrimSpace(out1.String())

	cmd2 := exec.Command("git", "rev-parse", "--short", hash+"^2")
	cmd2.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var out2 bytes.Buffer
	cmd2.Stdout = &out2
	cmd2.Run()
	p2 := strings.TrimSpace(out2.String())

	parent1 = resolveBranchName(p1)
	parent2 = resolveBranchName(p2)
	return
}

func resolveBranchName(hash string) string {
	cmd := exec.Command("git", "branch", "--all", "--format=%(refname:short)", "--points-at", hash)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil || strings.TrimSpace(out.String()) == "" {
		return hash
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "origin/HEAD") {
			return l
		}
	}
	return hash
}

func gitRevert(hash string) error {
	cmd := exec.Command("git", "revert", hash, "--no-edit")
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
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

func gitRevertMerge(hash string, mainline int) error {
	cmd := exec.Command("git", "revert", "-m", fmt.Sprintf("%d", mainline), hash, "--no-edit")
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
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

func colorRed(s string) string {
	return "\033[31m" + s + "\033[0m"
}

func colorDim(s string) string {
	return "\033[2m" + s + "\033[0m"
}

func colorMuted(s string) string {
	return "\033[90m" + s + "\033[0m"
}
