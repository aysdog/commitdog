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
	proj := loadProjectConfig()
	platform := proj.effectivePrimary()
	branch := getCurrentBranch()
	baseBranch := getBaseBranch()
	onBase := branch == "main" || branch == "master" || branch == baseBranch

	switch platform {
	case "gitlab":
		if cfg.GitLab.Token == "" {
			fatal("no GitLab token found. run 'commitdog setup' first.")
		}
		host := gitlabHost(cfg)
		if onBase {
			runPRListGitLab(cfg.GitLab.Token, host)
		} else {
			runPRCreateGitLab(cfg.GitLab.Token, host, branch, baseBranch)
		}
	case "gitea":
		if cfg.Gitea.Token == "" {
			fatal("no Gitea token found. run 'commitdog setup' first.")
		}
		if onBase {
			runPRListGitea(cfg.Gitea.Token, cfg.Gitea.Host)
		} else {
			runPRCreateGitea(cfg.Gitea.Token, cfg.Gitea.Host, branch, baseBranch)
		}
	case "forgejo":
		if cfg.Forgejo.Token == "" {
			fatal("no Forgejo token found. run 'commitdog setup' first.")
		}
		if onBase {
			runPRListForgejo(cfg.Forgejo.Token, cfg.Forgejo.Host)
		} else {
			runPRCreateForgejo(cfg.Forgejo.Token, cfg.Forgejo.Host, branch, baseBranch)
		}
	default:
		if cfg.GitHub.Token == "" {
			fatal("no GitHub token found. run 'commitdog setup' first.")
		}
		if onBase {
			runPRList(cfg.GitHub.Token)
		} else {
			runPRCreate(cfg.GitHub.Token, branch, baseBranch)
		}
	}
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

	titleInput, desc, ok := gatherPRContent(branch, base)
	if !ok {
		fmt.Println("  aborted.")
		return
	}
	if titleInput == "" {
		fmt.Println("  aborted: title cannot be empty.")
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
	fmt.Printf("  %s PR #%d created\n", colorGreen("✓"), pr.Number)
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

func showMergeDialog(pr prEntry) (string, bool) {
	fmt.Printf("  merge PR #%d — %s\n\n", pr.Number, pr.Title)
	fmt.Println("  1  merge commit")
	fmt.Println("  2  squash merge")
	fmt.Println("  3  rebase merge")
	fmt.Println()
	fmt.Printf("  [1/2/3] pick › ")

	methods := map[string]string{"1": "merge", "2": "squash", "3": "rebase"}
	for {
		input := strings.TrimSpace(readLine())
		if input == "q" {
			return "", false
		}
		if m, ok := methods[input]; ok {
			return m, true
		}
		fmt.Printf("  1, 2, or 3 › ")
	}
}

func renderPRList(owner, repo string, prs []prEntry, onMerge func(pr prEntry)) {
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

		fmt.Printf("  %s — %s/%s\n\n", colorBold("open PRs"), owner, repo)

		end := offset + visibleH
		if end > len(prs) {
			end = len(prs)
		}

		for i := offset; i < end; i++ {
			pr := prs[i]
			cursor := "  "
			if i == selected {
				cursor = colorYellow("›") + " "
			}
			title := pr.Title
			if len(title) > 40 {
				title = title[:37] + "..."
			}
			fmt.Printf("%s#%-4d %s → %s  %-42s %s\n",
				cursor, pr.Number,
				colorCyan(fmt.Sprintf("%-14s", pr.Head.Ref)),
				colorCyan(fmt.Sprintf("%-10s", pr.Base.Ref)),
				title,
				colorMuted("@"+pr.User.Login))
		}

		fmt.Println()
		fmt.Println(colorMuted("  [j/k/↑/↓] navigate  [d] view diff  [m] merge  [q] quit"))

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
			onMerge(prs[selected])
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

	renderPRList(owner, repo, prs, func(pr prEntry) {
		runPRMerge(token, owner, repo, pr)
	})
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

		fmt.Printf("  %s  %s  %s\n\n",
			colorBold(fmt.Sprintf("%d files changed", len(files))),
			colorGreen(fmt.Sprintf("+%d", totalAdds)),
			colorRed(fmt.Sprintf("-%d", totalDels)))

		end := offset + visibleH
		if end > len(files) {
			end = len(files)
		}

		for i := offset; i < end; i++ {
			f := files[i]
			cursor := "  "
			if i == selected {
				cursor = colorYellow("›") + " "
			}
			name := f.name
			if len(name) > 35 {
				name = "..." + name[len(name)-32:]
			}
			bar := renderBar(f.adds, f.dels, 16)
			fmt.Printf("%s%s %s %s  %s\n",
				cursor,
				colorCyan(fmt.Sprintf("%-36s", name)),
				colorGreen(fmt.Sprintf("+%-3d", f.adds)),
				colorRed(fmt.Sprintf("-%-3d", f.dels)),
				bar)
		}

		fmt.Println()
		fmt.Println(colorMuted("  [j/k/↑/↓] navigate  [d] view diff  [q] back"))

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
	method, ok := showMergeDialog(pr)
	if !ok {
		fmt.Println("  aborted.")
		return
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
	fmt.Printf("  %s merged PR #%d into %s\n\n", colorGreen("✓"), pr.Number, pr.Base.Ref)
	fmt.Printf("  delete %s branch? [Y/n] › ", pr.Head.Ref)

	if ans := readLine(); ans != "n" && ans != "no" {
		_, _ = githubRequest("DELETE",
			fmt.Sprintf("/repos/%s/%s/git/refs/heads/%s", owner, repo, pr.Head.Ref),
			token, nil,
		)
		exec.Command("git", "branch", "-d", pr.Head.Ref).Run()
		fmt.Printf("  %s deleted %s\n", colorGreen("✓"), pr.Head.Ref)
	}

	fmt.Printf("  pulling %s...\n", pr.Base.Ref)
	exec.Command("git", "checkout", pr.Base.Ref).Run()
	exec.Command("git", "pull", "--rebase").Run()
	fmt.Printf("  %s up to date\n\n", colorGreen("✓"))
}

func getRepoOwnerAndName() (string, string) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	var out strings.Builder
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", ""
	}
	return parseRemoteOwnerAndName(strings.TrimSpace(out.String()))
}

