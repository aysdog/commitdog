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
		if preview := bodyPreview(s); preview != "" {
			fmt.Printf("     \033[90m%s\033[0m\n", preview)
		}
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

	remote := remoteForPlatform(platform)
	if !remoteExists(remote) {
		fmt.Println()
		fmt.Printf("  remote '%s' not configured.\n\n", remote)
		fmt.Println("  1  add remote now")
		fmt.Println("  2  skip")
		fmt.Println()
		fmt.Printf("  [1/2] pick › ")
		if strings.TrimSpace(readLine()) == "1" {
			fmt.Printf("  remote URL › ")
			url := strings.TrimSpace(readLine())
			if url != "" {
				if err := gitAddRemote(remote, url); err != nil {
					fmt.Printf("  could not add remote: %v\n\n", err)
					return
				}
				fmt.Printf("  ✓ remote '%s' added\n\n", remote)
			}
		} else {
			return
		}
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
				proj := loadProjectConfig()
				mirrors := proj.mirrors
				errMsg := err.Error()
				isPlatformError := strings.Contains(errMsg, "403") || strings.Contains(errMsg, "permission") || strings.Contains(errMsg, "not allowed") || strings.Contains(errMsg, "authentication") || strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "repository")
				if len(mirrors) > 0 && isPlatformError {
					fmt.Printf("  %s push to %s failed.\n\n", colorRed("✗"), remote)
					fmt.Println("  1  push to mirror instead")
					fmt.Println("  2  skip")
					fmt.Println()
					fmt.Printf("  [1/2] pick › ")
					if strings.TrimSpace(readLine()) == "1" {
						var availableMirrors []string
						for _, m := range mirrors {
							if remoteExists(platformRemoteName(m)) {
								availableMirrors = append(availableMirrors, m)
							}
						}
						if len(availableMirrors) == 0 {
							fmt.Println("  no mirror remotes configured. run 'commitdog init' to add one.")
							return
						}
						for i, m := range availableMirrors {
							fmt.Printf("  %d  %s\n", i+1, m)
						}
						fmt.Println()
						fmt.Printf("  [1-%d] pick › ", len(availableMirrors))
						input := strings.TrimSpace(readLine())
						for i, m := range availableMirrors {
							if input == fmt.Sprintf("%d", i+1) {
								mirrorRemote := platformRemoteName(m)
								mirrorAuth := authHeaderForPlatformName(m)
								fmt.Printf("  pushing to %s...\n", m)
								if merr := runPushWithAuth(mirrorRemote, branch, mirrorAuth); merr != nil {
									fmt.Printf("  %s push to %s failed: %s\n", colorRed("✗"), m, merr)
								} else {
									fmt.Printf("  %s pushed to %s/%s\n", colorGreen("✓"), mirrorRemote, branch)
								}
								break
							}
						}
					}
				} else {
					fmt.Printf("  %s push failed: %s\n", colorRed("✗"), err)
				}
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
