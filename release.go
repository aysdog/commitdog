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

	releaseAll := false
	mirrorOnly := ""
	for _, arg := range os.Args[2:] {
		switch arg {
		case "--changelog-only", "-changelog-only":
			cl := buildChangelog(getLatestGitTag())
			fmt.Println()
			fmt.Println(cl)
			return
		case "config":
			runReleaseConfig()
			return
		case "--all", "-all":
			releaseAll = true
		case "-gh":
			mirrorOnly = "github"
		case "-gl":
			mirrorOnly = "gitlab"
		case "-gt":
			mirrorOnly = "gitea"
		case "-fg":
			mirrorOnly = "forgejo"
		}
	}

	cfg := loadConfig()
	proj2 := loadProjectConfig()

	if mirrorOnly != "" {
		runMirrorRelease(cfg, proj2, mirrorOnly)
		return
	}

	platform := proj2.effectivePrimary()
	if platform == "" {
		platform = "github"
	}
	token := tokenForPlatform(cfg, platform)
	if token == "" {
		fatal("no %s token found. run 'commitdog setup' first.", platform)
	}
	owner, repo := getRepoOwnerAndName()
	if owner == "" || repo == "" {
		fatal("could not detect %s repo.", platform)
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

	if gitTag == "" && hasRemoteTags() {
		fmt.Println()
		fmt.Println("  no local tags found but remote has tags.")
		fmt.Println()
		fmt.Println("  1  mirror repo — fetch tags from remote")
		fmt.Println("  2  new repo — start fresh")
		fmt.Println()
		fmt.Printf("  [1/2/q] pick › ")
		for {
			switch strings.TrimSpace(readLine()) {
			case "1":
				fmt.Printf("  fetching tags...")
				if err := fetchTags(); err != nil {
					fmt.Println()
					fatal("could not fetch tags: %v", err)
				}
				fmt.Println(" done")
				gitTag = getLatestGitTag()
			case "2":
			case "q", "":
				fmt.Println("  aborted.")
				return
			default:
				fmt.Printf("  1, 2, or q › ")
				continue
			}
			break
		}
		fmt.Println()
	}

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
	fmt.Printf("  detected: %s  ·  current version: %s\n\n", colorCyan(proj.lang), colorBold("v"+currentVer))

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
	branch := getCurrentBranch()

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

	authHeader := authHeaderForPlatform(token)
	printStepOrRollback("pushing...", &undos, func() error {
		return runPushTagsWithAuth("origin", branch, authHeader)
	}, func() error {
		exec.Command("git", "push", "origin", ":refs/tags/v"+nextVer).Run()
		if preRelease != "" {
			exec.Command("git", "push", "origin", preRelease+":"+branch, "--force").Run()
		}
		return nil
	})

	checksumFile := buildChecksums(binaries)
	releaseURL := platformRelease(cfg, platform, owner, repo, nextVer, changelog, isGo, binaries, checksumFile, &undos)

	if releaseAll {
		proj2 = loadProjectConfig()
		for _, mirror := range proj2.mirrors {
			var mirrorUndos []undoStep
			mirrorRemote := platformRemoteName(mirror)
			mirrorAuth := authHeaderForPlatformName(mirror)
			printStepOrRollback("pushing tags to "+mirror+"...", &mirrorUndos, func() error {
				return runPushTagsWithAuth(mirrorRemote, branch, mirrorAuth)
			}, nil)
			mirrorOwner, mirrorRepo := getMirrorOwnerRepo(mirrorRemote)
			if mirrorOwner == "" || mirrorRepo == "" {
				fmt.Printf("  %s could not resolve owner/repo for %s — skipping\n", colorYellow("⚠"), mirror)
				continue
			}
			mirrorURL := platformRelease(cfg, mirror, mirrorOwner, mirrorRepo, nextVer, changelog, isGo, binaries, checksumFile, &mirrorUndos)
			fmt.Printf("  %s\n", mirrorURL)
		}
	}

	for _, bin := range binaries {
		os.Remove(bin)
	}
	os.Remove("checksums.txt")

	fmt.Println()
	fmt.Printf("  %s\n", colorGreen("✓ v"+nextVer+" released"))
	fmt.Printf("  %s\n\n", releaseURL)
}

