package main

import (
	"path/filepath"
	"regexp"
	"strings"
)

type analysis struct {
	filesChanged     int
	filesAdded       []string
	filesDeleted     []string
	filesModified    []string
	primaryScope     string
	commitType       string
	addedFuncs       []string
	removedFuncs     []string
	renamedFuncs     [][2]string
	addedVars        []string
	removedVars      []string
	renamedFiles     [][2]string
	branchHint       string
	patterns         []string
	isMigration      bool
	isDepUpdate      bool
	isDocsOnly       bool
	isTestOnly       bool
	isConfigOnly     bool
	isDeleteOnly     bool
	isNewFiles       bool
	isRenameOnly     bool
	detectedVersions []string
	htmlElements     []string
	cssClasses       []string
	shellFuncs       []string
}

var (
	testFilePatterns = []string{
		"_test.go", ".test.ts", ".test.js", ".test.tsx", ".test.jsx",
		"_spec.rb", "_test.py", "test_", ".spec.",
	}
	docFilePatterns = []string{
		"README", ".md", ".rst", ".txt", "CHANGELOG", "LICENSE", "CONTRIBUTING",
	}
	htmlFilePatterns   = []string{".html", ".htm"}
	cssFilePatterns    = []string{".css", ".scss", ".less", ".sass"}
	shellFilePatterns  = []string{".sh", ".bash", ".zsh", ".fish"}
	configFilePatterns = []string{
		".toml", ".yaml", ".yml", ".json", ".env", ".ini", ".cfg",
		"Makefile", "Dockerfile", ".dockerignore", ".gitignore",
		"go.mod", "go.sum", "requirements.txt", "package.json",
		"pyproject.toml", "Cargo.toml",
	}
	lockFilePatterns = []string{
		"package-lock.json", "yarn.lock", "go.sum", "Cargo.lock",
		"Gemfile.lock", "poetry.lock", "pnpm-lock.yaml",
	}
	migrationPatterns = []string{
		"migration", "migrate", "schema", "001_", "002_", "003_",
	}
	depFilePatterns = []string{
		"package.json", "go.mod", "requirements.txt", "Cargo.toml",
		"pyproject.toml", "composer.json",
	}
)

var (
	reFuncGo   = regexp.MustCompile(`^\+func\s+(?:\(\w+\s+\*?\w+\)\s+)?(\w+)\s*\(`)
	reFuncJS   = regexp.MustCompile(`^\+(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*\(`)
	reArrowJS  = regexp.MustCompile(`^\+(?:export\s+)?(?:const|let)\s+(\w+)\s*=\s*(?:async\s*)?\(`)
	reFuncPy   = regexp.MustCompile(`^\+def\s+(\w+)\s*\(`)
	reClassPy  = regexp.MustCompile(`^\+class\s+(\w+)[\s:(]`)
	reFuncRb   = regexp.MustCompile(`^\+\s*def\s+(\w+)`)
	reFuncRs   = regexp.MustCompile(`^\+(?:pub\s+)?fn\s+(\w+)\s*[<(]`)
	reFuncJava = regexp.MustCompile(`^\+\s*(?:public|private|protected|static|\s)+\s+\w+\s+(\w+)\s*\(`)

	reRmFuncGo = regexp.MustCompile(`^-func\s+(?:\(\w+\s+\*?\w+\)\s+)?(\w+)\s*\(`)
	reRmFuncJS = regexp.MustCompile(`^-(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*\(`)
	reRmFuncPy = regexp.MustCompile(`^-def\s+(\w+)\s*\(`)
	reRmFuncRs = regexp.MustCompile(`^-(?:pub\s+)?fn\s+(\w+)\s*[<(]`)

	reVarGoAdd = regexp.MustCompile(`^\+(?:var|const)\s+(\w{2,})\b`)
	reVarGoRm  = regexp.MustCompile(`^-(?:var|const)\s+(\w{2,})\b`)
	reVarJSAdd = regexp.MustCompile(`^\+(?:const|let|var)\s+(\w{2,})\s*[=;]`)
	reVarJSRm  = regexp.MustCompile(`^-(?:const|let|var)\s+(\w{2,})\s*[=;]`)
	reVarPyAdd = regexp.MustCompile(`^\+([A-Z][A-Z0-9_]{2,})\s*=`)
	reVarPyRm  = regexp.MustCompile(`^-([A-Z][A-Z0-9_]{2,})\s*=`)

	reVersion      = regexp.MustCompile(`v\d+\.\d+\.\d+`)
	reHTMLTag      = regexp.MustCompile(`^\+\s*<(section|article|nav|header|footer|main|form|dialog|template|aside|table|ul|ol)\b`)
	reHTMLId       = regexp.MustCompile(`^\+.*\bid="([a-zA-Z][\w-]+)"`)
	reCSSClass     = regexp.MustCompile(`^\+\.([a-zA-Z][\w-]{2,})\s*(?:\{|,)`)
	reCSSVar       = regexp.MustCompile(`^\+\s*--([a-zA-Z][\w-]+)\s*:`)
	reFuncSh       = regexp.MustCompile(`^\+(?:function\s+)?(\w+)\s*\(\s*\)\s*\{`)
	reVarGoBlock   = regexp.MustCompile(`^\+\t([a-zA-Z]\w{2,})\s*(?:=|\s+\*?[A-Z])`)
	reVarGoBlockRm = regexp.MustCompile(`^-\t([a-zA-Z]\w{2,})\s*(?:=|\s+\*?[A-Z])`)

	reErrorHandling = regexp.MustCompile(`^\+.*(?:err|error|Error|exception|Exception|catch|rescue)\b`)
	reLogging       = regexp.MustCompile(`^\+.*(?:log\.|logger\.|console\.log|fmt\.Print|println!|logging\.)`)
	reRmLogging     = regexp.MustCompile(`^-.*(?:console\.log|println!|log\.Debug|log\.Printf|log\.Println)`)
	reTest          = regexp.MustCompile(`^\+.*(?:func Test|it\(|describe\(|test\(|assert|expect\()`)
	reComment       = regexp.MustCompile(`^\+\s*(?://|#|/\*|\*)`)
)

