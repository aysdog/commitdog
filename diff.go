package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type diffFile struct {
	name  string
	adds  int
	dels  int
	hunks []string
}

func getDiffFiles(base, head string) ([]diffFile, error) {
	args := []string{"diff", "--no-color", "--stat=200"}
	if base != "" && head != "" {
		args = append(args, base+"..."+head)
	} else {
		args = append(args, "HEAD")
	}
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Run()

	var files []diffFile
	for _, line := range strings.Split(out.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "...") {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		rest := strings.TrimSpace(parts[1])
		adds, dels := countPlusMinus(rest)
		files = append(files, diffFile{
			name: name,
			adds: adds,
			dels: dels,
		})
	}
	return files, nil
}

func countPlusMinus(s string) (int, int) {
	adds := strings.Count(s, "+")
	dels := strings.Count(s, "-")
	return adds, dels
}

func getFileDiff(base, head, filename string) []string {
	tryRefs := []string{base + "..." + head}
	if base != "" && head != "" {
		tryRefs = append(tryRefs, "origin/"+base+"..."+"origin/"+head)
		tryRefs = append(tryRefs, base+"...origin/"+head)
	}

	for _, ref := range tryRefs {
		args := []string{"diff", "--no-color", "-U5", ref, "--", filename}
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(), "GIT_PAGER=cat")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			continue
		}
		raw := out.String()
		if strings.TrimSpace(raw) == "" {
			continue
		}
		var lines []string
		for _, l := range strings.Split(raw, "\n") {
			if strings.HasPrefix(l, "diff --git") ||
				strings.HasPrefix(l, "index ") ||
				strings.HasPrefix(l, "--- ") ||
				strings.HasPrefix(l, "+++ ") {
				continue
			}
			lines = append(lines, l)
		}
		return lines
	}

	args := []string{"diff", "--no-color", "-U5", "HEAD", "--", filename}
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Run()
	var lines []string
	for _, l := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(l, "diff --git") ||
			strings.HasPrefix(l, "index ") ||
			strings.HasPrefix(l, "--- ") ||
			strings.HasPrefix(l, "+++ ") {
			continue
		}
		lines = append(lines, l)
	}
	return lines
}

func renderBar(adds, dels, width int) string {
	total := adds + dels
	if total == 0 {
		return strings.Repeat("░", width)
	}
	filled := (adds * width) / total
	if filled > width {
		filled = width
	}
	return "\033[32m" + strings.Repeat("█", filled) + "\033[31m" +
		strings.Repeat("█", width-filled) + "\033[0m"
}

func runDiffViewer(files []diffFile, base, head string) bool {
	if len(files) == 0 {
		return true
	}

	selected := 0
	termH := terminalHeight()
	visibleH := termH - 8
	if visibleH < 3 {
		visibleH = 3
	}
	offset := 0

	enableRawMode()
	defer disableRawMode()

	for {
		clearScreen()

		total := len(files)
		adds := 0
		dels := 0
		for _, f := range files {
			adds += f.adds
			dels += f.dels
		}

		fmt.Printf("  \033[1m%d files changed\033[0m  \033[32m+%d\033[0m  \033[31m-%d\033[0m\n\n", total, adds, dels)

		end := offset + visibleH
		if end > len(files) {
			end = len(files)
		}

		for i := offset; i < end; i++ {
			f := files[i]
			cursor := "  "
			if i == selected {
				cursor = "\033[33m›\033[0m "
			}

			name := f.name
			if len(name) > 35 {
				name = "..." + name[len(name)-32:]
			}

			bar := renderBar(f.adds, f.dels, 16)
			fmt.Printf("%s\033[36m%-36s\033[0m \033[32m+%-3d\033[0m \033[31m-%-3d\033[0m  %s\n",
				cursor, name, f.adds, f.dels, bar)
		}

		fmt.Println()
		fmt.Println("  \033[90m[j/k/↑/↓] navigate  [d] view diff  [c] create PR  [q] quit\033[0m")

		b := make([]byte, 3)
		n, _ := os.Stdin.Read(b)
		if n == 0 {
			continue
		}

		switch {
		case b[0] == 'q':
			clearScreen()
			return false
		case b[0] == 'c':
			clearScreen()
			return true
		case b[0] == 'd':
			disableRawMode()
			clearScreen()
			runInlineDiff(files[selected], base, head)
			enableRawMode()
		case isUpKey(b, n):
			if selected > 0 {
				selected--
				if selected < offset {
					offset--
				}
			}
		case isDownKey(b, n):
			if selected < len(files)-1 {
				selected++
				if selected >= offset+visibleH {
					offset++
				}
			}
		}
	}
}

func runInlineDiff(f diffFile, base, head string) {
	lines := getFileDiff(base, head, f.name)

	offset := 0
	termH := terminalHeight()
	visibleH := termH - 6

	enableRawMode()
	defer disableRawMode()

	for {
		clearScreen()

		fmt.Printf("  \033[1m\033[36m%s\033[0m  \033[32m+%d\033[0m  \033[31m-%d\033[0m\n", f.name, f.adds, f.dels)
		fmt.Println("  " + strings.Repeat("─", 60))

		end := offset + visibleH
		if end > len(lines) {
			end = len(lines)
		}

		for _, l := range lines[offset:end] {
			if strings.HasPrefix(l, "+") {
				fmt.Println("  \033[32m" + l + "\033[0m")
			} else if strings.HasPrefix(l, "-") {
				fmt.Println("  \033[31m" + l + "\033[0m")
			} else if strings.HasPrefix(l, "@@") {
				fmt.Println("  \033[36m" + l + "\033[0m")
			} else {
				fmt.Println("  \033[90m" + l + "\033[0m")
			}
		}

		fmt.Println()
		fmt.Println("  \033[90m[↑/↓] scroll  [q] back\033[0m")

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
		}
	}
}
