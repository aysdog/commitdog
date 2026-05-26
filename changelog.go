package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"unicode"
)

func buildChangelog(sinceTag string) string {
	var args []string
	if sinceTag != "" && sinceTag != "0.0.0" {
		args = []string{"log", "v" + sinceTag + "..HEAD", "--pretty=format:%s", "--no-merges"}
	} else {
		args = []string{"log", "--pretty=format:%s", "--no-merges"}
	}
	cmd := exec.Command("git", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Run()

	groups := map[string][]string{
		"feat":     {},
		"fix":      {},
		"chore":    {},
		"docs":     {},
		"refactor": {},
		"other":    {},
	}
	order := []string{"feat", "fix", "refactor", "docs", "chore", "other"}
	labels := map[string]string{
		"feat":     "Features",
		"fix":      "Bug Fixes",
		"refactor": "Refactoring",
		"docs":     "Documentation",
		"chore":    "Chores",
		"other":    "Other",
	}

	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		matched := false
		for _, key := range order[:len(order)-1] {
			if strings.HasPrefix(line, key+"(") || strings.HasPrefix(line, key+":") {
				groups[key] = append(groups[key], cleanChangelogLine(line))
				matched = true
				break
			}
		}
		if !matched {
			groups["other"] = append(groups["other"], cleanChangelogLine(line))
		}
	}

	var sb strings.Builder
	for _, key := range order {
		items := groups[key]
		if len(items) == 0 {
			continue
		}
		sb.WriteString("### " + labels[key] + "\n")
		for _, item := range items {
			sb.WriteString("- " + item + "\n")
		}
		sb.WriteString("\n")
	}

	result := strings.TrimSpace(sb.String())
	if result == "" {
		return "No changes."
	}
	return result
}