func parseRemoteOwnerAndName(rawURL string) (string, string) {
	rawURL = strings.TrimSuffix(rawURL, ".git")
	if strings.HasPrefix(rawURL, "git@") {
		parts := strings.SplitN(rawURL, ":", 2)
		if len(parts) == 2 {
			ownerRepo := strings.SplitN(parts[1], "/", 2)
			if len(ownerRepo) == 2 {
				return ownerRepo[0], ownerRepo[1]
			}
		}
		return "", ""
	}
	if idx := strings.Index(rawURL, "://"); idx >= 0 {
		rest := rawURL[idx+3:]
		if at := strings.LastIndex(rest, "@"); at >= 0 {
			rest = rest[at+1:]
		}
		parts := strings.SplitN(rest, "/", 3)
		if len(parts) == 3 {
			return parts[1], parts[2]
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

func runPRCreateGitLab(token, host, branch, base string) {
	owner, repo := getRepoOwnerAndName()
	if owner == "" || repo == "" {
		fatal("could not detect GitLab repo.")
	}

	projectID, err := getGitLabProjectID(token, host, owner, repo)
	if err != nil {
		fatal("could not get GitLab project: %v", err)
	}

	files, err := getDiffFiles(base, branch)
	if err != nil || len(files) == 0 {
		fmt.Println()
		fmt.Println("  no changes between " + branch + " and " + base)
		fmt.Println()
		return
	}

	if !runDiffViewer(files, base, branch) {
		fmt.Println("  aborted.")
		return
	}

	title, desc, ok := gatherPRContent(branch, base)
	if !ok {
		fmt.Println("  aborted.")
		return
	}
	if title == "" {
		fmt.Println("  aborted: title cannot be empty.")
		return
	}

	fmt.Println()
	fmt.Printf("  creating MR...")

	mr, err := createGitLabMR(token, host, projectID, title, desc, branch, base)
	if err != nil {
		fmt.Println()
		fatal("could not create MR: %v", err)
	}

	fmt.Println()
	fmt.Printf("  %s MR !%d created\n", colorGreen("✓"), mr.Number)
	fmt.Printf("  %s\n\n", mr.HTMLURL)
	fmt.Printf("  open in browser? [Y/n] › ")

	if ans := readLine(); ans != "n" && ans != "no" {
		openBrowser(mr.HTMLURL)
	}
	fmt.Println()
}

func runPRListGitLab(token, host string) {
	owner, repo := getRepoOwnerAndName()
	if owner == "" || repo == "" {
		fatal("could not detect GitLab repo.")
	}

	projectID, err := getGitLabProjectID(token, host, owner, repo)
	if err != nil {
		fatal("could not get GitLab project: %v", err)
	}

	fmt.Printf("  fetching MRs...")
	prs, err := listGitLabMRs(token, host, projectID)
	if err != nil {
		fmt.Println()
		fatal("could not fetch MRs: %v", err)
	}

	if len(prs) == 0 {
		fmt.Println()
		fmt.Println()
		fmt.Println("  no open MRs.")
		fmt.Println()
		return
	}

	renderPRList(owner, repo, prs, func(pr prEntry) {
		runPRMergeGitLab(token, host, projectID, pr)
	})
}

func runPRMergeGitLab(token, host, projectID string, pr prEntry) {
	method, ok := showMergeDialog(pr)
	if !ok {
		fmt.Println("  aborted.")
		return
	}

	fmt.Println()
	fmt.Printf("  merging...")

	if err := mergeGitLabMR(token, host, projectID, pr.Number, method); err != nil {
		fmt.Println()
		fatal("merge failed: %v", err)
	}

	fmt.Println()
	fmt.Printf("  %s merged MR !%d into %s\n\n", colorGreen("✓"), pr.Number, pr.Base.Ref)
	fmt.Printf("  delete %s branch? [Y/n] › ", pr.Head.Ref)

	if ans := readLine(); ans != "n" && ans != "no" {
		deleteGitLabBranch(token, host, projectID, pr.Head.Ref)
		exec.Command("git", "branch", "-d", pr.Head.Ref).Run()
		fmt.Printf("  %s deleted %s\n", colorGreen("✓"), pr.Head.Ref)
	}

	fmt.Printf("  pulling %s...\n", pr.Base.Ref)
	exec.Command("git", "checkout", pr.Base.Ref).Run()
	exec.Command("git", "pull", "--rebase").Run()
	fmt.Printf("  %s up to date\n\n", colorGreen("✓"))
}

func runPRCreateGitea(token, host, branch, base string) {
	owner, repo := getRepoOwnerAndName()
	if owner == "" || repo == "" {
		fatal("could not detect Gitea repo.")
	}

	files, err := getDiffFiles(base, branch)
	if err != nil || len(files) == 0 {
		fmt.Println()
		fmt.Println("  no changes between " + branch + " and " + base)
		fmt.Println()
		return
	}

	if !runDiffViewer(files, base, branch) {
		fmt.Println("  aborted.")
		return
	}

	title, desc, ok := gatherPRContent(branch, base)
	if !ok {
		fmt.Println("  aborted.")
		return
	}
	if title == "" {
		fmt.Println("  aborted: title cannot be empty.")
		return
	}

	fmt.Println()
	fmt.Printf("  creating PR...")

	pr, err := createGiteaPR(token, host, owner, repo, title, desc, branch, base)
	if err != nil {
		fmt.Println()
		fatal("could not create PR: %v", err)
	}

	fmt.Println()
	fmt.Printf("  %s PR #%d created\n", colorGreen("✓"), pr.Number)
	fmt.Printf("  %s\n\n", pr.HTMLURL)
	fmt.Printf("  open in browser? [Y/n] › ")

	if ans := readLine(); ans != "n" && ans != "no" {
		openBrowser(pr.HTMLURL)
	}
	fmt.Println()
}

func runPRListGitea(token, host string) {
	owner, repo := getRepoOwnerAndName()
	if owner == "" || repo == "" {
		fatal("could not detect Gitea repo.")
	}

	fmt.Printf("  fetching PRs...")
	prs, err := listGiteaPRs(token, host, owner, repo)
	if err != nil {
		fmt.Println()
		fatal("could not fetch PRs: %v", err)
	}

	if len(prs) == 0 {
		fmt.Println()
		fmt.Println()
		fmt.Println("  no open PRs.")
		fmt.Println()
		return
	}

	renderPRList(owner, repo, prs, func(pr prEntry) {
		runPRMergeGitea(token, host, owner, repo, pr)
	})
}

func runPRMergeGitea(token, host, owner, repo string, pr prEntry) {
	method, ok := showMergeDialog(pr)
	if !ok {
		fmt.Println("  aborted.")
		return
	}

	fmt.Println()
	fmt.Printf("  merging...")

	if err := mergeGiteaPR(token, host, owner, repo, pr.Number, method); err != nil {
		fmt.Println()
		fatal("merge failed: %v", err)
	}

	fmt.Println()
	fmt.Printf("  %s merged PR #%d into %s\n\n", colorGreen("✓"), pr.Number, pr.Base.Ref)
	fmt.Printf("  delete %s branch? [Y/n] › ", pr.Head.Ref)

	if ans := readLine(); ans != "n" && ans != "no" {
		deleteGiteaBranch(token, host, owner, repo, pr.Head.Ref)
		exec.Command("git", "branch", "-d", pr.Head.Ref).Run()
		fmt.Printf("  %s deleted %s\n", colorGreen("✓"), pr.Head.Ref)
	}

	fmt.Printf("  pulling %s...\n", pr.Base.Ref)
	exec.Command("git", "checkout", pr.Base.Ref).Run()
	exec.Command("git", "pull", "--rebase").Run()
	fmt.Printf("  %s up to date\n\n", colorGreen("✓"))
}

func runPRCreateForgejo(token, host, branch, base string) {
	owner, repo := getRepoOwnerAndName()
	if owner == "" || repo == "" {
		fatal("could not detect Forgejo repo.")
	}

	files, err := getDiffFiles(base, branch)
	if err != nil || len(files) == 0 {
		fmt.Println()
		fmt.Println("  no changes between " + branch + " and " + base)
		fmt.Println()
		return
	}

	if !runDiffViewer(files, base, branch) {
		fmt.Println("  aborted.")
		return
	}

	title, desc, ok := gatherPRContent(branch, base)
	if !ok {
		fmt.Println("  aborted.")
		return
	}
	if title == "" {
		fmt.Println("  aborted: title cannot be empty.")
		return
	}

	fmt.Println()
	fmt.Printf("  creating PR...")

	pr, err := createForgejoPR(token, host, owner, repo, title, desc, branch, base)
	if err != nil {
		fmt.Println()
		fatal("could not create PR: %v", err)
	}

	fmt.Println()
	fmt.Printf("  %s PR #%d created\n", colorGreen("✓"), pr.Number)
	fmt.Printf("  %s\n\n", pr.HTMLURL)
	fmt.Printf("  open in browser? [Y/n] › ")

	if ans := readLine(); ans != "n" && ans != "no" {
		openBrowser(pr.HTMLURL)
	}
	fmt.Println()
}

func runPRListForgejo(token, host string) {
	owner, repo := getRepoOwnerAndName()
	if owner == "" || repo == "" {
		fatal("could not detect Forgejo repo.")
	}

	fmt.Printf("  fetching PRs...")
	prs, err := listForgejoPRs(token, host, owner, repo)
	if err != nil {
		fmt.Println()
		fatal("could not fetch PRs: %v", err)
	}

	if len(prs) == 0 {
		fmt.Println()
		fmt.Println()
		fmt.Println("  no open PRs.")
		fmt.Println()
		return
	}

	renderPRList(owner, repo, prs, func(pr prEntry) {
		runPRMergeForgejo(token, host, owner, repo, pr)
	})
}

func runPRMergeForgejo(token, host, owner, repo string, pr prEntry) {
	method, ok := showMergeDialog(pr)
	if !ok {
		fmt.Println("  aborted.")
		return
	}

	fmt.Println()
	fmt.Printf("  merging...")

	if err := mergeForgejoPR(token, host, owner, repo, pr.Number, method); err != nil {
		fmt.Println()
		fatal("merge failed: %v", err)
	}

	fmt.Println()
	fmt.Printf("  %s merged PR #%d into %s\n\n", colorGreen("✓"), pr.Number, pr.Base.Ref)
	fmt.Printf("  delete %s branch? [Y/n] › ", pr.Head.Ref)

	if ans := readLine(); ans != "n" && ans != "no" {
		deleteForgejoBranch(token, host, owner, repo, pr.Head.Ref)
		exec.Command("git", "branch", "-d", pr.Head.Ref).Run()
		fmt.Printf("  %s deleted %s\n", colorGreen("✓"), pr.Head.Ref)
	}

	fmt.Printf("  pulling %s...\n", pr.Base.Ref)
	exec.Command("git", "checkout", pr.Base.Ref).Run()
	exec.Command("git", "pull", "--rebase").Run()
	fmt.Printf("  %s up to date\n\n", colorGreen("✓"))
}
