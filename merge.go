package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type mergeBranch struct {
	name      string
	added     int
	removed   int
	files     int
	conflicts bool
}

func runMerge() {
	if err := verifyGitRepo(); err != nil {
		fatal("not a git repository.")
	}

	current := getCurrentBranch()
	if current == "" || current == "HEAD" {
		fatal("not on a branch.")
	}

	branches, err := getMergeableBranches(current)
	if err != nil {
		fatal("could not list branches: %v", err)
	}
	if len(branches) == 0 {
		fmt.Println()
		fmt.Println("  no other branches to merge.")
		fmt.Println()
		return
	}

	stats, err := buildMergeStats(branches, current)
	if err != nil {
		fatal("could not read branch stats: %v", err)
	}
	if len(stats) == 0 {
		fmt.Println()
		fmt.Println("  no branches with changes to merge into " + current + ".")
		fmt.Println()
		return
	}

	fmt.Println()
	fmt.Printf("  merge into %s:\n", colorYellow(current))
	fmt.Println()

	for i, s := range stats {
		status := colorMuted("clean")
		if s.conflicts {
			status = colorRed("conflict")
		}
		fmt.Printf("  %d  %-28s +%-4d -%-4d  %d files   %s\n",
			i+1, s.name, s.added, s.removed, s.files, status)
	}

	fmt.Println()
	fmt.Printf("  [1-%d] pick, [q] quit › ", len(stats))

	for {
		input := readLine()
		switch input {
		case "q", "quit", "exit":
			fmt.Println("  aborted.")
			return
		}
		for i, s := range stats {
			if input == fmt.Sprintf("%d", i+1) {
				doMerge(s, current)
				return
			}
		}
		fmt.Printf("  enter 1-%d or q › ", len(stats))
	}
}

func doMerge(s mergeBranch, into string) {
	fmt.Println()
	fmt.Printf("  merging %s into %s\n", s.name, into)
	fmt.Println()

	fileStats, err := getMergeFileStat(s.name, into)
	if err == nil && len(fileStats) > 0 {
		for _, line := range fileStats {
			fmt.Printf("  %s\n", line)
		}
		fmt.Println()
	}

	if s.conflicts {
		fmt.Printf("  %s this merge has conflicts.\n", colorRed("⚠"))
		fmt.Println()
		fmt.Println("  1  merge and open conflicts in editor")
		fmt.Println("  2  cancel")
		fmt.Println()
		fmt.Printf("  [1/2] › ")
		for {
			input := readLine()
			switch input {
			case "1":
				doMergeWithConflicts(s.name)
				return
			case "2", "q":
				fmt.Println("  aborted.")
				return
			default:
				fmt.Printf("  1 or 2 › ")
			}
		}
	}

	fmt.Printf("  [1] merge   [2] view diff   [3] cancel › ")
	for {
		input := readLine()
		switch input {
		case "1":
			fmt.Printf("  merging...")
			if err := gitMerge(s.name); err != nil {
				fmt.Println()
				fatal("merge failed: %v", err)
			}
			fmt.Println(" done")

			bc, err := collectBranchChanges(s.name, into)
			if err == nil && bc.total > 0 {
				mergeMsg := generateMergeCommitMsg(bc, into)
				fmt.Println()
				fmt.Println(colorMuted("  ─────────────────────────────────"))
				for _, line := range strings.Split(mergeMsg, "\n") {
					fmt.Printf("  %s\n", line)
				}
				fmt.Println(colorMuted("  ─────────────────────────────────"))
				fmt.Println()
				fmt.Printf("  [enter] use this message  [e] edit  [s] skip › ")
				input := strings.ToLower(strings.TrimSpace(readLine()))
				switch input {
				case "e":
					edited, err := openInEditor(mergeMsg)
					if err == nil {
						mergeMsg = strings.TrimSpace(edited)
					}
					exec.Command("git", "commit", "--amend", "-m", mergeMsg).Run()
				case "s":
					break
				default:
					exec.Command("git", "commit", "--amend", "-m", mergeMsg).Run()
				}
			}

			fmt.Printf("  %s merged %s into %s\n", colorGreen("✓"), s.name, into)
			askPush("")
			return
		case "2":
			showMergeDiff(s.name, into)
			fmt.Printf("  [1] merge   [2] view diff   [3] cancel › ")
		case "3", "q":
			fmt.Println("  aborted.")
			return
		default:
			fmt.Printf("  1, 2, or 3 › ")
		}
	}
}

