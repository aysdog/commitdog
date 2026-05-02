package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type releaseProject struct {
	lang    string
	file    string
	pattern *regexp.Regexp
	replace func(content, newVer string) string
}

var releaseProjects = []releaseProject{
	{
		lang:    "Go",
		file:    "main.go",
		pattern: regexp.MustCompile(`const version\s*=\s*"([0-9]+\.[0-9]+\.[0-9]+)"`),
		replace: func(content, newVer string) string {
			re := regexp.MustCompile(`(const version\s*=\s*)"[0-9]+\.[0-9]+\.[0-9]+"`)
			return re.ReplaceAllString(content, `${1}"`+newVer+`"`)
		},
	},
	{
		lang:    "Node.js",
		file:    "package.json",
		pattern: regexp.MustCompile(`"version"\s*:\s*"([0-9]+\.[0-9]+\.[0-9]+)"`),
		replace: func(content, newVer string) string {
			re := regexp.MustCompile(`("version"\s*:\s*)"[0-9]+\.[0-9]+\.[0-9]+"`)
			return re.ReplaceAllString(content, `${1}"`+newVer+`"`)
		},
	},
	{
		lang:    "Rust",
		file:    "Cargo.toml",
		pattern: regexp.MustCompile(`(?m)^version\s*=\s*"([0-9]+\.[0-9]+\.[0-9]+)"`),
		replace: func(content, newVer string) string {
			re := regexp.MustCompile(`(?m)^(version\s*=\s*)"[0-9]+\.[0-9]+\.[0-9]+"`)
			return re.ReplaceAllString(content, `${1}"`+newVer+`"`)
		},
	},
	{
		lang:    "Python",
		file:    "pyproject.toml",
		pattern: regexp.MustCompile(`version\s*=\s*"([0-9]+\.[0-9]+\.[0-9]+)"`),
		replace: func(content, newVer string) string {
			re := regexp.MustCompile(`(version\s*=\s*)"[0-9]+\.[0-9]+\.[0-9]+"`)
			return re.ReplaceAllString(content, `${1}"`+newVer+`"`)
		},
	},
	{
		lang:    "Python",
		file:    "setup.py",
		pattern: regexp.MustCompile(`version\s*=\s*['"]([0-9]+\.[0-9]+\.[0-9]+)['""]`),
		replace: func(content, newVer string) string {
			re := regexp.MustCompile(`(version\s*=\s*)['"][0-9]+\.[0-9]+\.[0-9]+['"]`)
			return re.ReplaceAllString(content, `${1}"`+newVer+`"`)
		},
	},
	{
		lang:    "Java",
		file:    "pom.xml",
		pattern: regexp.MustCompile(`<version>([0-9]+\.[0-9]+\.[0-9]+)</version>`),
		replace: func(content, newVer string) string {
			re := regexp.MustCompile(`<version>[0-9]+\.[0-9]+\.[0-9]+</version>`)
			return re.ReplaceAllString(content, "<version>"+newVer+"</version>")
		},
	},
	{
		lang:    "any",
		file:    "VERSION",
		pattern: regexp.MustCompile(`^([0-9]+\.[0-9]+\.[0-9]+)`),
		replace: func(content, newVer string) string {
			re := regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+`)
			return re.ReplaceAllString(content, newVer)
		},
	},
}

func runVersionInit() (*releaseProject, string) {
	fmt.Println()
	fmt.Println("  could not find a version file.")
	fmt.Println()

	mainPy := detectMainPy()

	fmt.Println("  1  create pyproject.toml  (recommended for Python)")
	if mainPy != "" {
		fmt.Printf("  2  add __version__ to %s\n", mainPy)
	} else {
		fmt.Println("  2  add __version__ to main python file")
	}
	fmt.Println("  3  create VERSION file     (works for any language)")
	fmt.Println("  4  enter version manually  (one time, no file created)")
	fmt.Println()
	fmt.Printf("  [1/2/3/4/q] pick › ")

	for {
		input := strings.TrimSpace(readLine())
		switch input {
		case "1":
			if err := os.WriteFile("pyproject.toml", []byte("[project]\nname = \"app\"\nversion = \"0.1.0\"\n"), 0644); err != nil {
				fatal("could not create pyproject.toml: %v", err)
			}
			fmt.Println("  ✓ created pyproject.toml with version 0.1.0")
			p := &releaseProjects[3]
			return p, "0.1.0"
		case "2":
			target := mainPy
			if target == "" {
				fmt.Printf("  python file name › ")
				target = strings.TrimSpace(readLine())
			}
			data, _ := os.ReadFile(target)
			updated := "__version__ = \"0.1.0\"\n" + string(data)
			if err := os.WriteFile(target, []byte(updated), 0644); err != nil {
				fatal("could not write to %s: %v", target, err)
			}
			fmt.Printf("  ✓ added __version__ to %s\n", target)
			p := &releaseProject{
				lang:    "Python",
				file:    target,
				pattern: regexp.MustCompile(`__version__\s*=\s*"([0-9]+\.[0-9]+\.[0-9]+)"`),
				replace: func(content, newVer string) string {
					re := regexp.MustCompile(`(__version__\s*=\s*)"[0-9]+\.[0-9]+\.[0-9]+"`)
					return re.ReplaceAllString(content, `${1}"`+newVer+`"`)
				},
			}
			return p, "0.1.0"
		case "3":
			if err := os.WriteFile("VERSION", []byte("0.1.0\n"), 0644); err != nil {
				fatal("could not create VERSION: %v", err)
			}
			fmt.Println("  ✓ created VERSION file with 0.1.0")
			p := &releaseProjects[len(releaseProjects)-1]
			return p, "0.1.0"
		case "4":
			fmt.Printf("  version › ")
			ver := strings.TrimSpace(readLine())
			ver = strings.TrimPrefix(ver, "v")
			if !isValidSemver(ver) {
				fmt.Printf("  invalid semver (e.g. 1.0.0) › ")
				continue
			}
			p := &releaseProject{
				lang:    "manual",
				file:    "",
				replace: func(content, newVer string) string { return content },
			}
			return p, ver
		case "q", "":
			return nil, ""
		default:
			fmt.Printf("  1, 2, 3, 4, or q › ")
		}
	}
}

func detectMainPy() string {
	candidates := []string{"main.py", "app.py", "run.py", "server.py", "__init__.py"}
	for _, f := range candidates {
		if _, err := os.Stat(f); err == nil {
			return f
		}
	}
	return ""
}

type undoStep struct {
	label string
	fn    func() error
}

func runRelease() {
	if err := verifyGitRepo(); err != nil {
		fatal("not a git repository.")
	}

	if len(os.Args) > 2 {
		switch os.Args[2] {
		case "--changelog-only":
			cl := buildChangelog(getLatestGitTag())
			fmt.Println()
			fmt.Println(cl)
			return
		case "config":
			runReleaseConfig()
			return
		}
	}

	cfg := loadConfig()
	if cfg.Token == "" {
		fatal("no GitHub token found. run 'commitdog setup' first.")
	}

	owner, repo := getRepoOwnerAndName()
	if owner == "" || repo == "" {
		fatal("could not detect GitHub repo.")
	}

	proj, fileVer := detectProject()
	if proj == nil {
		proj, fileVer = runVersionInit()
		if proj == nil {
			fmt.Println("  aborted.")
			return
		}
	}

	gitTag := getLatestGitTag()
	if gitTag != "" && fileVer != gitTag {
		fmt.Printf("\n  %s version drift: %s says v%s but latest git tag is v%s\n", colorRed("!"), proj.file, fileVer, gitTag)
		fmt.Printf("  release anyway? [y/N] › ")
		if in := readLine(); in != "y" && in != "yes" {
			fmt.Println("  aborted. fix the version mismatch first.")
			return
		}
	}

	currentVer := gitTag
	if currentVer == "" {
		currentVer = fileVer
	}

	fmt.Println()
	fmt.Printf("  detected: \033[36m%s\033[0m  ·  current version: \033[1mv%s\033[0m\n\n", proj.lang, currentVer)

	major, minor, patch := splitVer(currentVer)
	fmt.Printf("  1  patch  →  v%d.%d.%d\n", major, minor, patch+1)
	fmt.Printf("  2  minor  →  v%d.%d.%d\n", major, minor+1, 0)
	fmt.Printf("  3  major  →  v%d.%d.%d\n", major+1, 0, 0)
	fmt.Printf("  4  custom\n\n")
	fmt.Printf("  [1/2/3/4/q] pick › ")

	var nextVer string
	for {
		input := strings.TrimSpace(readLine())
		switch input {
		case "1":
			nextVer = fmt.Sprintf("%d.%d.%d", major, minor, patch+1)
		case "2":
			nextVer = fmt.Sprintf("%d.%d.%d", major, minor+1, 0)
		case "3":
			nextVer = fmt.Sprintf("%d.%d.%d", major+1, 0, 0)
		case "4":
			fmt.Printf("  version › ")
			custom := strings.TrimPrefix(strings.TrimSpace(readLine()), "v")
			if !isValidSemver(custom) {
				fmt.Printf("  invalid semver (e.g. 1.2.3) › ")
				continue
			}
			nextVer = custom
		case "q":
			fmt.Println("  aborted.")
			return
		default:
			fmt.Printf("  1, 2, 3, or 4 › ")
			continue
		}
		break
	}

	changelog := buildChangelog(currentVer)
	fmt.Println()
	fmt.Println("  changelog preview:")
	fmt.Println()
	for _, line := range strings.Split(changelog, "\n") {
		fmt.Println("  " + line)
	}
	fmt.Println()
	fmt.Printf("  release v%s → v%s? [y/n] › ", currentVer, nextVer)
	if confirm := readLine(); confirm != "y" && confirm != "yes" {
		fmt.Println("  aborted.")
		return
	}
	fmt.Println()

	preReleaseHash, _ := exec.Command("git", "rev-parse", "HEAD").Output()
	preRelease := strings.TrimSpace(string(preReleaseHash))

	var undos []undoStep

	printStepOrRollback("bumping version in "+proj.file+"...", &undos, func() error {
		return bumpVersion(proj, nextVer)
	}, func() error {
		return bumpVersion(proj, currentVer)
	})

	isGo := proj.lang == "Go"
	var binaries []string

	if isGo {
		targets := ensureReleaseConfig()
		for _, t := range targets {
			name := fmt.Sprintf("%s-%s-%s%s", repo, t.goos, t.goarch, t.suffix)
			label := fmt.Sprintf("building %s/%s...", t.goos, t.goarch)
			goos, goarch, n := t.goos, t.goarch, name
			printStepOrRollback(label, &undos, func() error {
				cmd := exec.Command("go", "build", "-ldflags=-s -w", "-o", n, ".")
				cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch)
				var stderr bytes.Buffer
				cmd.Stderr = &stderr
				if err := cmd.Run(); err != nil {
					return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
				}
				return nil
			}, func() error {
				os.Remove(n)
				return nil
			})
			binaries = append(binaries, name)
		}
	}

	printStepOrRollback("committing...", &undos, func() error {
		exec.Command("git", "add", ".").Run()
		cmd := exec.Command("git", "commit", "-m", "chore: release v"+nextVer)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
		}
		return nil
	}, func() error {
		exec.Command("git", "reset", "--hard", preRelease).Run()
		return nil
	})

	printStepOrRollback("tagging v"+nextVer+"...", &undos, func() error {
		cmd := exec.Command("git", "tag", "v"+nextVer)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
		}
		return nil
	}, func() error {
		exec.Command("git", "tag", "-d", "v"+nextVer).Run()
		return nil
	})

	printStepOrRollback("pushing...", &undos, func() error {
		cmd := exec.Command("git", "push", "origin", "main", "--tags")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			msg := strings.TrimSpace(stderr.String())
			r := detectAndRecover(msg)
			if r != nil {
				fmt.Println()
				if offerRecovery(r) {
					return nil
				}
			}
			return fmt.Errorf("%s", msg)
		}
		return nil
	}, func() error {
		exec.Command("git", "push", "origin", ":refs/tags/v"+nextVer).Run()
		if preRelease != "" {
			exec.Command("git", "push", "origin", preRelease+":main", "--force").Run()
		}
		return nil
	})

	var releaseID int64
	var uploadURL string
	printStepOrRollback("creating GitHub release...", &undos, func() error {
		id, url, err := createGitHubReleaseWithBody(cfg.Token, owner, repo, nextVer, changelog)
		if err != nil {
			return err
		}
		releaseID = id
		uploadURL = url
		return nil
	}, func() error {
		if releaseID != 0 {
			githubRequest("DELETE",
				fmt.Sprintf("/repos/%s/%s/releases/%d", owner, repo, releaseID),
				cfg.Token, nil,
			)
		}
		return nil
	})

	if isGo && uploadURL != "" {
		sha256lines := []string{}
		for _, bin := range binaries {
			b := bin
			printStepOrRollback("uploading "+b+"...", &undos, func() error {
				return uploadReleaseAsset(cfg.Token, uploadURL, b)
			}, nil)
			sum := fileSHA256(b)
			if sum != "" {
				sha256lines = append(sha256lines, sum+"  "+b)
			}
		}
		for _, bin := range binaries {
			os.Remove(bin)
		}
		if len(sha256lines) > 0 {
			checksumFile := "checksums.txt"
			os.WriteFile(checksumFile, []byte(strings.Join(sha256lines, "\n")+"\n"), 0644)
			printStepOrRollback("uploading checksums.txt...", &undos, func() error {
				return uploadReleaseAsset(cfg.Token, uploadURL, checksumFile)
			}, nil)
			os.Remove(checksumFile)
		}
	}

	fmt.Println()
	fmt.Printf("  \033[32m✓ v%s released\033[0m\n", nextVer)
	fmt.Printf("  https://github.com/%s/%s/releases/tag/v%s\n\n", owner, repo, nextVer)
}

func printStepOrRollback(label string, undos *[]undoStep, fn func() error, undo func() error) {
	fmt.Printf("  %-38s", label)
	if err := fn(); err != nil {
		fmt.Printf("\033[31m✗\033[0m\n")
		fmt.Printf("\n  %s step failed: %s\n", colorRed("✗"), err)
		if len(*undos) > 0 {
			fmt.Printf("\n  rolling back %d step(s)...\n", len(*undos))
			for i := len(*undos) - 1; i >= 0; i-- {
				u := (*undos)[i]
				fmt.Printf("  ↩  %s", u.label)
				if rerr := u.fn(); rerr != nil {
					fmt.Printf(" (could not undo: %s)\n", rerr)
				} else {
					fmt.Println(" done")
				}
			}
			fmt.Println()
			fmt.Println("  release rolled back cleanly. fix the issue and run commitdog release again.")
		}
		fmt.Println()
		os.Exit(1)
	}
	fmt.Printf("\033[32m✓\033[0m\n")
	if undo != nil {
		*undos = append(*undos, undoStep{label: strings.TrimSuffix(label, "..."), fn: undo})
	}
}

func getLatestGitTag() string {
	cmd := exec.Command("git", "tag", "--sort=-version:refname")
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			return strings.TrimPrefix(line, "v")
		}
	}
	return ""
}

func detectProject() (*releaseProject, string) {
	for i := range releaseProjects {
		p := &releaseProjects[i]
		data, err := os.ReadFile(p.file)
		if err != nil {
			continue
		}
		match := p.pattern.FindSubmatch(data)
		if match != nil {
			return p, string(match[1])
		}
	}
	return nil, ""
}

func bumpVersion(proj *releaseProject, newVer string) error {
	if proj.file == "" {
		return nil
	}
	data, err := os.ReadFile(proj.file)
	if err != nil {
		return err
	}
	updated := proj.replace(string(data), newVer)
	return os.WriteFile(proj.file, []byte(updated), 0644)
}

func splitVer(ver string) (int, int, int) {
	ver = strings.TrimPrefix(ver, "v")
	parts := strings.SplitN(ver, ".", 3)
	if len(parts) != 3 {
		return 0, 0, 0
	}
	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])
	patch, _ := strconv.Atoi(parts[2])
	return major, minor, patch
}

func isValidSemver(s string) bool {
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			return false
		}
	}
	return true
}

func uploadReleaseAsset(token, uploadURL, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return err
	}

	name := filepath.Base(path)
	url := uploadURL + "?name=" + name

	req, err := http.NewRequest("POST", url, f)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Accept", "application/vnd.github+json")
	req.ContentLength = stat.Size()

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("upload failed: HTTP %d — %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}
