package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var reader = bufio.NewReader(os.Stdin)

func pickSuggestion(suggestions []string) string {
	fmt.Println()
	fmt.Println("  suggestions:")
	fmt.Println()

	for i, s := range suggestions {
		fmt.Printf("  %d  %s\n", i+1, subjectLine(s))
	}

	fmt.Println()
	fmt.Printf("  [")
	for i := range suggestions {
		if i > 0 {
			fmt.Printf("/")
		}
		fmt.Printf("%d", i+1)
	}
	fmt.Printf("] pick, [e] edit, [q] quit › ")

	for {
		input := readLine()

		switch input {
		case "q", "quit", "exit":
			return ""
		case "e", "edit":
			return editMessage(suggestions[0])
		}

		for i := range suggestions {
			if input == fmt.Sprintf("%d", i+1) {
				return suggestions[i]
			}
		}

		fmt.Printf("  enter ")
		for i := range suggestions {
			if i > 0 {
				fmt.Printf("/")
			}
			fmt.Printf("%d", i+1)
		}
		fmt.Printf(", e, or q › ")
	}
}

func editMessage(suggestion string) string {
	fmt.Printf("\n  edit message (enter to keep, ctrl+c to abort):\n")
	fmt.Printf("  > %s\n", suggestion)
	fmt.Printf("  > ")

	input := readLine()
	if input == "" {
		return suggestion
	}

	cleaned := sanitizeMessage(input)
	if cleaned == "" {
		fmt.Println("  empty message, using original.")
		return suggestion
	}
	return cleaned
}

func askPush(platform string) {
	proj := loadProjectConfig()
	if platform == "" {
		platform = proj.effectivePrimary()
	}
	if platform == "" {
		platform = "github"
	}

	branch := getCurrentBranch()
	if branch == "" || branch == "HEAD" {
		return
	}

	remote := platformRemoteName(platform)
	remotes := getRemotes()
	found := false
	for _, r := range remotes {
		if r == remote {
			found = true
			break
		}
	}
	if !found {
		if len(remotes) == 0 {
			return
		}
		remote = remotes[0]
	}

	fmt.Printf("\n  push to %s/%s? [Y/n] › ", remote, branch)

	for {
		input := readLine()
		switch input {
		case "y", "yes", "":
			fmt.Printf("  pushing...")
			authHeader := authHeaderForPlatformName(platform)
			var err error
			if !hasUpstream(branch) {
				err = runPushUpstreamWithAuth(remote, branch, authHeader)
			} else {
				err = runPushWithAuth(remote, branch, authHeader)
			}
			if err != nil {
				fmt.Println()
				r := detectAndRecover(err.Error())
				if r != nil {
					offerRecovery(r)
					return
				}
				fmt.Printf("\n  %s push failed: %s\n", colorRed("✗"), err)
			} else {
				fmt.Printf("\n  %s pushed to %s/%s\n", colorGreen("✓"), remote, branch)
			}
			return
		case "n", "no":
			fmt.Println("  skipped.")
			return
		default:
			fmt.Printf("  Y or n › ")
		}
	}
}

func hasUpstream(branch string) bool {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", branch+"@{upstream}")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = nil
	return cmd.Run() == nil
}

func readLine() string {
	line, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSpace(line)
}