func analyzeDiffWithBranch(diff string, branch string) analysis {
	a := analysis{}
	a.branchHint = branch

	lines := strings.Split(diff, "\n")
	currentFile := ""
	isDeleted := false
	isNew := false
	isRename := false
	renameFrom := ""

	for _, line := range lines {
		if strings.HasPrefix(line, "deleted file mode") {
			isDeleted = true
			continue
		}

		if strings.HasPrefix(line, "new file mode") {
			isNew = true
			continue
		}

		if strings.HasPrefix(line, "diff --git ") {
			if isNew && currentFile != "" {
				a.filesAdded = appendUnique(a.filesAdded, currentFile)
				a.isNewFiles = true
			}
			if isDeleted && currentFile != "" {
				a.filesDeleted = appendUnique(a.filesDeleted, currentFile)
			}
			isDeleted = false
			isNew = false
			isRename = false
			renameFrom = ""
			currentFile = parseDiffGitPath(line)
			continue
		}

		if strings.HasPrefix(line, "rename from ") {
			renameFrom = strings.TrimSpace(strings.TrimPrefix(line, "rename from "))
			continue
		}

		if strings.HasPrefix(line, "rename to ") {
			isRename = true
			renameTo := strings.TrimSpace(strings.TrimPrefix(line, "rename to "))
			a.filesModified = appendUnique(a.filesModified, renameTo)
			if renameFrom != "" {
				a.renamedFiles = append(a.renamedFiles, [2]string{renameFrom, renameTo})
			}
			currentFile = renameTo
			continue
		}

		if strings.HasPrefix(line, "--- a/") && isDeleted {
			path := strings.TrimSpace(strings.TrimPrefix(line, "--- a/"))
			a.filesDeleted = appendUnique(a.filesDeleted, path)
			currentFile = path
			continue
		}

		if strings.HasPrefix(line, "+++ b/") {
			currentFile = strings.TrimSpace(strings.TrimPrefix(line, "+++ b/"))
			if isDeleted || isRename {
				continue
			}
			if isNew {
				a.filesAdded = appendUnique(a.filesAdded, currentFile)
				a.isNewFiles = true
			} else {
				categorizeFile(&a, currentFile)
			}
			continue
		}

		if strings.HasPrefix(line, "Binary files") && strings.Contains(line, "differ") {
			if currentFile != "" && !isNew && !isDeleted {
				a.filesModified = appendUnique(a.filesModified, currentFile)
			}
			continue
		}

		if strings.HasPrefix(line, "---") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "@@") {
			continue
		}

		if isDeleted || isRename {
			continue
		}

		if funcs := extractFuncName(line, currentFile, true); funcs != "" {
			a.addedFuncs = appendUnique(a.addedFuncs, funcs)
		}
		if funcs := extractFuncName(line, currentFile, false); funcs != "" {
			a.removedFuncs = appendUnique(a.removedFuncs, funcs)
		}

		if v := extractVarName(line, currentFile, true); v != "" {
			a.addedVars = appendUnique(a.addedVars, v)
		}
		if v := extractVarName(line, currentFile, false); v != "" {
			a.removedVars = appendUnique(a.removedVars, v)
		}

		if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
			if m := reVersion.FindString(line); m != "" {
				if strings.HasPrefix(line, "+") {
					a.detectedVersions = appendUnique(a.detectedVersions, m)
				}
			}
		}

		ext := strings.ToLower(filepath.Ext(currentFile))
		if ext == ".html" || ext == ".htm" {
			if m := reHTMLTag.FindStringSubmatch(line); len(m) > 1 {
				a.htmlElements = appendUnique(a.htmlElements, m[1])
			}
			if m := reHTMLId.FindStringSubmatch(line); len(m) > 1 {
				a.htmlElements = appendUnique(a.htmlElements, m[1])
			}
		}
		if ext == ".css" || ext == ".scss" || ext == ".less" || ext == ".sass" {
			if m := reCSSClass.FindStringSubmatch(line); len(m) > 1 {
				a.cssClasses = appendUnique(a.cssClasses, m[1])
			}
			if m := reCSSVar.FindStringSubmatch(line); len(m) > 1 {
				a.cssClasses = appendUnique(a.cssClasses, "--"+m[1])
			}
		}
		if ext == ".sh" || ext == ".bash" || ext == ".zsh" || ext == ".fish" {
			if m := reFuncSh.FindStringSubmatch(line); len(m) > 1 {
				a.shellFuncs = appendUnique(a.shellFuncs, m[1])
			}
		}

		if reErrorHandling.MatchString(line) {
			a.patterns = appendUnique(a.patterns, "error-handling")
		}
		if reRmLogging.MatchString(line) {
			a.patterns = appendUnique(a.patterns, "remove-debug-logs")
		}
		if reLogging.MatchString(line) {
			a.patterns = appendUnique(a.patterns, "logging")
		}
		if reTest.MatchString(line) {
			a.patterns = appendUnique(a.patterns, "tests")
		}
		if reComment.MatchString(line) {
			a.patterns = appendUnique(a.patterns, "comments")
		}
	}

	if isNew && currentFile != "" {
		a.filesAdded = appendUnique(a.filesAdded, currentFile)
		a.isNewFiles = true
	}
	if isDeleted && currentFile != "" {
		a.filesDeleted = appendUnique(a.filesDeleted, currentFile)
	}

	detectRenamedFuncs(&a)

	a.filesChanged = len(a.filesAdded) + len(a.filesDeleted) + len(a.filesModified)

	if len(a.filesDeleted) > 0 && len(a.filesAdded) == 0 && len(a.filesModified) == 0 {
		a.isDeleteOnly = true
	}

	if len(a.renamedFiles) > 0 && len(a.filesAdded) == 0 && len(a.filesDeleted) == 0 &&
		len(a.addedFuncs) == 0 && len(a.removedFuncs) == 0 {
		a.isRenameOnly = true
	}

	a.primaryScope = inferScope(a)
	a.commitType = inferType(a)

	return a
}