func createGitHubReleaseWithBody(token, owner, repo, ver, body string) (int64, string, error) {
	existing, err := githubRequest("GET",
		fmt.Sprintf("/repos/%s/%s/releases/tags/v%s", owner, repo, ver),
		token, nil,
	)
	if err == nil {
		var result struct {
			ID        int64  `json:"id"`
			UploadURL string `json:"upload_url"`
		}
		if json.Unmarshal(existing, &result) == nil && result.ID != 0 {
			githubRequest("PATCH",
				fmt.Sprintf("/repos/%s/%s/releases/%d", owner, repo, result.ID),
				token, map[string]interface{}{"body": body},
			)
			return result.ID, strings.Split(result.UploadURL, "{")[0], nil
		}
	}

	payload := map[string]interface{}{
		"tag_name":               "v" + ver,
		"name":                   "v" + ver,
		"body":                   body,
		"draft":                  false,
		"generate_release_notes": false,
	}
	data, err := githubRequest("POST",
		fmt.Sprintf("/repos/%s/%s/releases", owner, repo),
		token, payload,
	)
	if err != nil {
		return 0, "", err
	}
	var result struct {
		ID        int64  `json:"id"`
		UploadURL string `json:"upload_url"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return 0, "", err
	}
	return result.ID, strings.Split(result.UploadURL, "{")[0], nil
}

func fileSHA256(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func cleanChangelogLine(s string) string {
	re := regexp.MustCompile(`^[a-z]+(?:\([^)]+\))?:\s*`)
	s = re.ReplaceAllString(s, "")
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

type branchChanges struct {
	features  []string
	fixes     []string
	security  []string
	docs      []string
	removed   []string
	refactors []string
	chores    []string
	branch    string
	base      string
	total     int
}

func collectBranchChanges(head, base string) (branchChanges, error) {
	branch := head
	if head == "HEAD" {
		branch = getCurrentBranch()
	}
	cmd := exec.Command("git", "log", base+".."+head, "--pretty=format:%s", "--no-merges")
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return branchChanges{}, err
	}
	raw := strings.TrimSpace(out.String())
	bc := branchChanges{branch: branch, base: base}
	if raw == "" {
		return bc, nil
	}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		bc.total++
		cleaned := cleanChangelogLine(line)
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "feat") || strings.HasPrefix(lower, "feature"):
			bc.features = append(bc.features, cleaned)
		case strings.HasPrefix(lower, "fix") || strings.HasPrefix(lower, "bug"):
			bc.fixes = append(bc.fixes, cleaned)
		case strings.HasPrefix(lower, "sec"):
			bc.security = append(bc.security, cleaned)
		case strings.HasPrefix(lower, "doc"):
			bc.docs = append(bc.docs, cleaned)
		case strings.HasPrefix(lower, "remove") || strings.HasPrefix(lower, "revert") || strings.HasPrefix(lower, "drop") || strings.HasPrefix(lower, "delete"):
			bc.removed = append(bc.removed, cleaned)
		case strings.HasPrefix(lower, "refactor"):
			bc.refactors = append(bc.refactors, cleaned)
		default:
			bc.chores = append(bc.chores, cleaned)
		}
	}
	return bc, nil
}

func generateIntroLine(bc branchChanges) string {
	platformMap := map[string]string{
		"gitlab": "GitLab", "gitea": "Gitea", "forgejo": "Forgejo", "github": "GitHub",
	}
	var platforms []string
	seen := map[string]bool{}
	for _, f := range bc.features {
		lower := strings.ToLower(f)
		for key, display := range platformMap {
			if strings.Contains(lower, key) && !seen[key] {
				platforms = append(platforms, display)
				seen[key] = true
			}
		}
	}
	if len(platforms) >= 2 {
		last := platforms[len(platforms)-1]
		rest := strings.Join(platforms[:len(platforms)-1], ", ")
		return fmt.Sprintf("commitdog now supports %s and %s — same workflow, same commands, different platform.", rest, last)
	}
	if len(platforms) == 1 {
		return fmt.Sprintf("commitdog now supports %s — same workflow, same commands.", platforms[0])
	}
	if len(bc.features) > 0 {
		return strings.ToLower(bc.features[0]) + "."
	}
	if bc.total > 0 {
		return fmt.Sprintf("%d changes in this release.", bc.total)
	}
	return ""
}

func generatePRTitle(bc branchChanges) string {
	all := append(bc.features, bc.fixes...)
	if len(all) == 0 {
		all = bc.chores
	}
	if len(all) == 0 {
		return bc.branch
	}
	title := all[0]
	if len(all) > 1 {
		title += fmt.Sprintf(" (+%d more)", len(all)-1)
	}
	if len(title) > 72 {
		title = title[:69] + "..."
	}
	return title
}

func generatePRDescription(bc branchChanges) string {
	var sb strings.Builder
	sb.WriteString("## what's changed\n\n")
	sections := []struct {
		title string
		items []string
	}{
		{"features", bc.features},
		{"bug fixes", bc.fixes},
		{"security", bc.security},
		{"removed", bc.removed},
		{"refactoring", bc.refactors},
		{"documentation", bc.docs},
		{"chores", bc.chores},
	}
	for _, s := range sections {
		if len(s.items) == 0 {
			continue
		}
		sb.WriteString("### " + s.title + "\n")
		for _, item := range s.items {
			sb.WriteString("- " + item + "\n")
		}
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

func generateMergeCommitMsg(bc branchChanges, into string) string {
	var sb strings.Builder
	var titleParts []string
	if len(bc.features) > 0 {
		titleParts = append(titleParts, strings.ToLower(bc.features[0]))
	}
	if len(bc.fixes) > 0 && len(bc.features) > 0 {
		titleParts = append(titleParts, fmt.Sprintf("fix %d issues", len(bc.fixes)))
	} else if len(bc.fixes) > 0 {
		titleParts = append(titleParts, strings.ToLower(bc.fixes[0]))
	}
	subject := fmt.Sprintf("merge %s → %s", bc.branch, into)
	if len(titleParts) > 0 {
		subject += ": " + strings.Join(titleParts, ", ")
	}
	subject += fmt.Sprintf(" (%d commits)", bc.total)
	sb.WriteString(subject + "\n\n")
	sections := []struct {
		title string
		items []string
	}{
		{"features", bc.features},
		{"bug fixes", bc.fixes},
		{"security", bc.security},
		{"removed", bc.removed},
		{"refactoring", bc.refactors},
		{"documentation", bc.docs},
	}
	for _, s := range sections {
		if len(s.items) == 0 {
			continue
		}
		sb.WriteString(s.title + "\n")
		for _, item := range s.items {
			sb.WriteString("  · " + item + "\n")
		}
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

func generateReleaseNotes(version string, bc branchChanges) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## what's new in %s\n\n", version))
	intro := generateIntroLine(bc)
	if intro != "" {
		sb.WriteString(intro + "\n\n")
	}
	sections := []struct {
		title string
		items []string
	}{
		{"features", bc.features},
		{"bug fixes", bc.fixes},
		{"security", bc.security},
		{"removed", bc.removed},
		{"refactoring", bc.refactors},
		{"documentation", bc.docs},
	}
	for _, s := range sections {
		if len(s.items) == 0 {
			continue
		}
		sb.WriteString("### " + s.title + "\n")
		for _, item := range s.items {
			sb.WriteString("- " + item + "\n")
		}
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

func openInEditor(content string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		for _, e := range []string{"nano", "vim", "vi"} {
			if _, err := exec.LookPath(e); err == nil {
				editor = e
				break
			}
		}
	}
	if editor == "" {
		return content, fmt.Errorf("no editor found — set $EDITOR")
	}
	tmp, err := os.CreateTemp("", "commitdog-*.md")
	if err != nil {
		return content, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(content); err != nil {
		return content, err
	}
	tmp.Close()
	cmd := exec.Command(editor, tmp.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return content, err
	}
	edited, err := os.ReadFile(tmp.Name())
	if err != nil {
		return content, err
	}
	return string(edited), nil
}

func gatherPRContent(branch, base string) (title, desc string, ok bool) {
	bc, err := collectBranchChanges("HEAD", base)
	if err != nil || bc.total == 0 {
		return getLastCommitSubject(), "", true
	}
	title = generatePRTitle(bc)
	desc = generatePRDescription(bc)
	preview := strings.Split(desc, "\n")
	limit := 12
	if len(preview) < limit {
		limit = len(preview)
	}
	fmt.Println()
	fmt.Printf("  %s commits on %s\n\n", colorCyan(fmt.Sprintf("%d", bc.total)), colorCyan(branch))
	fmt.Printf("  title: %s\n\n", colorBold(title))
	fmt.Println(colorMuted("  ─────────────────────────────────"))
	for _, line := range preview[:limit] {
		fmt.Printf("  %s\n", colorMuted(line))
	}
	if len(strings.Split(desc, "\n")) > limit {
		fmt.Printf("  %s\n", colorMuted("  ..."))
	}
	fmt.Println(colorMuted("  ─────────────────────────────────"))
	fmt.Println()
	fmt.Printf("  [enter] use as-is  [e] edit  [t] edit title  [q] cancel › ")
	for {
		input := strings.ToLower(strings.TrimSpace(readLine()))
		switch input {
		case "", "y":
			return title, desc, true
		case "e":
			edited, err := openInEditor(title + "\n\n" + desc)
			if err != nil {
				fmt.Printf("  %s %v\n", colorRed("✗"), err)
				return title, desc, true
			}
			parts := strings.SplitN(strings.TrimSpace(edited), "\n", 2)
			title = strings.TrimSpace(parts[0])
			if len(parts) > 1 {
				desc = strings.TrimSpace(parts[1])
			}
			return title, desc, true
		case "t":
			fmt.Printf("  title › ")
			t := strings.TrimSpace(readLine())
			if t != "" {
				title = t
			}
			fmt.Printf("  [enter] use as-is  [e] edit description  [q] cancel › ")
		case "q":
			return "", "", false
		default:
			fmt.Printf("  [enter] use as-is  [e] edit  [t] edit title  [q] cancel › ")
		}
	}
}
