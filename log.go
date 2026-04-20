package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const logDefaultLimit = 20

const colorMain = 0

var laneColors = []string{
	"\033[38;2;30;100;255m",
	"\033[38;2;50;220;100m",
	"\033[38;2;255;200;0m",
	"\033[38;2;255;60;60m",
	"\033[38;2;50;220;100m",
	"\033[38;2;50;220;100m",
	"\033[38;2;50;220;100m",
	"\033[38;2;50;220;100m",
}

const resetColor = "\033[0m"
const dimColor = "\033[90m"
const refColor = "\033[38;2;255;200;0m"
const hashColor = "\033[1;38;2;100;200;255m"

func lc(idx int) string {
	return laneColors[idx%len(laneColors)]
}

type gCommit struct {
	hash    string
	short   string
	parents []string
	refs    string
	subject string
	when    string
	isMerge bool
}

func runLog() {
	if err := verifyGitRepo(); err != nil {
		fatal("not a git repository.")
	}
	showAll := false
	if len(os.Args) > 2 && os.Args[2] == "--all" {
		showAll = true
	}
	commits, err := fetchGCommits(showAll)
	if err != nil {
		fatal("could not read git log: %v", err)
	}
	if len(commits) == 0 {
		fmt.Println("  no commits found.")
		return
	}
	lines := buildGGraph(commits)
	runLogViewer(lines, showAll, len(commits))
}

func fetchGCommits(all bool) ([]gCommit, error) {
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
	var commits []gCommit
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
		commits = append(commits, gCommit{
			hash:    parts[0],
			parents: parents,
			short:   parts[2],
			refs:    strings.TrimSpace(parts[3]),
			subject: strings.TrimSpace(parts[4]),
			when:    strings.TrimSpace(parts[5]),
			isMerge: len(parents) > 1,
		})
	}
	return commits, nil
}

