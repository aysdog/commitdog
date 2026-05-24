package main

import (
	"fmt"
	"os"
	"os/exec"
)

const version = "0.2.9"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "-v":
			printAsciiArt()
			os.Exit(0)
		case "help", "-h":
			printHelp()
			os.Exit(0)
		case "setup":
			runSetup()
			os.Exit(0)
		case "init":
			runInit()
			os.Exit(0)
		case "revert":
			runRevert()
			os.Exit(0)
		case "--update", "update":
			runUpdate()
			os.Exit(0)
		case "--uninstall":
			runUninstall()
			os.Exit(0)
		case "branch":
			runBranch()
			os.Exit(0)
		case "switch":
			if err := verifyGitRepo(); err != nil {
				fatal("not a git repository.")
			}
			runBranchSwitch()
			os.Exit(0)
		case "merge":
			runMerge()
			os.Exit(0)
		case "secrets":
			runSecretsHistoryScan()
			os.Exit(0)
		case "sync":
			runSync()
			os.Exit(0)
		case "stash":
			runStash()
			os.Exit(0)
		case "pr":
			runPR()
			os.Exit(0)
		case "release":
			runRelease()
			os.Exit(0)
		case "updatebrew":
			runUpdateBrew()
			os.Exit(0)
		case "updateaur":
			runUpdateAUR()
			os.Exit(0)
		case "-gh":
			runCommitFlow(os.Args[2:], "github")
			os.Exit(0)
		case "-gl":
			runCommitFlow(os.Args[2:], "gitlab")
			os.Exit(0)
		case "-gt":
			runCommitFlow(os.Args[2:], "gitea")
			os.Exit(0)
		case "-fg":
			runCommitFlow(os.Args[2:], "forgejo")
			os.Exit(0)
		default:
			platform, files := parsePlatformFlag(os.Args[1:])
			runCommitFlow(files, platform)
			os.Exit(0)
		}
	}

	checkFirstRun()

	if err := verifyGitRepo(); err != nil {
		fatal("not a git repository. run 'commitdog init' to create one.")
	}

	runCommitFlow(nil, "")
}

func parsePlatformFlag(args []string) (string, []string) {
	var platform string
	var rest []string
	for _, arg := range args {
		switch arg {
		case "-gh":
			platform = "github"
		case "-gl":
			platform = "gitlab"
		case "-gt":
			platform = "gitea"
		case "-fg":
			platform = "forgejo"
		default:
			rest = append(rest, arg)
		}
	}
	return platform, rest
}

