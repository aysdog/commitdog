package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

func runUpdateBrew() {
	if err := verifyGitRepo(); err != nil {
		fatal("not a git repository.")
	}
	cfg := loadConfig()
	if cfg.Token == "" {
		fatal("no GitHub token found. run 'commitdog setup' first.")
	}
	_, repo := getRepoOwnerAndName()
	ver := getLatestGitTag()
	if ver == "" {
		fatal("no git tag found. run 'commitdog release' first.")
	}
	fmt.Println()
	fmt.Printf("  updating homebrew tap for v%s...\n\n", ver)

	sha256map, err := fetchChecksums(cfg.Token, repo, ver)
	if err != nil {
		fatal("could not fetch checksums: %v", err)
	}

	fmt.Printf("  %-38s", "updating homebrew tap...")
	if err := updateHomebrew(cfg.Token, ver, repo, sha256map); err != nil {
		fmt.Printf("\033[31m✗\033[0m\n")
		fatal("%s", err)
	}
	fmt.Printf("\033[32m✓\033[0m\n")
	fmt.Printf("\n  \033[32m✓ homebrew tap updated\033[0m\n\n")
}

func runUpdateAUR() {
	if err := verifyGitRepo(); err != nil {
		fatal("not a git repository.")
	}
	cfg := loadConfig()
	if cfg.Token == "" {
		fatal("no GitHub token found. run 'commitdog setup' first.")
	}
	_, repo := getRepoOwnerAndName()
	ver := getLatestGitTag()
	if ver == "" {
		fatal("no git tag found. run 'commitdog release' first.")
	}
	fmt.Println()
	fmt.Printf("  updating AUR PKGBUILD for v%s...\n\n", ver)

	sha256map, err := fetchChecksums(cfg.Token, repo, ver)
	if err != nil {
		fatal("could not fetch checksums: %v", err)
	}

	fmt.Printf("  %-38s", "updating AUR PKGBUILD...")
	if err := updateAUR(ver, repo, sha256map); err != nil {
		fmt.Printf("\033[31m✗\033[0m\n")
		fatal("%s", err)
	}
	fmt.Printf("\033[32m✓\033[0m\n")
	fmt.Printf("\n  \033[32m✓ AUR updated\033[0m\n\n")
}

func fetchChecksums(token, repo, ver string) (map[string]string, error) {
	owner, _ := getRepoOwnerAndName()
	body, err := githubRequest("GET",
		fmt.Sprintf("/repos/%s/%s/releases/tags/v%s", owner, repo, ver),
		token, nil,
	)
	if err != nil {
		return nil, err
	}

	var release struct {
		Assets []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, err
	}

	var checksumsURL string
	for _, a := range release.Assets {
		if a.Name == "checksums.txt" {
			checksumsURL = a.BrowserDownloadURL
			break
		}
	}
	if checksumsURL == "" {
		return nil, fmt.Errorf("checksums.txt not found in release assets")
	}

	resp, err := http.Get(checksumsURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	result := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 {
			result[parts[1]] = parts[0]
		}
	}
	return result, nil
}