func categorizeFile(a *analysis, path string) {
	base := filepath.Base(path)
	lower := strings.ToLower(path)

	for _, p := range lockFilePatterns {
		if strings.HasSuffix(lower, strings.ToLower(p)) || base == p {
			a.isDepUpdate = true
			return
		}
	}

	for _, p := range migrationPatterns {
		if strings.Contains(lower, p) {
			a.isMigration = true
		}
	}

	for _, p := range depFilePatterns {
		if base == p {
			a.isDepUpdate = true
		}
	}

	if isTestFile(path) {
		a.filesModified = appendUnique(a.filesModified, path)
		a.isTestOnly = true
		return
	}
	if isDocFile(path) {
		a.filesModified = appendUnique(a.filesModified, path)
		a.isDocsOnly = true
		return
	}
	if isHTMLFile(path) || isCSSFile(path) || isShellFile(path) {
		a.filesModified = appendUnique(a.filesModified, path)
		a.isTestOnly = false
		a.isDocsOnly = false
		a.isConfigOnly = false
		return
	}
	if isConfigFile(path) {
		a.filesModified = appendUnique(a.filesModified, path)
		a.isConfigOnly = true
		return
	}

	a.filesModified = appendUnique(a.filesModified, path)
	a.isTestOnly = false
	a.isDocsOnly = false
	a.isConfigOnly = false
}

