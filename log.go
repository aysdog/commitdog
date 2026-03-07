package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const logDefaultLimit = 20

var laneColors = []string{
	"\033[36m",
	"\033[32m",
	"\033[33m",
	"\033[35m",
	"\033[34m",
	"\033[31m",
	"\033[97m",
}

const resetColor = "\033[0m"
const dimColor = "\033[90m"
const boldColor = "\033[1m"
const refColor = "\033[33m"
const hashColor = "\033[96m"

type logCommit struct {
	hash    string
	short   string
	parents []string
	refs    string
	subject string
	when    string
}

type lane struct {
	hash     string
	colorIdx int
}

func laneColor(l lane) string {
	return laneColors[l.colorIdx%len(laneColors)]
}

func runLog() {
	if err := verifyGitRepo(); err != nil {
		fatal("not a git repository.")
	}
	showAll := false
	if len(os.Args) > 2 && os.Args[2] == "--all" {
		showAll = true
	}
	commits, err := fetchLogCommits(showAll)
	if err != nil {
		fatal("could not read git log: %v", err)
	}
	if len(commits) == 0 {
		fmt.Println("  no commits found.")
		return
	}
	lines := buildGraph(commits)
	runLogViewer(lines, showAll, len(commits))
}

func fetchLogCommits(all bool) ([]logCommit, error) {
	args := []string{"log", "--pretty=format:%H|%P|%h|%D|%s|%cr", "--topo-order", "--all"}
	if !all {
		args = append(args, fmt.Sprintf("-%d", logDefaultLimit))
	}
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	var commits []logCommit
	for _, line := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 6)
		if len(parts) < 6 {
			continue
		}
		var parents []string
		for _, p := range strings.Fields(parts[1]) {
			if p != "" {
				parents = append(parents, p)
			}
		}
		commits = append(commits, logCommit{
			hash:    parts[0],
			parents: parents,
			short:   parts[2],
			refs:    strings.TrimSpace(parts[3]),
			subject: strings.TrimSpace(parts[4]),
			when:    strings.TrimSpace(parts[5]),
		})
	}
	return commits, nil
}

func buildGraph(commits []logCommit) []string {
	lanes := []lane{}
	colorCounter := 0
	var output []string

	firstBranchName := inferBranchName(commits, 0)
	if firstBranchName != "" {
		c := laneColors[0]
		output = append(output, c+firstBranchName+resetColor)
		output = append(output, laneColors[0]+"│"+resetColor)
	}

	for ci, commit := range commits {
		col := findLane(lanes, commit.hash)
		if col == -1 {
			lanes = append(lanes, lane{hash: commit.hash, colorIdx: colorCounter % len(laneColors)})
			col = len(lanes) - 1
			colorCounter++
		}

		commitColor := laneColor(lanes[col])
		output = append(output, buildCommitRow(lanes, col, commit, commitColor))

		if ci == len(commits)-1 {
			break
		}

		oldLanes := make([]lane, len(lanes))
		copy(oldLanes, lanes)

		newLanes := updateLanes(lanes, col, commit, &colorCounter)

		// Branch opening: merge commit added a new lane to the right
		if len(newLanes) > len(oldLanes) {
			for d := len(oldLanes); d < len(newLanes); d++ {
				newLane := newLanes[d]
				var sb strings.Builder
				for i := 0; i < len(newLanes); i++ {
					l := newLanes[i]
					if i == col {
						sb.WriteString(laneColor(l) + "├" + resetColor)
						sb.WriteString(laneColor(newLane) + "─" + resetColor)
					} else if i == d {
						sb.WriteString(laneColor(newLane) + "╮" + resetColor)
						sb.WriteString(" ")
					} else if i > col && i < d {
						sb.WriteString(laneColor(newLane) + "─" + resetColor)
						sb.WriteString(laneColor(newLane) + "─" + resetColor)
					} else {
						sb.WriteString(laneColor(l) + "│" + resetColor)
						sb.WriteString(" ")
					}
				}
				output = append(output, sb.String())
			}
			output = append(output, buildConnector(newLanes))

			// Branch closing: a lane disappeared (its last commit was just processed)
		} else if len(newLanes) < len(oldLanes) {
			output = append(output, buildConnector(newLanes))

		} else {
			output = append(output, buildConnector(newLanes))
		}

		lanes = newLanes
	}

	return output
}

func findLane(lanes []lane, hash string) int {
	for i, l := range lanes {
		if l.hash == hash {
			return i
		}
	}
	return -1
}