func doMergeWithConflicts(branch string) {
	fmt.Printf("  merging (conflicts expected)...")
	gitMergeNoCommit(branch)
	fmt.Println(" done")

	conflicted, err := getConflictedFiles()
	if err != nil || len(conflicted) == 0 {
		fmt.Println("  no conflict files found — run 'git status' to check.")
		return
	}

	fmt.Println()
	fmt.Println("  conflicted files:")
	fmt.Println()
	for i, f := range conflicted {
		fmt.Printf("  %d  %s\n", i+1, f)
	}
	fmt.Println()

	if len(conflicted) == 1 {
		fmt.Printf("  opening %s in editor...\n", conflicted[0])
		openFileInEditor(conflicted[0])
	} else {
		fmt.Printf("  [1-%d] open file, [q] quit › ", len(conflicted))
		for {
			input := readLine()
			if input == "q" {
				break
			}
			for i, f := range conflicted {
				if input == fmt.Sprintf("%d", i+1) {
					openFileInEditor(f)
					fmt.Printf("  [1-%d] open another, [q] done › ", len(conflicted))
					break
				}
			}
		}
	}

	fmt.Println()
	fmt.Println("  after resolving all conflicts:")
	fmt.Println("    git add .")
	fmt.Println("    git commit")
	fmt.Println()
}

func getMergeableBranches(current string) ([]string, error) {
	all, err := getAllBranches()
	if err != nil {
		return nil, err
	}
	var result []string
	for _, b := range all {
		if b != current {
			result = append(result, b)
		}
	}
	return result, nil
}

func buildMergeStats(branches []string, into string) ([]mergeBranch, error) {
	var result []mergeBranch
	for _, b := range branches {
		added, removed, files, err := getDiffStat(b, into)
		if err != nil {
			continue
		}
		if files == 0 {
			continue
		}
		conflicts := checkWouldConflict(b)
		result = append(result, mergeBranch{
			name:      b,
			added:     added,
			removed:   removed,
			files:     files,
			conflicts: conflicts,
		})
	}
	return result, nil
}

func getDiffStat(branch, base string) (added, removed, files int, err error) {
	ref := base + "..." + branch
	cmd := exec.Command("git", "diff", "--shortstat", "--no-color", ref)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err = cmd.Run(); err != nil {
		return
	}
	line := strings.TrimSpace(out.String())
	if line == "" {
		return
	}
	parts := strings.Split(line, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		fields := strings.Fields(p)
		if len(fields) < 2 {
			continue
		}
		n, e := strconv.Atoi(fields[0])
		if e != nil {
			continue
		}
		switch {
		case strings.Contains(p, "file"):
			files = n
		case strings.Contains(p, "insertion"):
			added = n
		case strings.Contains(p, "deletion"):
			removed = n
		}
	}
	return
}

func getMergeFileStat(branch, base string) ([]string, error) {
	ref := base + "..." + branch
	cmd := exec.Command("git", "diff", "--stat", "--no-color", ref)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "changed") || strings.Contains(line, "file changed") {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) > 10 {
		extra := len(lines) - 10
		lines = lines[:10]
		lines = append(lines, fmt.Sprintf("... and %d more files", extra))
	}
	return lines, nil
}

func checkWouldConflict(branch string) bool {
	cmd := exec.Command("git", "merge", "--no-commit", "--no-ff", branch)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	gitMergeAbort()
	if err != nil {
		msg := strings.ToLower(stderr.String())
		if strings.Contains(msg, "conflict") || strings.Contains(msg, "merge conflict") {
			return true
		}
	}
	return false
}

func gitMerge(branch string) error {
	if !isSafeGitRef(branch) {
		return fmt.Errorf("invalid branch name")
	}
	cmd := exec.Command("git", "merge", "--no-ff", branch)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func gitMergeNoCommit(branch string) {
	if !isSafeGitRef(branch) {
		return
	}
	cmd := exec.Command("git", "merge", "--no-commit", "--no-ff", branch)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	cmd.Run()
}

func gitMergeAbort() {
	cmd := exec.Command("git", "merge", "--abort")
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	cmd.Run()
}

func getConflictedFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

func openFileInEditor(file string) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, file)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func showMergeDiff(branch, base string) {
	ref := base + "..." + branch
	cmd := exec.Command("git", "diff", "--no-color", ref)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		fmt.Println("  could not get diff.")
		return
	}
	lines := strings.Split(out.String(), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		fmt.Println("  no diff found.")
		return
	}
	fmt.Println()
	pageSize := 40
	start := 0
	for {
		end := start + pageSize
		if end > len(lines) {
			end = len(lines)
		}
		for _, l := range lines[start:end] {
			switch {
			case strings.HasPrefix(l, "+") && !strings.HasPrefix(l, "+++"):
				fmt.Println("  " + colorGreen(l))
			case strings.HasPrefix(l, "-") && !strings.HasPrefix(l, "---"):
				fmt.Println("  " + colorRed(l))
			case strings.HasPrefix(l, "@@"):
				fmt.Println("  " + colorMuted(l))
			default:
				fmt.Println("  " + l)
			}
		}
		if end >= len(lines) {
			fmt.Println()
			fmt.Printf("  [enter] back › ")
			readLine()
			break
		}
		start = end
		fmt.Printf("  [enter] more   [q] back › ")
		input := readLine()
		if input == "q" {
			fmt.Println()
			break
		}
	}
}
