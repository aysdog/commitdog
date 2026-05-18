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
		runBranchMenu()
		return
	}

	switch args[0] {
	case "create", "new":
		runBranchCreate()
	case "ls", "list":
		runBranchList()
	case "delete", "del", "rm":
		runBranchDelete()
	default:
		fmt.Fprintf(os.Stderr, "  unknown branch command: %s\n", args[0])
		fmt.Fprintf(os.Stderr, "  usage: commitdog branch [create|ls|delete]\n")
		os.Exit(1)
	}
}

func runBranchMenu() {
	fmt.Println()
	fmt.Println("  branch:")
	fmt.Println()
	fmt.Println("  1  switch")
	fmt.Println("  2  create new")
	fmt.Println("  3  delete")
	fmt.Println()
	fmt.Printf("  [1-3] pick, [q] quit › ")

	for {
		input := readLine()
		switch input {
		case "1":
			runBranchSwitch()
			return
		case "2":
			runBranchCreate()
			return
		case "3":
			runBranchDelete()
			return
		case "q", "quit", "exit":
			fmt.Println("  aborted.")
			return
		default:
			fmt.Printf("  1, 2, 3, or q › ")
		}
	}
}

func runBranchSwitch() {
	pruneRemoteRefs()
	all, err := getLocalBranches()
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

	var suggestions []string
	diff, err := getStagedDiff()
	if err == nil && diff != "" {
		a := analyzeDiffWithBranch(diff, getCurrentBranch())
		suggestions = generateBranchNameSuggestions(a)
	}

	if len(suggestions) > 0 {
		fmt.Println("  suggested branch names:")
		fmt.Println()
		for i, s := range suggestions {
			fmt.Printf("  %d  %s\n", i+1, s)
		}
		fmt.Println()
		fmt.Printf("  [1-%d] pick, [e] enter name › ", len(suggestions))

		input := readLine()
		picked := false
		name := ""
		for i, s := range suggestions {
			if input == fmt.Sprintf("%d", i+1) {
				name = s
				picked = true
				break
			}
		}
		if !picked {
			fmt.Printf("  new branch name › ")
			for {
				var raw string
				if input != "e" && input != "" {
					raw = input
					input = ""
				} else {
					raw = strings.TrimSpace(readLine())
				}
				if raw == "" {
					fmt.Printf("  name cannot be empty › ")
					continue
				}
				if !isSafeGitRef(raw) {
					fmt.Printf("  invalid branch name (use letters, numbers, - _ /) › ")
					continue
				}
				name = raw
				break
			}
		}
		doCreateBranch(name)
	} else {
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
		doCreateBranch(name)
	}
}

func doCreateBranch(name string) {
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
	if err := runPushUpstreamWithAuth(remotes[0], name, currentAuthHeader()); err != nil {
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

func isProtectedBranch(b string) bool {
	return b == "main" || b == "master"
}

func isLocalOnly(b string) bool {
	return !hasRemoteBranch("origin", b)
}

func runBranchDelete() {
	pruneRemoteRefs()
	all, err := getLocalBranches()
	if err != nil {
		fatal("could not list branches: %v", err)
	}

	current := getCurrentBranch()
	var branches []string
	for _, b := range all {
		if b != current {
			branches = append(branches, b)
		}
	}

	if len(branches) == 0 {
		fmt.Println()
		fmt.Println("  no other branches found.")
		fmt.Println()
		return
	}

	fmt.Println()
	fmt.Println("  select branch to delete:")
	fmt.Println()

	for i, b := range branches {
		if isProtectedBranch(b) {
			fmt.Printf("  %d  %-28s %s\n", i+1, b, colorMuted("(protected)"))
			continue
		}
		var tags []string
		if isLocalOnly(b) {
			tags = append(tags, "local")
		}
		merged, _ := isMergedBranch(b, current)
		if merged {
			tags = append(tags, "merged")
		} else {
			tags = append(tags, "unmerged")
		}
		label := "(" + strings.Join(tags, ", ") + ")"
		if !merged {
			fmt.Printf("  %d  %-28s %s\n", i+1, b, colorRed(label))
		} else {
			fmt.Printf("  %d  %-28s %s\n", i+1, b, colorMuted(label))
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
			if input != fmt.Sprintf("%d", i+1) {
				continue
			}
			if isProtectedBranch(b) {
				fmt.Printf("  %s is protected and cannot be deleted.\n", b)
				fmt.Printf("  [1-%d] pick, [q] quit › ", len(branches))
				break
			}

			localOnly := isLocalOnly(b)
			merged, _ := isMergedBranch(b, current)

			if !merged {
				fmt.Printf("\n  %s %s has unmerged commits.\n", colorRed("⚠"), b)
				fmt.Printf("  delete anyway? this cannot be undone. [y/N] › ")
				confirm := readLine()
				if confirm != "y" && confirm != "yes" {
					fmt.Println("  aborted.")
					return
				}
				fmt.Printf("  deleting %s...", b)
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

			if localOnly {
				return
			}

			remotes := getRemotes()
			if len(remotes) == 0 {
				return
			}

			if !hasRemoteBranch(remotes[0], b) {
				fmt.Printf("  remote branch is not there.\n\n")
				return
			}

			fmt.Printf("  delete %s from %s too? [y/N] › ", b, remotes[0])
			confirm := readLine()
			if confirm == "y" || confirm == "yes" {
				fmt.Printf("  deleting remote branch...")
				if err := gitDeleteRemoteBranch(remotes[0], b); err != nil {
					fmt.Printf("\n  could not reach remote — deleted locally only.\n")
				} else {
					fmt.Printf("\n  ✓ deleted %s/%s\n", remotes[0], b)
				}
			}
			return
		}

		fmt.Printf("  enter 1-%d or q › ", len(branches))
	}
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
func getLocalBranches() ([]string, error) {
	cmd := exec.Command("git", "branch", "--no-color", "--format=%(refname:short)")
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}

	current := getCurrentBranch()
	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		b := strings.TrimSpace(line)
		if b == "" {
			continue
		}
		if b == current {
			branches = append([]string{b}, branches...)
		} else {
			branches = append(branches, b)
		}
	}
	return branches, nil
}

func pruneRemoteRefs() {
	cmd := exec.Command("git", "remote", "prune", "origin")
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	cmd.Run()
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

func colorYellow(s string) string {
	return "\033[33m" + s + "\033[0m"
}
