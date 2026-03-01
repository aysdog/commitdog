package main

import (
	"fmt"
	"os"
)

const version = "0.1.4"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			printAsciiArt()
			fmt.Println()
			fmt.Println("commitdog v" + version)
			fmt.Println("zero-bs commits · no AI · no telemetry")
			fmt.Println("aysdog.com")
			os.Exit(0)
		case "--help", "-h":
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
		case "branch":
			runBranch()
			os.Exit(0)
		case "sync":
			runSync()
			os.Exit(0)
		case "stash":
			runStash()
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
			fmt.Fprintf(os.Stderr, "run 'commitdog --help' for usage.\n")
			os.Exit(1)
		}
	}

	checkFirstRun()

	if err := verifyGitRepo(); err != nil {
		fatal("not a git repository. run 'commitdog init' to create one.")
	}

	diff, err := getStagedDiff()
	if err != nil {
		fatal("failed to read staged diff: %v", err)
	}

	if diff == "" {
		fatal("nothing staged. run 'git add .' first.")
	}

	a := analyzeDiff(diff)

	if a.filesChanged == 0 && !a.isNewFiles {
		fatal("no changes detected in staged diff.")
	}

	if a.filesChanged == 0 && a.isNewFiles {
		a.filesChanged = len(a.filesAdded)
	}

	suggestions := generateSuggestions(a)

	chosen := pickSuggestion(suggestions)
	if chosen == "" {
		fmt.Println("  aborted.")
		os.Exit(0)
	}

	if err := runCommit(chosen); err != nil {
		fatal("commit failed: %v", err)
	}

	fmt.Printf("\n  ✓ committed: %s\n", chosen)

	askPush()
}

func printHelp() {
	fmt.Println(`commitdog — zero-bs commit message generator

usage:
  commitdog                 generate commit message from staged diff

  commitdog init            create a new GitHub repo and first push
  commitdog setup           configure email and GitHub token

  commitdog revert          pick from last 5 commits and revert

  commitdog branch          interactive branch switcher (shows 5 + edit)
  commitdog branch switch   same as above
  commitdog branch ls       list all local + remote branches
  commitdog branch create   create a new branch with optional base
  commitdog branch delete   delete local branch (+ optionally remote)

  commitdog sync            fetch + pull rebase + push in one command

  commitdog stash           save/pop stashes interactively
                            if stashes exist: pick to pop, d# to drop, s to save
                            if no stashes: goes straight to save

  commitdog --update        update to latest version
  commitdog --version       show version and logo
  commitdog --help          show this help

workflow:
  first time:
    commitdog setup         ← set email + GitHub token once
    mkdir my-project && cd my-project
    commitdog init          ← creates repo on GitHub, first commit, push

  daily:
    git add .
    commitdog               ← suggests message, commits, asks to push
    commitdog sync          ← fetch + rebase + push in one shot
    commitdog branch        ← switch branches fast
    commitdog stash         ← save work in progress

  branching:
    commitdog branch ls     ← see all branches
    commitdog branch create ← new branch, optional base, push
    commitdog branch switch ← pick from 5 recent or type name
    commitdog branch delete ← safe delete with unmerged warning

  oops:
    commitdog revert        ← pick a commit to revert and push

no ai. no network (except push/init). no telemetry. just works.`)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "\n  commitdog: "+format+"\n\n", args...)
	os.Exit(1)
}