func updateHomebrew(token, ver, repo string, sha256map map[string]string) error {
	arm64sum := sha256map[repo+"-darwin-arm64"]
	amd64sum := sha256map[repo+"-darwin-amd64"]
	if arm64sum == "" || amd64sum == "" {
		return fmt.Errorf("missing darwin SHA256 values")
	}

	dir, err := os.MkdirTemp("", "homebrew-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	cloneURL := fmt.Sprintf("https://%s@github.com/aysdog/homebrew-commitdog.git", token)
	cmd := exec.Command("git", "clone", "--depth=1", cloneURL, dir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("clone failed: %s", strings.TrimSpace(stderr.String()))
	}

	rbPath := dir + "/commitdog.rb"
	data, err := os.ReadFile(rbPath)
	if err != nil {
		return fmt.Errorf("could not read commitdog.rb: %v", err)
	}

	content := string(data)
	content = regexp.MustCompile(`version "[0-9]+\.[0-9]+\.[0-9]+"`).
		ReplaceAllString(content, `version "`+ver+`"`)
	content = regexp.MustCompile(`(on_arm do\s+url[^\n]+\s+sha256 ")[a-f0-9]+`).
		ReplaceAllString(content, "${1}"+arm64sum)
	content = regexp.MustCompile(`(on_intel do\s+url[^\n]+\s+sha256 ")[a-f0-9]+`).
		ReplaceAllString(content, "${1}"+amd64sum)

	if err := os.WriteFile(rbPath, []byte(content), 0644); err != nil {
		return err
	}

	cmds := [][]string{
		{"git", "-C", dir, "config", "user.email", "commitdog@aysdog.com"},
		{"git", "-C", dir, "config", "user.name", "commitdog"},
		{"git", "-C", dir, "add", "commitdog.rb"},
		{"git", "-C", dir, "commit", "-m", "chore: release v" + ver},
		{"git", "-C", dir, "push"},
	}
	for _, args := range cmds {
		c := exec.Command(args[0], args[1:]...)
		var se bytes.Buffer
		c.Stderr = &se
		if err := c.Run(); err != nil {
			return fmt.Errorf("%s failed: %s", args[len(args)-1], strings.TrimSpace(se.String()))
		}
	}
	return nil
}

func updateAUR(ver, repo string, sha256map map[string]string) error {
	amd64sum := sha256map[repo+"-linux-amd64"]
	arm64sum := sha256map[repo+"-linux-arm64"]
	if amd64sum == "" || arm64sum == "" {
		return fmt.Errorf("missing linux SHA256 values")
	}

	dir, err := os.MkdirTemp("", "aur-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	cmd := exec.Command("git", "clone", "ssh://aur@aur.archlinux.org/commitdog-bin.git", dir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("aur clone failed: %s", strings.TrimSpace(stderr.String()))
	}

	pkgPath := dir + "/PKGBUILD"
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return fmt.Errorf("could not read PKGBUILD: %v", err)
	}

	content := string(data)
	content = regexp.MustCompile(`pkgver=[0-9]+\.[0-9]+\.[0-9]+`).
		ReplaceAllString(content, "pkgver="+ver)
	content = regexp.MustCompile(`pkgrel=[0-9]+`).
		ReplaceAllString(content, "pkgrel=1")
	content = regexp.MustCompile(`sha256sums_x86_64=\('[a-f0-9]+'\)`).
		ReplaceAllString(content, "sha256sums_x86_64=('"+amd64sum+"')")
	content = regexp.MustCompile(`sha256sums_aarch64=\('[a-f0-9]+'\)`).
		ReplaceAllString(content, "sha256sums_aarch64=('"+arm64sum+"')")

	if err := os.WriteFile(pkgPath, []byte(content), 0644); err != nil {
		return err
	}

	srcinfo, err := exec.Command("bash", "-c", "cd "+dir+" && makepkg --printsrcinfo 2>/dev/null").Output()
	if err == nil && len(srcinfo) > 0 {
		os.WriteFile(dir+"/.SRCINFO", srcinfo, 0644)
	}

	cmds := [][]string{
		{"git", "-C", dir, "config", "user.email", "commitdog@aysdog.com"},
		{"git", "-C", dir, "config", "user.name", "commitdog"},
		{"git", "-C", dir, "add", "PKGBUILD", ".SRCINFO"},
		{"git", "-C", dir, "commit", "-m", "chore: release v" + ver},
		{"git", "-C", dir, "push"},
	}
	for _, args := range cmds {
		c := exec.Command(args[0], args[1:]...)
		var se bytes.Buffer
		c.Stderr = &se
		if err := c.Run(); err != nil {
			return fmt.Errorf("%s failed: %s", args[len(args)-1], strings.TrimSpace(se.String()))
		}
	}
	return nil
}
