package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	sReset  = "\033[0m"
	sBold   = "\033[1m"
	sDim    = "\033[90m"
	sBlue   = "\033[38;2;30;100;255m"
	sGreen  = "\033[38;2;50;220;100m"
	sYellow = "\033[38;2;255;200;0m"
	sRed    = "\033[38;2;255;60;60m"
	sCyan   = "\033[38;2;100;200;255m"
	sGray   = "\033[38;2;140;140;140m"
	sBox    = "\033[38;2;70;70;80m"
)

func runStatus() {
	if err := verifyGitRepo(); err != nil {
		fatal("not a git repository.")
	}

	type result struct {
		commits     []statusCommit
		branches    []statusBranch
		prs         []statusPR
		repoName    string
		version     string
		lastRelease string
	}

	ch := make(chan result, 1)
	go func() {
		repoName := statusRepoName()
		r := result{
			commits:     statusCommits(),
			branches:    statusBranches(),
			prs:         statusPRs(),
			repoName:    repoName,
			version:     statusVersion(),
			lastRelease: statusLastRelease(),
		}
		ch <- r
	}()

	fmt.Print("  loading...")
	r := <-ch
	fmt.Print("\r                \r")

	termW := terminalWidth()
	if termW < 60 {
		termW = 60
	}

	usable := termW - 4
	leftW := usable * 40 / 100
	rightW := usable - leftW - 2

	fmt.Println()
	fmt.Println("  " + sBold + "commitdog status" + sReset + "  " + sDim + r.repoName + sReset)
	fmt.Println()

	left := stackBoxes([][]string{
		renderCommitsBox(r.commits, leftW, 9),
		renderPRBox(r.prs, leftW, 5),
	})
	right := stackBoxes([][]string{
		renderInfoBox(r.repoName, r.version, r.lastRelease, rightW, 4),
		renderBranchBox(r.branches, rightW, 6),
		renderIssuesBox(rightW, 5),
	})

	maxH := len(left)
	if len(right) > maxH {
		maxH = len(right)
	}
	for i := 0; i < maxH; i++ {
		fmt.Print("  ")
		if i < len(left) {
			fmt.Print(left[i])
		} else {
			fmt.Print(strings.Repeat(" ", leftW))
		}
		fmt.Print("  ")
		if i < len(right) {
			fmt.Print(right[i])
		}
		fmt.Println()
	}

	fmt.Println()
}

func stackBoxes(boxes [][]string) []string {
	var result []string
	for _, box := range boxes {
		result = append(result, box...)
	}
	return result
}

func boxLines(title string, w, fixedH int, content []string) []string {
	inner := w - 2
	contentW := inner - 2

	rows := fixedH
	if rows == 0 {
		rows = len(content)
	}
	if rows == 0 {
		rows = 1
	}

	var lines []string
	lines = append(lines, sBox+"┌"+strings.Repeat("─", inner)+"┐"+sReset)
	titlePad := inner - 1 - visibleLen(title)
	if titlePad < 0 {
		titlePad = 0
	}
	lines = append(lines, sBox+"│"+sReset+" "+sBold+title+sReset+strings.Repeat(" ", titlePad)+sBox+"│"+sReset)
	lines = append(lines, sBox+"├"+strings.Repeat("─", inner)+"┤"+sReset)

	for i := 0; i < rows; i++ {
		body := ""
		if i < len(content) {
			body = content[i]
		}
		body = truncate(body, contentW)
		lines = append(lines, sBox+"│"+sReset+" "+padRight(body, contentW)+" "+sBox+"│"+sReset)
	}

	lines = append(lines, sBox+"└"+strings.Repeat("─", inner)+"┘"+sReset)
	return lines
}

func visibleLen(s string) int {
	inEsc := false
	count := 0
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\033' {
			inEsc = true
		}
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		count++
	}
	return count
}

func padRight(s string, width int) string {
	pad := width - visibleLen(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

func dotLine(color, hash, msg string, maxMsg int) string {
	msg = truncate(msg, maxMsg)
	return color + "● " + sReset + sDim + hash + sReset + " " + msg
}

func renderCommitsBox(commits []statusCommit, w, h int) []string {
	contentW := w - 4
	var content []string
	max := 5
	if len(commits) < max {
		max = len(commits)
	}
	for i := 0; i < max; i++ {
		line := sBlue + "● " + sReset + sDim + commits[i].short + sReset + " " + truncate(commits[i].subject, contentW-9)
		content = append(content, line)
		if i < max-1 {
			content = append(content, sBlue+"│"+sReset)
		}
	}
	return boxLines("latest commits:", w, h, content)
}

func renderInfoBox(repo, version, lastRelease string, w, h int) []string {
	shortRepo := repo
	if idx := strings.LastIndex(repo, "/"); idx != -1 {
		shortRepo = repo[idx+1:]
	}
	kv := func(key, val string) string {
		return sDim + key + sReset + " " + sCyan + val + sReset
	}
	return boxLines("project info:", w, h, []string{
		kv("repo:   ", shortRepo),
		kv("version:", version),
		kv("release:", lastRelease),
	})
}

func renderBranchBox(branches []statusBranch, w, h int) []string {
	var content []string
	content = append(content, sBlue+"main"+sReset)
	if len(branches) == 0 {
		content = append(content, sDim+"no other branches"+sReset)
	}
	for _, b := range branches {
		col := sGreen
		if !b.merged && !b.remote {
			col = sRed
		} else if !b.merged && b.remote {
			col = sYellow
		}
		content = append(content, col+b.name+sReset)
	}
	return boxLines("branch list:", w, h, content)
}

func renderIssuesBox(w, h int) []string {
	issues := statusIssues()
	var content []string
	if len(issues) == 0 {
		content = append(content, sDim+"no open issues"+sReset)
	}
	for i, iss := range issues {
		if i >= 5 {
			break
		}
		content = append(content, fmt.Sprintf(sDim+"#%d"+sReset+"  %s", iss.number, iss.title))
	}
	return boxLines("issues:", w, h, content)
}

func renderPRBox(prs []statusPR, w, h int) []string {
	var content []string
	if len(prs) == 0 {
		content = append(content, sDim+"no open pull requests"+sReset)
	}
	for i, pr := range prs {
		if i >= 5 {
			break
		}
		content = append(content, fmt.Sprintf(sDim+"#%d"+sReset+"  %s", pr.number, pr.title))
	}
	return boxLines("pending pr's:", w, h, content)
}

type statusCommit struct {
	short   string
	subject string
}

type statusPR struct {
	number int
	title  string
}

type statusIssue struct {
	number int
	title  string
}

type statusBranch struct {
	name   string
	merged bool
	remote bool
}

func statusCommits() []statusCommit {
	out := gitOut("log", "--pretty=format:%h %s", "-5")
	var result []statusCommit
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			result = append(result, statusCommit{short: parts[0], subject: parts[1]})
		}
	}
	return result
}