func updateLanes(lanes []lane, col int, commit logCommit, colorCounter *int) []lane {
	result := make([]lane, len(lanes))
	copy(result, lanes)

	if len(commit.parents) == 0 {
		result = append(result[:col], result[col+1:]...)
		return result
	}

	firstParent := commit.parents[0]

	// Check if first parent already exists in another lane — branch rejoins
	existingCol := findLane(result, firstParent)
	if existingCol != -1 && existingCol != col {
		// This lane is merging back into an existing lane — remove this lane
		result = append(result[:col], result[col+1:]...)
		return result
	}

	result[col] = lane{hash: firstParent, colorIdx: lanes[col].colorIdx}

	// Handle merge commit second+ parents (branch opening)
	for _, p := range commit.parents[1:] {
		existing := findLane(result, p)
		if existing != -1 {
			// already tracked — remove it (it's being merged in)
			result = append(result[:existing], result[existing+1:]...)
		} else {
			result = append(result, lane{hash: p, colorIdx: *colorCounter % len(laneColors)})
			*colorCounter++
		}
	}

	return result
}

func buildCommitRow(lanes []lane, col int, commit logCommit, commitColor string) string {
	var sb strings.Builder
	for i, l := range lanes {
		if i == col {
			sb.WriteString(commitColor + "●" + resetColor)
		} else {
			sb.WriteString(laneColor(l) + "│" + resetColor)
		}
		sb.WriteString(" ")
	}

	subject := commit.subject
	if len(subject) > 50 {
		subject = subject[:47] + "..."
	}

	sb.WriteString(hashColor + boldColor + commit.short + resetColor)
	if commit.refs != "" {
		sb.WriteString(" " + refColor + "(" + cleanRefs(commit.refs) + ")" + resetColor)
	}
	sb.WriteString("  " + subject)
	sb.WriteString("  " + dimColor + commit.when + resetColor)
	return sb.String()
}

func buildConnector(lanes []lane) string {
	var sb strings.Builder
	for i, l := range lanes {
		sb.WriteString(laneColor(l) + "│" + resetColor)
		if i < len(lanes)-1 {
			sb.WriteString(" ")
		}
	}
	return sb.String()
}

func inferBranchName(commits []logCommit, idx int) string {
	if idx >= len(commits) {
		return ""
	}
	refs := commits[idx].refs
	if refs == "" {
		return ""
	}
	for _, ref := range strings.Split(refs, ", ") {
		ref = strings.TrimSpace(ref)
		if strings.HasPrefix(ref, "HEAD -> ") {
			return strings.TrimPrefix(ref, "HEAD -> ")
		}
	}
	for _, ref := range strings.Split(refs, ", ") {
		ref = strings.TrimSpace(ref)
		if !strings.HasPrefix(ref, "tag:") && ref != "HEAD" && !strings.Contains(ref, "HEAD") {
			return ref
		}
	}
	return ""
}

func cleanRefs(refs string) string {
	parts := strings.Split(refs, ", ")
	var clean []string
	seen := map[string]bool{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		clean = append(clean, p)
	}
	if len(clean) > 3 {
		clean = clean[:3]
	}
	return strings.Join(clean, ", ")
}

func runLogViewer(lines []string, showAll bool, total int) {
	offset := 0
	termH := terminalHeight()
	visibleH := termH - 4
	if visibleH < 5 {
		visibleH = 5
	}

	enableRawMode()
	defer disableRawMode()

	for {
		clearScreen()

		header := fmt.Sprintf("  \033[1mcommitdog log\033[0m  \033[90m%d commits", total)
		if !showAll {
			header += "  [a] show all"
		}
		header += "  [j/k] scroll  [q] quit\033[0m"
		fmt.Println(header)
		fmt.Println()

		end := offset + visibleH
		if end > len(lines) {
			end = len(lines)
		}
		for _, l := range lines[offset:end] {
			fmt.Println("  " + l)
		}

		maxOffset := len(lines) - visibleH
		if maxOffset < 0 {
			maxOffset = 0
		}

		b := make([]byte, 3)
		n, _ := os.Stdin.Read(b)
		if n == 0 {
			continue
		}

		switch {
		case b[0] == 'q':
			clearScreen()
			return
		case b[0] == 'j' || (n == 3 && b[0] == 27 && b[1] == 91 && b[2] == 66):
			if offset < maxOffset {
				offset++
			}
		case b[0] == 'k' || (n == 3 && b[0] == 27 && b[1] == 91 && b[2] == 65):
			if offset > 0 {
				offset--
			}
		case b[0] == 'a' && !showAll:
			disableRawMode()
			newCommits, err := fetchLogCommits(true)
			if err == nil {
				lines = buildGraph(newCommits)
				total = len(newCommits)
				showAll = true
				offset = 0
			}
			enableRawMode()
		}
	}
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}