func extractVarName(line, file string, added bool) string {
	ext := strings.ToLower(filepath.Ext(file))
	var re *regexp.Regexp

	if added {
		switch ext {
		case ".go":
			re = reVarGoAdd
		case ".js", ".ts", ".jsx", ".tsx":
			re = reVarJSAdd
		case ".py":
			re = reVarPyAdd
		}
	} else {
		switch ext {
		case ".go":
			re = reVarGoRm
		case ".js", ".ts", ".jsx", ".tsx":
			re = reVarJSRm
		case ".py":
			re = reVarPyRm
		}
	}

	if re == nil {
		if ext == ".go" {
			var blockRe *regexp.Regexp
			if added {
				blockRe = reVarGoBlock
			} else {
				blockRe = reVarGoBlockRm
			}
			if m := blockRe.FindStringSubmatch(line); len(m) > 1 {
				name := m[1]
				if len(name) > 1 {
					return name
				}
			}
		}
		return ""
	}
	if m := re.FindStringSubmatch(line); len(m) > 1 {
		name := m[1]
		if len(name) <= 1 {
			return ""
		}
		return name
	}
	return ""
}

func detectRenamedFuncs(a *analysis) {
	if len(a.addedFuncs) == 0 || len(a.removedFuncs) == 0 {
		return
	}
	used := map[string]bool{}
	for _, added := range a.addedFuncs {
		for _, removed := range a.removedFuncs {
			if used[removed] {
				continue
			}
			if isSimilarName(removed, added) {
				a.renamedFuncs = append(a.renamedFuncs, [2]string{removed, added})
				used[removed] = true
				break
			}
		}
	}
}

func isSimilarName(a, b string) bool {
	al := strings.ToLower(a)
	bl := strings.ToLower(b)
	if strings.Contains(al, bl) || strings.Contains(bl, al) {
		return true
	}
	if len(al) > 3 && len(bl) > 3 && al[:3] == bl[:3] {
		return true
	}
	return false
}

func extractFuncName(line, file string, added bool) string {
	ext := strings.ToLower(filepath.Ext(file))
	var patterns []*regexp.Regexp

	if added {
		switch ext {
		case ".go":
			patterns = []*regexp.Regexp{reFuncGo}
		case ".js", ".ts", ".jsx", ".tsx":
			patterns = []*regexp.Regexp{reFuncJS, reArrowJS}
		case ".py":
			patterns = []*regexp.Regexp{reFuncPy, reClassPy}
		case ".rb":
			patterns = []*regexp.Regexp{reFuncRb}
		case ".rs":
			patterns = []*regexp.Regexp{reFuncRs}
		case ".java", ".kt":
			patterns = []*regexp.Regexp{reFuncJava}
		case ".sh", ".bash", ".zsh", ".fish":
			patterns = []*regexp.Regexp{reFuncSh}
		}
	} else {
		switch ext {
		case ".go":
			patterns = []*regexp.Regexp{reRmFuncGo}
		case ".js", ".ts", ".jsx", ".tsx":
			patterns = []*regexp.Regexp{reRmFuncJS}
		case ".py":
			patterns = []*regexp.Regexp{reRmFuncPy}
		case ".rs":
			patterns = []*regexp.Regexp{reRmFuncRs}
		}
	}

	for _, re := range patterns {
		if m := re.FindStringSubmatch(line); len(m) > 1 {
			name := m[1]
			if len(name) <= 1 || strings.HasPrefix(name, "_") {
				continue
			}
			return name
		}
	}
	return ""
}