func statusBranches() []statusBranch {
	out := gitOut("branch", "-a", "--format=%(refname:short)|%(upstream:short)")
	mergedOut := gitOut("branch", "--merged", "HEAD", "--format=%(refname:short)")
	mergedSet := map[string]bool{}
	for _, b := range strings.Split(mergedOut, "\n") {
		mergedSet[strings.TrimSpace(b)] = true
	}
	var result []statusBranch
	seen := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "|", 2)
		if len(parts) == 0 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		if name == "" || strings.HasPrefix(name, "origin/HEAD") {
			continue
		}
		short := name
		if strings.HasPrefix(name, "origin/") {
			short = strings.TrimPrefix(name, "origin/")
		}
		if short == "main" || short == "master" || short == "origin" || seen[short] {
			continue
		}
		seen[short] = true
		hasRemote := len(parts) > 1 && strings.TrimSpace(parts[1]) != ""
		result = append(result, statusBranch{
			name:   short,
			merged: mergedSet[short],
			remote: hasRemote || strings.HasPrefix(name, "origin/"),
		})
		if len(result) >= 7 {
			break
		}
	}
	return result
}

func statusPRs() []statusPR {
	cmd := exec.Command("gh", "pr", "list", "--limit", "4", "--state", "open", "--json", "number,title")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil
	}
	raw := strings.TrimSpace(out.String())
	if raw == "" || raw == "[]" || raw == "null" {
		return nil
	}
	var prs []statusPR
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "number") {
			continue
		}
		var num int
		var title string
		for _, part := range strings.Split(line, ",") {
			part = strings.TrimSpace(strings.Trim(part, "{}[]"))
			if strings.HasPrefix(part, `"number"`) {
				fmt.Sscanf(strings.SplitN(part, ":", 2)[1], "%d", &num)
			}
			if strings.HasPrefix(part, `"title"`) {
				title = strings.Trim(strings.SplitN(part, ":", 2)[1], `" `)
			}
		}
		if num > 0 && title != "" {
			words := strings.Fields(title)
			if len(words) > 5 {
				words = words[:5]
			}
			prs = append(prs, statusPR{number: num, title: strings.Join(words, " ")})
		}
	}
	return prs
}

func statusIssues() []statusIssue {
	return nil
}

func statusRepoName() string {
	out := strings.TrimSpace(gitOut("remote", "get-url", "origin"))
	out = strings.TrimSuffix(out, ".git")
	// SSH: git@github.com:user/repo
	if strings.Contains(out, ":") && !strings.Contains(out, "//") {
		parts := strings.SplitN(out, ":", 2)
		return parts[1]
	}
	// HTTPS: https://github.com/user/repo
	parts := strings.Split(out, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return out
}

func statusVersion() string {
	out := gitOut("describe", "--tags", "--abbrev=0")
	out = strings.TrimSpace(out)
	if out == "" {
		return "untagged"
	}
	return out
}

func statusLastRelease() string {
	out := gitOut("log", "--tags", "--simplify-by-decoration", "--pretty=format:%cr", "-1")
	out = strings.TrimSpace(out)
	if out == "" {
		return "never"
	}
	return out
}

func gitOut(args ...string) string {
	cmd := exec.Command("git", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Run()
	return strings.TrimSpace(out.String())
}

func terminalWidth() int {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil {
		var rows, cols int
		fmt.Sscanf(strings.TrimSpace(out.String()), "%d %d", &rows, &cols)
		if cols > 40 {
			return cols
		}
	}
	cmd2 := exec.Command("tput", "cols")
	var out2 bytes.Buffer
	cmd2.Stdout = &out2
	if err := cmd2.Run(); err == nil {
		var cols int
		fmt.Sscanf(strings.TrimSpace(out2.String()), "%d", &cols)
		if cols > 40 {
			return cols
		}
	}
	return 160
}