func getMirrorOwnerRepo(remote string) (string, string) {
	cmd := exec.Command("git", "remote", "get-url", remote)
	var out strings.Builder
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", ""
	}
	return parseRemoteOwnerAndName(strings.TrimSpace(out.String()))
}

func printStepOrRollback(label string, undos *[]undoStep, fn func() error, undo func() error) {
	fmt.Printf("  %-38s", label)
	if err := fn(); err != nil {
		fmt.Printf("%s\n", colorRed("✗"))
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
	fmt.Printf("%s\n", colorGreen("✓"))
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

func buildChecksums(binaries []string) string {
	var lines []string
	for _, bin := range binaries {
		sum := fileSHA256(bin)
		if sum != "" {
			lines = append(lines, sum+"  "+bin)
		}
	}
	if len(lines) == 0 {
		return ""
	}
	checksumFile := "checksums.txt"
	os.WriteFile(checksumFile, []byte(strings.Join(lines, "\n")+"\n"), 0644)
	return checksumFile
}

func platformRelease(cfg config, platform, owner, repo, nextVer, changelog string, isGo bool, binaries []string, checksumFile string, undos *[]undoStep) string {
	token := tokenForPlatform(cfg, platform)

	switch platform {
	case "gitlab":
		return releaseGitLab(cfg, token, owner, repo, nextVer, changelog, isGo, binaries, checksumFile, undos)
	case "gitea":
		return releaseGitea(token, cfg.Gitea.Host, owner, repo, nextVer, changelog, isGo, binaries, checksumFile, undos)
	case "forgejo":
		return releaseForgejo(token, cfg.Forgejo.Host, owner, repo, nextVer, changelog, isGo, binaries, checksumFile, undos)
	default:
		return releaseGitHub(token, owner, repo, nextVer, changelog, isGo, binaries, checksumFile, undos)
	}
}

func releaseGitHub(token, owner, repo, nextVer, changelog string, isGo bool, binaries []string, checksumFile string, undos *[]undoStep) string {
	var releaseID int64
	var uploadURL string
	printStepOrRollback("creating GitHub release...", undos, func() error {
		id, url, err := createGitHubReleaseWithBody(token, owner, repo, nextVer, changelog)
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
				token, nil,
			)
		}
		return nil
	})

	if isGo && uploadURL != "" {
		for _, bin := range binaries {
			b := bin
			printStepOrRollback("uploading "+b+"...", undos, func() error {
				return uploadReleaseAsset(token, uploadURL, b)
			}, nil)
		}
		if checksumFile != "" {
			printStepOrRollback("uploading checksums.txt...", undos, func() error {
				return uploadReleaseAsset(token, uploadURL, checksumFile)
			}, nil)
		}
	}

	return fmt.Sprintf("https://github.com/%s/%s/releases/tag/v%s", owner, repo, nextVer)
}

func releaseGitLab(cfg config, token, owner, repo, nextVer, changelog string, isGo bool, binaries []string, checksumFile string, undos *[]undoStep) string {
	host := gitlabHost(cfg)

	var projectID string
	printStepOrRollback("creating GitLab release...", undos, func() error {
		id, err := getGitLabProjectID(token, host, owner, repo)
		if err != nil {
			return err
		}
		projectID = id
		return createGitLabRelease(token, host, projectID, nextVer, changelog)
	}, func() error {
		if projectID != "" {
			gitlabRequest("DELETE",
				fmt.Sprintf("/projects/%s/releases/v%s", projectID, nextVer),
				token, host, nil,
			)
		}
		return nil
	})

	if isGo && projectID != "" {
		allFiles := append(binaries, checksumFile)
		for _, bin := range allFiles {
			if bin == "" {
				continue
			}
			b := bin
			printStepOrRollback("uploading "+b+"...", undos, func() error {
				pkgURL, err := uploadGitLabPackage(token, host, projectID, repo, nextVer, b)
				if err != nil {
					return err
				}
				return addGitLabReleaseLink(token, host, projectID, nextVer, filepath.Base(b), pkgURL)
			}, nil)
		}
	}

	return fmt.Sprintf("%s/%s/%s/-/releases/v%s", host, owner, repo, nextVer)
}

