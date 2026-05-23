package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const releaseAPI = "https://api.github.com/repos/aysdog/commitdog/releases/latest"

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type githubRelease struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

func runUpdate() {
	fmt.Println()

	fmt.Printf("  checking latest version...")
	release, err := fetchLatestRelease()
	if err != nil {
		fmt.Println()
		fatal("could not reach GitHub: %v", err)
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	current := version

	fmt.Printf("\n  current: v%s\n", current)
	fmt.Printf("  latest:  v%s\n", latest)
	fmt.Println()

	if latest == current {
		fmt.Println("  already up to date.")
		fmt.Println()
		return
	}

	assetName := buildAssetName()
	var downloadURL string
	for _, a := range release.Assets {
		if a.Name == assetName {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		fatal("no binary found for %s in release %s", assetName, release.TagName)
	}

	fmt.Printf("  downloading %s...", assetName)
	tmpPath, err := downloadBinary(downloadURL)
	if err != nil {
		fmt.Println()
		fatal("download failed: %v", err)
	}
	fmt.Println(" done")

	selfPath, err := os.Executable()
	if err != nil {
		fatal("could not find current binary: %v", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, 0755); err != nil {
			fatal("could not set permissions: %v", err)
		}
	}

	fmt.Printf("  installing to %s...", selfPath)
	if err := replaceBinary(tmpPath, selfPath); err != nil {
		fmt.Println()
		if runtime.GOOS == "windows" {
			fatal("install failed: %v\n\n  run PowerShell as Administrator and try again", err)
		}
		fatal("install failed: %v\n\n  try: sudo commitdog --update", err)
	}
	fmt.Println(" done")

	fmt.Printf("\n  ✓ updated to v%s\n", latest)
	fmt.Println("  config and saved settings are untouched.")
	fmt.Println()
}

func fetchLatestRelease() (githubRelease, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", releaseAPI, nil)
	if err != nil {
		return githubRelease{}, err
	}
	req.Header.Set("User-Agent", "commitdog/"+version)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return githubRelease{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return githubRelease{}, err
	}

	var r githubRelease
	if err := json.Unmarshal(body, &r); err != nil {
		return githubRelease{}, err
	}
	if r.TagName == "" {
		return githubRelease{}, fmt.Errorf("no release found")
	}
	return r, nil
}

func buildAssetName() string {
	arch := runtime.GOARCH
	switch runtime.GOOS {
	case "windows":
		return fmt.Sprintf("commitdog-windows-%s.exe", arch)
	case "darwin":
		return fmt.Sprintf("commitdog-darwin-%s", arch)
	default:
		return fmt.Sprintf("commitdog-linux-%s", arch)
	}
}

func downloadBinary(url string) (string, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "commitdog-update-*")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}

	return tmp.Name(), nil
}

func replaceBinary(src, dst string) error {
	if runtime.GOOS == "windows" {
		return replaceBinaryWindows(src, dst)
	}
	return replaceBinaryUnix(src, dst)
}

func replaceBinaryUnix(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	cmd := exec.Command("sudo", "mv", src, dst)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		os.Remove(src)
		return err
	}
	return nil
}

func replaceBinaryWindows(src, dst string) error {
	old := dst + ".old"
	os.Remove(old)

	if err := os.Rename(dst, old); err != nil {
		os.Remove(src)
		return fmt.Errorf("could not move old binary: %v", err)
	}

	if err := os.Rename(src, dst); err != nil {
		os.Rename(old, dst)
		os.Remove(src)
		return fmt.Errorf("could not place new binary: %v", err)
	}

	cmd := exec.Command("cmd", "/C", "ping", "-n", "2", "127.0.0.1", ">nul", "&", "del", "/F", "/Q", old)
	cmd.SysProcAttr = nil
	_ = cmd.Start()

	return nil
}
