package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func runBranch() {
	if err := verifyGitRepo(); err != nil {
		fatal("not a git repository.")
	}

	args := os.Args[2:]
	if len(args) == 0 {
		runBranchSwitch()
		return
	}

	switch args[0] {
	case "switch", "sw":
		runBranchSwitch()
	case "create", "new":
		runBranchCreate()
	case "ls", "list":
		runBranchList()
	case "delete", "del", "rm":
		runBranchDelete()
	default:
		fmt.Fprintf(os.Stderr, "  unknown branch command: %s\n", args[0])
		fmt.Fprintf(os.Stderr, "  usage: commitdog branch [switch|create|ls|delete]\n")
		os.Exit(1)
	}
}

func runBranchSwitch() {
	all, err := getAllBranches()
	if err != nil {
		fatal("could not list branches: %v", err)
	}
	if len(all) == 0 {
		fatal("no branches found.")
	}

	current := getCurrentBranch()

	switchable := []string{}
	for _, b := range all {
		if b != current {
			switchable = append(switchable, b)
		}
	}

	if len(switchable) == 0 {
		fmt.Println()
		fmt.Printf("  only one branch exists: %s\n\n", colorYellow(current))
		return
	}

	displayed := switchable
	if len(displayed) > 5 {
		displayed = displayed[:5]
	}

	fmt.Println()
	fmt.Println("  branches:")
	fmt.Println()

	fmt.Printf("  %s %s %s\n", colorYellow("→"), colorYellow(current), colorMuted("(current)"))
	fmt.Println()

	for i, b := range displayed {
		fmt.Printf("  %d  %s\n", i+1, b)
	}

	fmt.Println()
	fmt.Printf("  [1-%d] pick, [e] enter name, [q] quit › ", len(displayed))

	for {
		input := readLine()

		switch input {
		case "q", "quit", "exit":
			fmt.Println("  aborted.")
			return
		case "e", "edit":
			chosen := askBranchName(all, current)
			if chosen == "" {
				return
			}
			doSwitch(chosen)
			return
		}

		for i, b := range displayed {
			if input == fmt.Sprintf("%d", i+1) {
				doSwitch(b)
				return
			}
		}

		fmt.Printf("  enter 1-%d, e, or q › ", len(displayed))
	}
}

func doSwitch(branch string) {
	fmt.Printf("  switching to %s...\n", branch)
	if err := gitCheckout(branch); err != nil {
		fatal("switch failed: %v", err)
	}
	fmt.Printf("  ✓ switched to %s\n", branch)
}

func askBranchName(all []string, current string) string {
	fmt.Println()
	fmt.Printf("  enter branch name › ")
	for {
		input := strings.TrimSpace(readLine())
		if input == "" || input == "q" {
			fmt.Println("  aborted.")
			return ""
		}
		if !isSafeGitRef(input) {
			fmt.Printf("  invalid branch name › ")
			continue
		}
		if input == current {
			fmt.Printf("  already on %s › ", input)
			continue
		}
		exists := false
		for _, b := range all {
			if b == input {
				exists = true
				break
			}
		}
		if !exists {
			fmt.Printf("  branch %q not found › ", input)
			continue
		}
		return input
	}
}

func runBranchCreate() {
	fmt.Println()
	fmt.Printf("  new branch name › ")

	name := ""
	for {
		input := strings.TrimSpace(readLine())
		if input == "" {
			fmt.Printf("  name cannot be empty › ")
			continue
		}
		if !isSafeGitRef(input) {
			fmt.Printf("  invalid branch name (use letters, numbers, - _ /) › ")
			continue
		}
		name = input
		break
	}

	current := getCurrentBranch()
	fmt.Printf("  base branch? [enter = %s] › ", current)
	base := strings.TrimSpace(readLine())
	if base == "" {
		base = current
	}

	if err := gitCheckoutNewBranch(name, base); err != nil {
		fatal("could not create branch: %v", err)
	}

	fmt.Printf("  ✓ created and switched to %s\n", name)

	remotes := getRemotes()
	if len(remotes) == 0 {
		return
	}

	fmt.Printf("\n  push %s to %s? [Y/n] › ", name, remotes[0])
	confirm := readLine()
	if confirm == "n" || confirm == "no" {
		fmt.Println("  skipped.")
		return
	}

	fmt.Printf("  pushing...")
	if err := runPushUpstream(remotes[0], name); err != nil {
		fmt.Printf("\n  push failed: %v\n", err)
	} else {
		fmt.Printf("\n  ✓ pushed %s to %s\n", name, remotes[0])
	}
}

func runBranchList() {
	branches, err := getAllBranches()
	if err != nil {
		fatal("could not list branches: %v", err)
	}
	if len(branches) == 0 {
		fatal("no branches found.")
	}

	current := getCurrentBranch()

	fmt.Println()
	fmt.Println("  branches:")
	fmt.Println()

	for _, b := range branches {
		if b == current {
			fmt.Printf("  %s %s %s\n", colorYellow("→"), colorYellow(b), colorMuted("(current)"))
		} else {
			fmt.Printf("    %s\n", b)
		}
	}

	fmt.Println()
}