func releaseGitea(token, host, owner, repo, nextVer, changelog string, isGo bool, binaries []string, checksumFile string, undos *[]undoStep) string {
	var releaseID int64
	printStepOrRollback("creating Gitea release...", undos, func() error {
		id, err := createGiteaRelease(token, host, owner, repo, nextVer, changelog)
		if err != nil {
			return err
		}
		releaseID = id
		return nil
	}, func() error {
		if releaseID != 0 {
			giteaRequest("DELETE",
				fmt.Sprintf("/repos/%s/%s/releases/%d", owner, repo, releaseID),
				token, host, nil,
			)
		}
		return nil
	})

	if isGo && releaseID != 0 {
		var toUpload []string
		for _, f := range append(binaries, checksumFile) {
			if f != "" {
				toUpload = append(toUpload, f)
			}
		}
		printStepOrRollback("uploading assets...", undos, func() error {
			return uploadGiteaAssetsBatched(token, host, owner, repo, releaseID, toUpload)
		}, nil)
	}

	return fmt.Sprintf("%s/%s/%s/releases/tag/v%s", host, owner, repo, nextVer)
}

func releaseForgejo(token, host, owner, repo, nextVer, changelog string, isGo bool, binaries []string, checksumFile string, undos *[]undoStep) string {
	var releaseID int64
	printStepOrRollback("creating Forgejo release...", undos, func() error {
		id, err := createForgejoRelease(token, host, owner, repo, nextVer, changelog)
		if err != nil {
			return err
		}
		releaseID = id
		return nil
	}, func() error {
		if releaseID != 0 {
			forgejoRequest("DELETE",
				fmt.Sprintf("/repos/%s/%s/releases/%d", owner, repo, releaseID),
				token, host, nil,
			)
		}
		return nil
	})

	if isGo && releaseID != 0 {
		var toUpload []string
		for _, f := range append(binaries, checksumFile) {
			if f != "" {
				toUpload = append(toUpload, f)
			}
		}
		printStepOrRollback("uploading assets...", undos, func() error {
			return uploadForgejoAssetsBatched(token, host, owner, repo, releaseID, toUpload)
		}, nil)
	}

	return fmt.Sprintf("%s/%s/%s/releases/tag/v%s", host, owner, repo, nextVer)
}

func hasRemoteTags() bool {
	cmd := exec.Command("git", "ls-remote", "--tags", "origin")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return false
	}
	return strings.TrimSpace(out.String()) != ""
}

func fetchTags() error {
	cmd := exec.Command("git", "fetch", "--tags", "origin")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func runMirrorRelease(cfg config, proj projectConfig, platform string) {
	token := tokenForPlatform(cfg, platform)
	if token == "" {
		fatal("no %s token found. run 'commitdog setup' first.", platform)
	}

	gitTag := getLatestGitTag()
	if gitTag == "" {
		fatal("no git tag found. run 'commitdog release' first to create a release.")
	}

	remote := platformRemoteName(platform)
	if !remoteExists(remote) {
		fatal("remote '%s' not configured. run 'commitdog init' to add %s as a mirror.", remote, platform)
	}

	fmt.Println()
	fmt.Printf("  publishing v%s to %s\n\n", gitTag, platform)

	authHeader := authHeaderForPlatformName(platform)
	printStepOrRollback("pushing tags to "+platform+"...", nil, func() error {
		return runPushTagsWithAuth(remote, getCurrentBranch(), authHeader)
	}, nil)

	owner, repo := getMirrorOwnerRepo(remote)
	if owner == "" || repo == "" {
		fatal("could not detect repo owner and name from remote '%s'.", remote)
	}

	changelog := buildChangelog(gitTag)

	proj2 := loadProjectConfig()
	isGo := false
	if p, _ := detectProject(); p != nil && p.lang == "Go" {
		isGo = true
	}

	var binaries []string
	var undos []undoStep
	if isGo {
		targets := resolveTargets(proj2.targets)
		if len(targets) == 0 {
			targets = allBuildTargets
		}
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

	releaseURL := platformRelease(cfg, platform, owner, repo, gitTag, changelog, isGo, binaries, buildChecksums(binaries), &undos)

	for _, bin := range binaries {
		os.Remove(bin)
	}
	os.Remove("checksums.txt")

	fmt.Println()
	fmt.Printf("  %s\n", colorGreen("✓ v"+gitTag+" published to "+platform))
	fmt.Printf("  %s\n\n", releaseURL)
}