func inferScope(a analysis) string {
	all := append(append(a.filesAdded, a.filesDeleted...), a.filesModified...)
	if len(all) == 0 {
		return ""
	}

	scores := map[string]int{}
	for _, f := range all {
		parts := strings.Split(filepath.Dir(f), "/")
		for _, p := range parts {
			p = strings.ToLower(p)
			if p == "" || p == "." || p == "src" || p == "lib" ||
				p == "pkg" || p == "internal" || p == "app" ||
				p == "cmd" || p == "main" {
				continue
			}
			scores[p]++
		}
		base := strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
		base = strings.ToLower(base)
		base = strings.TrimSuffix(base, "_test")
		base = strings.TrimSuffix(base, ".test")
		base = strings.TrimSuffix(base, ".spec")
		if base != "" && base != "main" && base != "index" && base != ".gitkeep" {
			scores[base]++
		}
	}

	best := ""
	bestScore := 0
	for k, v := range scores {
		if v > bestScore || (v == bestScore && k < best) {
			best = k
			bestScore = v
		}
	}

	if len(best) > 20 {
		best = best[:20]
	}

	return best
}

func inferType(a analysis) string {
	if a.branchHint != "" {
		hint := strings.ToLower(a.branchHint)
		if strings.HasPrefix(hint, "fix/") || strings.HasPrefix(hint, "bugfix/") || strings.HasPrefix(hint, "hotfix/") {
			return "fix"
		}
		if strings.HasPrefix(hint, "feat/") || strings.HasPrefix(hint, "feature/") {
			return "feat"
		}
		if strings.HasPrefix(hint, "docs/") {
			return "docs"
		}
		if strings.HasPrefix(hint, "refactor/") || strings.HasPrefix(hint, "chore/") {
			return "refactor"
		}
		if strings.HasPrefix(hint, "test/") {
			return "test"
		}
	}

	if len(a.detectedVersions) > 0 && len(a.addedFuncs) == 0 && len(a.removedFuncs) == 0 {
		return "chore"
	}

	if len(a.htmlElements) > 0 {
		return "feat"
	}

	if len(a.cssClasses) > 0 {
		return "feat"
	}

	if a.isRenameOnly {
		return "refactor"
	}
	if a.isDeleteOnly {
		return "chore"
	}
	if a.isNewFiles && len(a.filesModified) == 0 && len(a.filesDeleted) == 0 {
		return "feat"
	}
	if a.isDepUpdate && len(a.filesModified) <= 2 {
		return "chore"
	}
	if a.isMigration {
		return "feat"
	}
	if a.isDocsOnly {
		return "docs"
	}
	if a.isTestOnly {
		return "test"
	}
	if a.isConfigOnly {
		return "chore"
	}
	if contains(a.patterns, "remove-debug-logs") && len(a.filesModified) <= 3 {
		return "chore"
	}
	if len(a.renamedFuncs) > 0 && len(a.addedFuncs) == len(a.renamedFuncs) {
		return "refactor"
	}
	if len(a.addedFuncs) > 0 && len(a.removedFuncs) == 0 {
		return "feat"
	}
	if len(a.removedFuncs) > 0 && len(a.addedFuncs) == 0 {
		return "refactor"
	}
	if contains(a.patterns, "error-handling") {
		return "fix"
	}
	if len(a.filesAdded) > 0 {
		return "feat"
	}
	if len(a.filesDeleted) > 0 {
		return "refactor"
	}
	return "refactor"
}

func isTestFile(path string) bool {
	lower := strings.ToLower(path)
	for _, p := range testFilePatterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

func isDocFile(path string) bool {
	base := strings.ToUpper(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(path))
	for _, p := range docFilePatterns {
		if strings.Contains(base, strings.ToUpper(p)) || ext == strings.ToLower(p) {
			return true
		}
	}
	return false
}

func isConfigFile(path string) bool {
	base := filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(path))
	for _, p := range configFilePatterns {
		if base == p || ext == strings.ToLower(p) {
			return true
		}
	}
	return false
}

func parseDiffGitPath(line string) string {
	parts := strings.Fields(line)
	if len(parts) < 4 {
		return ""
	}
	b := parts[3]
	if strings.HasPrefix(b, "b/") {
		return b[2:]
	}
	return ""
}

func appendUnique(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func firstN(slice []string, n int) []string {
	if len(slice) <= n {
		return slice
	}
	return slice[:n]
}

func isHTMLFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, p := range htmlFilePatterns {
		if ext == p {
			return true
		}
	}
	return false
}

func isCSSFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, p := range cssFilePatterns {
		if ext == p {
			return true
		}
	}
	return false
}

func isShellFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, p := range shellFilePatterns {
		if ext == p {
			return true
		}
	}
	return false
}