func runBranchDelete() {
	branches, err := getDeletableBranches()
	if err != nil {
		fatal("could not list branches: %v", err)
	}
	if len(branches) == 0 {
		fmt.Println()
		fmt.Println("  no branches available to delete.")
		fmt.Println("  (current branch, main, and master are protected)")
		fmt.Println()
		return
	}

	current := getCurrentBranch()

	fmt.Println()
	fmt.Println("  select branch to delete:")
	fmt.Println()

	for i, b := range branches {
		merged, _ := isMergedBranch(b, current)
		if merged {
			fmt.Printf("  %d  %s %s\n", i+1, b, colorMuted("(merged)"))
		} else {
			fmt.Printf("  %d  %s %s\n", i+1, b, colorRed("(unmerged)"))
		}
	}

	fmt.Println()
	fmt.Printf("  [1-%d] pick, [q] quit › ", len(branches))

	for {
		input := readLine()
		switch input {
		case "q", "quit", "exit":
			fmt.Println("  aborted.")
			return
		}

		for i, b := range branches {
			if input == fmt.Sprintf("%d", i+1) {
				merged, _ := isMergedBranch(b, current)
				if !merged {
					fmt.Printf("\n  %s %s has unmerged commits.\n", colorRed("⚠"), b)
					fmt.Printf("  delete anyway? this cannot be undone. [y/N] › ")
					confirm := readLine()
					if confirm != "y" && confirm != "yes" {
						fmt.Println("  aborted.")
						return
					}
					fmt.Printf("  force deleting %s...", b)
					if err := gitDeleteBranchForce(b); err != nil {
						fmt.Println()
						fatal("delete failed: %v", err)
					}
				} else {
					fmt.Printf("  deleting %s...", b)
					if err := gitDeleteBranch(b); err != nil {
						fmt.Println()
						fatal("delete failed: %v", err)
					}
				}
				fmt.Println(" done")
				fmt.Printf("  ✓ deleted %s\n\n", b)

				remotes := getRemotes()
				if len(remotes) == 0 {
					return
				}
				if hasRemoteBranch(remotes[0], b) {
					fmt.Printf("  delete %s from %s too? [y/N] › ", b, remotes[0])
					confirm := readLine()
					if confirm == "y" || confirm == "yes" {
						fmt.Printf("  deleting remote branch...")
						if err := gitDeleteRemoteBranch(remotes[0], b); err != nil {
							fmt.Printf("\n  remote delete failed: %v\n", err)
						} else {
							fmt.Printf("\n  ✓ deleted %s/%s\n", remotes[0], b)
						}
					}
				}
				return
			}
		}

		fmt.Printf("  enter 1-%d or q › ", len(branches))
	}
}

func getDeletableBranches() ([]string, error) {
	all, err := getAllBranches()
	if err != nil {
		return nil, err
	}
	current := getCurrentBranch()
	var result []string
	for _, b := range all {
		if b == current {
			continue
		}
		if b == "main" || b == "master" {
			continue
		}
		result = append(result, b)
	}
	return result, nil
}

func isMergedBranch(branch, current string) (bool, error) {
	cmd := exec.Command("git", "branch", "--merged", current, "--no-color")
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return false, err
	}
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.TrimSpace(strings.TrimPrefix(line, "*")) == branch {
			return true, nil
		}
	}
	return false, nil
}

func gitDeleteBranch(branch string) error {
	if !isSafeGitRef(branch) {
		return fmt.Errorf("invalid branch name")
	}
	cmd := exec.Command("git", "branch", "-d", branch)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func gitDeleteBranchForce(branch string) error {
	if !isSafeGitRef(branch) {
		return fmt.Errorf("invalid branch name")
	}
	cmd := exec.Command("git", "branch", "-D", branch)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func hasRemoteBranch(remote, branch string) bool {
	ref := "refs/remotes/" + remote + "/" + branch
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", ref)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	return cmd.Run() == nil
}

func gitDeleteRemoteBranch(remote, branch string) error {
	if !isSafeGitRef(remote) || !isSafeGitRef(branch) {
		return fmt.Errorf("invalid remote or branch name")
	}
	cmd := exec.Command("git", "push", remote, "--delete", branch)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	return nil
}
func getAllBranches() ([]string, error) {
	cmd := exec.Command("git", "branch", "-a", "--no-color", "--format=%(refname:short)")
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}

	seen := map[string]bool{}
	var branches []string
	current := getCurrentBranch()

	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		b := strings.TrimSpace(line)
		if b == "" {
			continue
		}
		b = strings.TrimPrefix(b, "origin/")
		b = strings.TrimPrefix(b, "HEAD -> ")
		if strings.Contains(b, "HEAD") {
			continue
		}
		if seen[b] {
			continue
		}
		seen[b] = true
		if b == current {
			branches = append([]string{b}, branches...)
		} else {
			branches = append(branches, b)
		}
	}

	return branches, nil
}

func gitCheckout(branch string) error {
	if !isSafeGitRef(branch) {
		return fmt.Errorf("invalid branch name")
	}
	cmd := exec.Command("git", "checkout", branch)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func gitCheckoutNewBranch(name, base string) error {
	if !isSafeGitRef(name) || !isSafeGitRef(base) {
		return fmt.Errorf("invalid branch name")
	}
	cmd := exec.Command("git", "checkout", "-b", name, base)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func runPushUpstream(remote, branch string) error {
	if !isSafeGitRef(remote) || !isSafeGitRef(branch) {
		return fmt.Errorf("invalid remote or branch name")
	}
	cmd := exec.Command("git", "push", "--set-upstream", remote, branch)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func colorYellow(s string) string {
	return "\033[33m" + s + "\033[0m"
}
