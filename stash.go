package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func runStash() {
	if err := verifyGitRepo(); err != nil {
		fatal("not a git repository.")
	}

	stashes, err := getStashList()
	if err != nil {
		fatal("could not read stash list: %v", err)
	}

	if len(stashes) == 0 {
		runStashSave()
		return
	}

	fmt.Println()
	fmt.Println("  stashes:")
	fmt.Println()

	for i, s := range stashes {
		fmt.Printf("  %d  %s\n", i+1, s)
	}

	fmt.Println()
	fmt.Printf("  [s] save new, [1-%d] pop, [d1-%d] drop, [q] quit › ", len(stashes), len(stashes))

	for {
		input := strings.ToLower(strings.TrimSpace(readLine()))

		switch input {
		case "q", "quit", "exit":
			fmt.Println("  aborted.")
			return
		case "s", "save":
			runStashSave()
			return
		}

		if strings.HasPrefix(input, "d") {
			numStr := strings.TrimPrefix(input, "d")
			for i := range stashes {
				if numStr == fmt.Sprintf("%d", i+1) {
					fmt.Printf("  dropping stash %d...", i+1)
					if err := gitStashDrop(i); err != nil {
						fmt.Println()
						fatal("drop failed: %v", err)
					}
					fmt.Println(" done")
					fmt.Printf("  ✓ dropped stash %d\n", i+1)
					return
				}
			}
		}

		for i, s := range stashes {
			if input == fmt.Sprintf("%d", i+1) {
				fmt.Printf("  popping: %s\n", s)
				if err := gitStashPop(i); err != nil {
					if isStashConflict(err) {
						fmt.Printf("\n  %s stash pop has conflicts — resolve them:\n", colorRed("✗"))
						fmt.Println()
						fmt.Println("    1. fix conflicting files")
						fmt.Println("    2. git add .")
						fmt.Println("    3. git stash drop stash@{0}")
						fmt.Println()
						os.Exit(1)
					}
					fatal("pop failed: %v", err)
				}
				fmt.Printf("  ✓ popped stash %d\n", i+1)
				return
			}
		}

		fmt.Printf("  s, 1-%d, d1-%d, or q › ", len(stashes), len(stashes))
	}
}

func runStashSave() {
	fmt.Println()
	fmt.Printf("  stash message (optional, enter to skip) › ")
	msg := strings.TrimSpace(readLine())

	fmt.Printf("  saving...")
	if err := gitStashSave(msg); err != nil {
		fmt.Println()
		if isNothingToStash(err) {
			fmt.Println()
			fmt.Println("  nothing to stash — no local changes.")
			return
		}
		fatal("stash failed: %v", err)
	}
	fmt.Println(" done")

	if msg != "" {
		fmt.Printf("  ✓ stashed: %s\n\n", msg)
	} else {
		fmt.Printf("  ✓ stashed current changes\n\n")
	}
}

func getStashList() ([]string, error) {
	cmd := exec.Command("git", "stash", "list", "--no-color")
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var stashes []string
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ": ", 3)
		if len(parts) == 3 {
			stashes = append(stashes, parts[1]+": "+parts[2])
		} else {
			stashes = append(stashes, line)
		}
	}
	return stashes, nil
}

func gitStashSave(msg string) error {
	var cmd *exec.Cmd
	if msg != "" {
		cmd = exec.Command("git", "stash", "push", "-u", "-m", msg)
	} else {
		cmd = exec.Command("git", "stash", "push", "-u")
	}
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	out := stdout.String() + stderr.String()
	if strings.Contains(out, "No local changes") {
		return fmt.Errorf("nothing to stash")
	}
	return nil
}

func gitStashPop(index int) error {
	ref := fmt.Sprintf("stash@{%d}", index)
	cmd := exec.Command("git", "stash", "pop", ref)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func gitStashDrop(index int) error {
	ref := fmt.Sprintf("stash@{%d}", index)
	cmd := exec.Command("git", "stash", "drop", ref)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func isStashConflict(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "conflict") ||
		strings.Contains(msg, "merge conflict")
}

func isNothingToStash(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "nothing to stash") ||
		strings.Contains(msg, "no local changes")
}
