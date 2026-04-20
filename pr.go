package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type prEntry struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	HTMLURL string `json:"html_url"`
	Head    struct {
		Ref string `json:"ref"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
	User struct {
		Login string `json:"login"`
	} `json:"user"`
	UpdatedAt string `json:"updated_at"`
}

type createPRRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Head  string `json:"head"`
	Base  string `json:"base"`
}

type mergePRRequest struct {
	MergeMethod string `json:"merge_method"`
}

func runPR() {
	if err := verifyGitRepo(); err != nil {
		fatal("not a git repository.")
	}

	cfg := loadConfig()
	if cfg.Token == "" {
		fatal("no GitHub token found. run 'commitdog setup' first.")
	}

	branch := getCurrentBranch()
	baseBranch := getBaseBranch()

	if branch == "main" || branch == "master" || branch == baseBranch {
		runPRList(cfg.Token)
		return
	}

	runPRCreate(cfg.Token, branch, baseBranch)
}

func runPRCreate(token, branch, base string) {
	owner, repo := getRepoOwnerAndName()
	if owner == "" || repo == "" {
		fatal("could not detect GitHub repo. make sure remote is set to git@github.com:user/repo.git")
	}

	files, err := getDiffFiles(base, branch)
	if err != nil || len(files) == 0 {
		fmt.Println()
		fmt.Println("  no changes detected between " + branch + " and " + base)
		fmt.Println()
		return
	}

	proceed := runDiffViewer(files, base, branch)
	if !proceed {
		fmt.Println("  aborted.")
		return
	}

	defaultTitle := getLastCommitSubject()

	fmt.Println()
	fmt.Printf("  creating PR: \033[36m%s\033[0m → \033[36m%s\033[0m\n\n", branch, base)
	fmt.Printf("  title › ")

	titleInput := readLine()
	if titleInput == "" {
		titleInput = defaultTitle
	}
	if titleInput == "" {
		fmt.Println("  aborted: title cannot be empty.")
		return
	}

	fmt.Printf("  description (optional) › ")
	desc := readLine()

	fmt.Println()
	fmt.Printf("  [enter] confirm  [q] cancel › ")
	confirm := readLine()
	if confirm == "q" {
		fmt.Println("  aborted.")
		return
	}

	fmt.Println()
	fmt.Printf("  creating PR...")

	payload := createPRRequest{
		Title: titleInput,
		Body:  desc,
		Head:  branch,
		Base:  base,
	}

	body, err := githubRequest("POST", "/repos/"+owner+"/"+repo+"/pulls", token, payload)
	if err != nil {
		fmt.Println()
		fatal("could not create PR: %v", err)
	}

	var pr prEntry
	if err := json.Unmarshal(body, &pr); err != nil {
		fmt.Println()
		fatal("unexpected response from GitHub: %v", err)
	}

	fmt.Println()
	fmt.Printf("  \033[32m✓\033[0m PR #%d created\n", pr.Number)
	fmt.Printf("  %s\n\n", pr.HTMLURL)
	fmt.Printf("  open in browser? [Y/n] › ")

	if ans := readLine(); ans != "n" && ans != "no" {
		openBrowser(pr.HTMLURL)
	}
	fmt.Println()
}

func isUpKey(b []byte, n int) bool {
	return b[0] == 'k' || (n == 3 && b[0] == 27 && b[1] == 91 && b[2] == 65)
}

func isDownKey(b []byte, n int) bool {
	return b[0] == 'j' || (n == 3 && b[0] == 27 && b[1] == 91 && b[2] == 66)
}

func runPRList(token string) {
	owner, repo := getRepoOwnerAndName()
	if owner == "" || repo == "" {
		fatal("could not detect GitHub repo.")
	}

	fmt.Printf("  fetching PRs...")
	body, err := githubRequest("GET", "/repos/"+owner+"/"+repo+"/pulls?state=open&per_page=20", token, nil)
	if err != nil {
		fmt.Println()
		fatal("could not fetch PRs: %v", err)
	}

	var prs []prEntry
	if err := json.Unmarshal(body, &prs); err != nil {
		fmt.Println()
		fatal("unexpected response from GitHub: %v", err)
	}

	if len(prs) == 0 {
		fmt.Println()
		fmt.Println()
		fmt.Println("  no open PRs.")
		fmt.Println()
		return
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

		fmt.Printf("  \033[1mopen PRs\033[0m — %s/%s\n\n", owner, repo)

		end := offset + visibleH
		if end > len(prs) {
			end = len(prs)
		}

		for i := offset; i < end; i++ {
			pr := prs[i]
			cursor := "  "
			if i == selected {
				cursor = "\033[33m›\033[0m "
			}
			title := pr.Title
			if len(title) > 40 {
				title = title[:37] + "..."
			}
			fmt.Printf("%s#%-4d \033[36m%-14s\033[0m → \033[36m%-10s\033[0m  %-42s \033[90m@%s\033[0m\n",
				cursor, pr.Number, pr.Head.Ref, pr.Base.Ref, title, pr.User.Login)
		}

		fmt.Println()
		fmt.Println("  \033[90m[j/k/↑/↓] navigate  [d] view diff  [m] merge  [q] quit\033[0m")

		b := make([]byte, 3)
		n, _ := os.Stdin.Read(b)
		if n == 0 {
			continue
		}

		switch {
		case b[0] == 'q':
			clearScreen()
			return
		case b[0] == 'm':
			disableRawMode()
			clearScreen()
			runPRMerge(token, owner, repo, prs[selected])
			return
		case b[0] == 'd':
			disableRawMode()
			clearScreen()
			files, _ := getDiffFiles(prs[selected].Base.Ref, prs[selected].Head.Ref)
			if len(files) > 0 {
				runDiffViewerReview(files, prs[selected].Base.Ref, prs[selected].Head.Ref)
			}
			enableRawMode()
		case isUpKey(b, n):
			if selected > 0 {
				selected--
				if selected < offset {
					offset--
				}
			}
		case isDownKey(b, n):
			if selected < len(prs)-1 {
				selected++
				if selected >= offset+visibleH {
					offset++
				}
			}
		}
	}
}

func runDiffViewerReview(files []diffFile, base, head string) {
	selected := 0
	termH := terminalHeight()
	visibleH := termH - 10
	if visibleH < 3 {
		visibleH = 3
	}
	offset := 0

	enableRawMode()
	defer disableRawMode()

	for {
		clearScreen()

		totalAdds := 0
		totalDels := 0
		for _, f := range files {
			totalAdds += f.adds
			totalDels += f.dels
		}

		fmt.Printf("  \033[1m%d files changed\033[0m  \033[32m+%d\033[0m  \033[31m-%d\033[0m\n\n", len(files), totalAdds, totalDels)

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
		fmt.Println("  \033[90m[j/k/↑/↓] navigate  [d] view diff  [q] back\033[0m")

		b := make([]byte, 3)
		n, _ := os.Stdin.Read(b)
		if n == 0 {
			continue
		}

		switch {
		case b[0] == 'q':
			clearScreen()
			return
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

func runPRMerge(token, owner, repo string, pr prEntry) {
	fmt.Printf("  merge PR #%d — %s\n\n", pr.Number, pr.Title)
	fmt.Println("  1  merge commit")
	fmt.Println("  2  squash merge")
	fmt.Println("  3  rebase merge")
	fmt.Println()
	fmt.Printf("  [1/2/3] pick › ")

	methods := map[string]string{
		"1": "merge",
		"2": "squash",
		"3": "rebase",
	}

	var method string
	for {
		input := strings.TrimSpace(readLine())
		if input == "q" {
			fmt.Println("  aborted.")
			return
		}
		if m, ok := methods[input]; ok {
			method = m
			break
		}
		fmt.Printf("  1, 2, or 3 › ")
	}

	fmt.Println()
	fmt.Printf("  merging...")

	_, err := githubRequest("PUT",
		fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", owner, repo, pr.Number),
		token,
		mergePRRequest{MergeMethod: method},
	)
	if err != nil {
		fmt.Println()
		fatal("merge failed: %v", err)
	}

	fmt.Println()
	fmt.Printf("  \033[32m✓\033[0m merged PR #%d into %s\n\n", pr.Number, pr.Base.Ref)
	fmt.Printf("  delete %s branch? [Y/n] › ", pr.Head.Ref)

	if ans := readLine(); ans != "n" && ans != "no" {
		_, _ = githubRequest("DELETE",
			fmt.Sprintf("/repos/%s/%s/git/refs/heads/%s", owner, repo, pr.Head.Ref),
			token, nil,
		)
		exec.Command("git", "branch", "-d", pr.Head.Ref).Run()
		fmt.Printf("  \033[32m✓\033[0m deleted %s\n", pr.Head.Ref)
	}

	fmt.Printf("  pulling %s...\n", pr.Base.Ref)
	exec.Command("git", "checkout", pr.Base.Ref).Run()
	exec.Command("git", "pull", "--rebase").Run()
	fmt.Printf("  \033[32m✓\033[0m up to date\n\n")
}

func getRepoOwnerAndName() (string, string) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	var out strings.Builder
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", ""
	}
	url := strings.TrimSpace(out.String())
	if strings.HasPrefix(url, "git@github.com:") {
		path := strings.TrimPrefix(url, "git@github.com:")
		path = strings.TrimSuffix(path, ".git")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
	}
	if strings.HasPrefix(url, "https://github.com/") {
		path := strings.TrimPrefix(url, "https://github.com/")
		path = strings.TrimSuffix(path, ".git")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
	}
	return "", ""
}

func getBaseBranch() string {
	cmd := exec.Command("git", "rev-parse", "--verify", "main")
	if cmd.Run() == nil {
		return "main"
	}
	return "master"
}

func getLastCommitSubject() string {
	cmd := exec.Command("git", "log", "-1", "--pretty=format:%s")
	cmd.Env = append([]string{"GIT_PAGER=cat"}, cmd.Env...)
	var out strings.Builder
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}

func openBrowser(url string) {
	cmds := [][]string{
		{"xdg-open", url},
		{"open", url},
		{"cmd", "/c", "start", url},
	}
	for _, c := range cmds {
		if err := exec.Command(c[0], c[1:]...).Start(); err == nil {
			return
		}
	}
}