func buildGGraph(commits []gCommit) []string {
	active := map[int]int{}
	colAssign := map[string]int{}
	colorCtr := 0

	allocCol := func() (int, int) {
		c := 1
		for {
			if _, used := active[c]; !used {
				break
			}
			c++
		}
		ci := 1 + (colorCtr % (len(laneColors) - 1))
		colorCtr++
		return c, ci
	}

	byHash := map[string]int{}
	for i, c := range commits {
		byHash[c.hash] = i
	}

	mergedSet := map[string]bool{}
	pendingSet := map[string]bool{}
	deletedSet := map[string]bool{}
	allRefs := map[string]bool{}
	for _, c := range commits {
		if c.refs != "" {
			allRefs[c.hash] = true
		}
	}
	for _, c := range commits {
		if len(c.parents) > 1 {
			p2 := c.parents[1]
			mergedSet[p2] = true
		}
	}
	for _, c := range commits {
		if !c.isMerge && len(c.parents) > 0 && allRefs[c.hash] {
			isMerged := mergedSet[c.hash]
			if !isMerged {
				hasRemote := strings.Contains(c.refs, "origin/")
				if hasRemote {
					pendingSet[c.hash] = true
				} else {
					deletedSet[c.hash] = true
				}
			}
		}
	}
	mainColor := colorMain
	active[0] = mainColor

	if len(commits) > 0 {
		cur := commits[0].hash
		for {
			colAssign[cur] = 0
			ci, ok := byHash[cur]
			if !ok || len(commits[ci].parents) == 0 {
				break
			}
			c := commits[ci]
			next := c.parents[0]
			if len(c.parents) > 1 {
				for _, p := range c.parents {
					if pi, ok2 := byHash[p]; ok2 {
						refs := commits[pi].refs
						if strings.Contains(refs, "origin/main") ||
							strings.Contains(refs, "origin/master") ||
							strings.Contains(refs, "main") {
							next = p
							break
						}
					}
				}
			}
			cur = next
		}
	}

	type rowData struct {
		commit     gCommit
		col        int
		colorIdx   int
		snapActive map[int]int
		postActive map[int]int
		p1col      int
		p2col      int
	}

	rows := make([]rowData, 0, len(commits))

	for _, c := range commits {
		col, ok := colAssign[c.hash]
		var colorIdx int
		if !ok {
			col, colorIdx = allocCol()
			active[col] = colorIdx
			colAssign[c.hash] = col
		} else {
			colorIdx = active[col]
			if colorIdx == 0 && col == 0 {
				colorIdx = mainColor
			}
		}

		snapActive := copyIntMap(active)
		p1col := -1
		p2col := -1

		if len(c.parents) == 0 {
			delete(active, col)
		} else {
			p1 := c.parents[0]
			if existCol, exists := colAssign[p1]; exists && existCol != col {
				p1col = existCol
				delete(active, col)
			} else {
				colAssign[p1] = col
				p1col = col
			}
			if len(c.parents) > 1 {
				p2 := c.parents[1]
				if existCol, exists := colAssign[p2]; exists {
					p2col = existCol
				} else {
					nc, nci := allocCol()
					active[nc] = nci
					colAssign[p2] = nc
					p2col = nc
				}
			}
		}

		rows = append(rows, rowData{
			commit:     c,
			col:        col,
			colorIdx:   colorIdx,
			snapActive: snapActive,
			postActive: copyIntMap(active),
			p1col:      p1col,
			p2col:      p2col,
		})
	}
	maxW := 1
	for _, r := range rows {
		for c := range r.snapActive {
			if c+1 > maxW {
				maxW = c + 1
			}
		}
		if r.col+1 > maxW {
			maxW = r.col + 1
		}
		if r.p2col+1 > maxW {
			maxW = r.p2col + 1
		}
	}

	var out []string
	for ri, r := range rows {
		out = append(out, renderCommitLine(r.snapActive, maxW, r.col, r.colorIdx, r.commit))

		if ri >= len(rows)-1 {
			continue
		}

		hasFork := r.p2col != -1 && r.p2col != r.col
		hasConverge := r.p1col != -1 && r.p1col != r.col

		if hasFork {
			out = append(out, renderForkLine(r.postActive, maxW, r.col, r.p2col, r.colorIdx))
			out = append(out, renderStraightLine(r.postActive, maxW))
		} else if hasConverge {
			out = append(out, renderConvergeLine(r.postActive, maxW, r.col, r.p1col, r.colorIdx))
		} else {
			out = append(out, renderStraightLine(r.postActive, maxW))
		}
	}

	return out
}
func renderCommitLine(active map[int]int, w, col, colorIdx int, c gCommit) string {
	var sb strings.Builder
	for i := 0; i < w; i++ {
		if i == col {
			dot := "● "
			if c.isMerge {
				dot = "● "
			}
			sb.WriteString(lc(colorIdx) + dot + resetColor)
		} else if ci, ok := active[i]; ok {
			sb.WriteString(lc(ci) + "│ " + resetColor)
		} else {
			sb.WriteString("  ")
		}
	}

	subject := c.subject
	if len(subject) > 55 {
		subject = subject[:52] + "..."
	}
	sb.WriteString(hashColor + c.short + resetColor)
	if c.refs != "" {
		sb.WriteString(" " + refColor + "(" + cleanRefs(c.refs) + ")" + resetColor)
	}
	sb.WriteString("  " + subject)
	sb.WriteString("  " + dimColor + c.when + resetColor)
	return sb.String()
}
func renderStraightLine(active map[int]int, w int) string {
	var sb strings.Builder
	for i := 0; i < w; i++ {
		if ci, ok := active[i]; ok {
			sb.WriteString(lc(ci) + "│ " + resetColor)
		} else {
			sb.WriteString("  ")
		}
	}
	return sb.String()
}
func renderForkLine(postActive map[int]int, w, col, p2col, colorIdx int) string {
	size := w
	if p2col+1 > size {
		size = p2col + 1
	}
	cells := make([]string, size)
	for i := 0; i < size; i++ {
		if ci, ok := postActive[i]; ok {
			cells[i] = lc(ci) + "│ " + resetColor
		} else {
			cells[i] = "  "
		}
	}
	cells[col] = lc(colorIdx) + "├─" + resetColor
	for i := col + 1; i < p2col && i < size; i++ {
		cells[i] = lc(colorIdx) + "──" + resetColor
	}
	if p2col < size {
		newCI := colorIdx
		if v, ok := postActive[p2col]; ok {
			newCI = v
		}
		cells[p2col] = lc(newCI) + "┐ " + resetColor
	}

	return strings.Join(cells, "")
}
func renderConvergeLine(postActive map[int]int, w, col, p1col, colorIdx int) string {
	size := w
	cells := make([]string, size)
	for i := 0; i < size; i++ {
		if ci, ok := postActive[i]; ok {
			cells[i] = lc(ci) + "│ " + resetColor
		} else {
			cells[i] = "  "
		}
	}
	if col > p1col {
		cells[col] = lc(colorIdx) + "┘ " + resetColor
		for i := p1col + 1; i < col && i < size; i++ {
			cells[i] = lc(colorIdx) + "──" + resetColor
		}
	} else {
		cells[col] = lc(colorIdx) + "└─" + resetColor
		for i := col + 1; i < p1col && i < size; i++ {
			cells[i] = lc(colorIdx) + "──" + resetColor
		}
	}

	return strings.Join(cells, "")
}

func copyIntMap(m map[int]int) map[int]int {
	c := make(map[int]int, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
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
		case isDownKey(b, n):
			if offset < maxOffset {
				offset++
			}
		case isUpKey(b, n):
			if offset > 0 {
				offset--
			}
		case b[0] == 'a' && !showAll:
			disableRawMode()
			newCommits, err := fetchGCommits(true)
			if err == nil {
				lines = buildGGraph(newCommits)
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