func runCommitFlow(files []string, platform string) {
	if err := verifyGitRepo(); err != nil {
		fatal("not a git repository. run 'commitdog init' to create one.")
	}

	if len(files) > 0 {
		for _, f := range files {
			if _, err := os.Stat(f); os.IsNotExist(err) {
				fatal("file not found: %s", f)
			}
		}
		if err := stageFiles(files); err != nil {
			fatal("staging failed: %v", err)
		}
		stageUntrackedEmpty()
	} else {
		diff, err := getStagedDiff()
		if err != nil {
			fatal("failed to read staged diff: %v", err)
		}
		if diff == "" {
			fmt.Println("  nothing staged. staging all changes...")
			if err := stageFiles([]string{"."}); err != nil {
				fatal("staging failed: %v", err)
			}
		}
		stageUntrackedEmpty()
	}

	diff, err := getStagedDiff()
	if err != nil {
		fatal("failed to read staged diff: %v", err)
	}

	stagedNew := getStagedNewFiles()

	if diff == "" && len(stagedNew) == 0 {
		warnEmptyDirs()
		proj := loadProjectConfig()
		targetPlatform := platform
		if targetPlatform == "" {
			targetPlatform = proj.effectivePrimary()
		}
		if targetPlatform == "" {
			targetPlatform = "github"
		}
		remote := remoteForPlatform(targetPlatform)
		branch := getCurrentBranch()
		if hasUnpushedCommits(remote, branch) {
			fmt.Printf("  nothing to commit — pushing unpushed commits to %s...\n", targetPlatform)
			askPush(platform)
			return
		}
		proj2 := loadProjectConfig()
		platformFlags := map[string]string{
			"github":  "gh",
			"gitlab":  "gl",
			"gitea":   "gt",
			"forgejo": "fg",
		}
		for _, m := range proj2.mirrors {
			mr := remoteForPlatform(m)
			if hasUnpushedCommits(mr, branch) {
				fmt.Printf("  up to date with %s\n", targetPlatform)
				fmt.Printf("  %s has unpushed commits — run 'commitdog -%s' to push\n\n", m, platformFlags[m])
				return
			}
		}
		fatal("nothing to commit — no changes found.")
	}

	a := analyzeDiffWithBranch(diff, getCurrentBranch())

	for _, f := range stagedNew {
		a.filesAdded = appendUnique(a.filesAdded, f)
		a.isNewFiles = true
	}

	if a.filesChanged == 0 && !a.isNewFiles {
		fatal("no changes detected in staged diff.")
	}
	if a.filesChanged == 0 && a.isNewFiles {
		a.filesChanged = len(a.filesAdded)
	}

	suggestions := generateSuggestions(a)

	if !checkSecretsInDiff(diff) {
		os.Exit(1)
	}

	chosen := pickSuggestion(suggestions)
	if chosen == "" {
		exec.Command("git", "restore", "--staged", ".").Run()
		fmt.Println()
		fmt.Println("  aborted.")
		fmt.Printf("  %s staged changes removed\n\n", colorYellow("⚠"))
		os.Exit(0)
	}

	if err := runCommit(chosen); err != nil {
		fatal("commit failed: %v", err)
	}

	fmt.Printf("\n  ✓ committed: %s\n", chosen)

	askPush(platform)
}

func printHelp() {
	fmt.Println(`commitdog — zero-bs git workflow CLI

usage:
  commitdog                 stage all changes and generate commit message
  commitdog <file>          stage specific file and generate commit message
  commitdog -gl/-gt/-fg     commit and push to gitlab / gitea / forgejo
  commitdog --uninstall     remove commitdog from this device completely

  commitdog init            create a new GitHub repo and first push
  commitdog setup           configure email and GitHub token

  commitdog revert          pick from last 5 commits and revert

  commitdog branch          1 switch  2 create new  3 delete
  commitdog switch          go straight to branch switcher
  commitdog branch ls       list all local + remote branches
  commitdog branch create   create branch (suggests names from staged diff)
  commitdog branch delete   delete local branch (+ optionally remote)

  commitdog merge           merge a branch into current with preview

  commitdog pr              on feature branch: create PR with diff preview
                            on main: list open PRs, view diff, merge, close

  commitdog secrets          scan full commit history for leaked secrets

  commitdog sync            fetch + pull rebase + push in one command

  commitdog stash           save/pop stashes interactively
                            if stashes exist: pick to pop, d# to drop, s to save
                            if no stashes: goes straight to save

  commitdog release         bump version, build, tag, push, create GitHub release
                            auto-detects Go / Node.js / Rust / Python / Java

  commitdog release config    configure build targets

  commitdog --update        update to latest version
  commitdog version         show version and logo
  commitdog help            show this help

workflow:
  first time:
    commitdog setup         - set email + GitHub token once
    mkdir my-project && cd my-project
    commitdog init          - creates repo on GitHub, first commit, push

  daily:
    commitdog               - stages everything, suggests message, commits
    commitdog file.go       - stage one file, suggest message, commit
    commitdog sync          - fetch + rebase + push in one shot
    commitdog branch        - switch / create / delete
    commitdog switch        - straight to branch picker
    commitdog stash         - save work in progress

  branching:
    commitdog branch ls     - see all branches
    commitdog branch create - new branch, suggests names from diff
    commitdog switch        - pick from recent or type name
    commitdog branch delete - safe delete with unmerged warning

  pull requests:
    commitdog pr            - create PR from feature branch with diff preview
    commitdog pr            - list + review + merge PRs when on main

  releasing:
    commitdog release       - bump version, build, tag, push, upload to GitHub

  oops:
    commitdog revert        - pick a commit to revert and push`)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "\n  commitdog: "+format+"\n\n", args...)
	os.Exit(1)
}
