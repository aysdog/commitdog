package main

import (
	"path/filepath"
	"regexp"
	"strings"
)

type analysis struct {
	filesChanged  int
	filesAdded    []string
	filesDeleted  []string
	filesModified []string
	primaryScope  string
	commitType    string
	addedFuncs    []string
	removedFuncs  []string
	patterns      []string
	isMigration   bool
	isDepUpdate   bool
	isDocsOnly    bool
	isTestOnly    bool
	isConfigOnly  bool
	isDeleteOnly  bool
}

var (
	testFilePatterns = []string{
		"_test.go", ".test.ts", ".test.js", ".test.tsx", ".test.jsx",
		"_spec.rb", "_test.py", "test_", ".spec.",
	}
	docFilePatterns = []string{
		"README", ".md", ".rst", ".txt", "CHANGELOG", "LICENSE", "CONTRIBUTING",
	}
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

	reErrorHandling = regexp.MustCompile(`^\+.*(?:err|error|Error|exception|Exception|catch|rescue)\b`)
	reLogging       = regexp.MustCompile(`^\+.*(?:log\.|logger\.|console\.log|fmt\.Print|println!|logging\.)`)
	reRmLogging     = regexp.MustCompile(`^-.*(?:console\.log|fmt\.Print|println!|log\.Debug)`)
	reTest          = regexp.MustCompile(`^\+.*(?:func Test|it\(|describe\(|test\(|assert|expect\()`)
	reComment       = regexp.MustCompile(`^\+\s*(?://|#|/\*|\*)`)
)

func analyzeDiff(diff string) analysis {
	a := analysis{}

	lines := strings.Split(diff, "\n")
	currentFile := ""
	isDeleted := false

	for _, line := range lines {
		// detect deleted file header
		if strings.HasPrefix(line, "deleted file mode") {
			isDeleted = true
			continue
		}

		// new diff block
		if strings.HasPrefix(line, "diff --git ") {
			isDeleted = false
			currentFile = ""
			continue
		}

		// track deleted file path from --- line
		if strings.HasPrefix(line, "--- a/") && isDeleted {
			path := strings.TrimPrefix(line, "--- a/")
			path = strings.TrimSpace(path)
			a.filesDeleted = appendUnique(a.filesDeleted, path)
			currentFile = path
			continue
		}

		if strings.HasPrefix(line, "+++ b/") {
			currentFile = strings.TrimPrefix(line, "+++ b/")
			currentFile = strings.TrimSpace(currentFile)
			if !isDeleted {
				categorizeFile(&a, currentFile)
			}
			continue
		}

		if strings.HasPrefix(line, "---") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "@@") {
			continue
		}

		if isDeleted {
			continue
		}

		if funcs := extractFuncName(line, currentFile, true); funcs != "" {
			a.addedFuncs = appendUnique(a.addedFuncs, funcs)
		}
		if funcs := extractFuncName(line, currentFile, false); funcs != "" {
			a.removedFuncs = appendUnique(a.removedFuncs, funcs)
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

	a.filesChanged = len(a.filesAdded) + len(a.filesDeleted) + len(a.filesModified)

	// pure deletion — no additions, no modifications
	if len(a.filesDeleted) > 0 && len(a.filesAdded) == 0 && len(a.filesModified) == 0 {
		a.isDeleteOnly = true
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
		if base != "" && base != "main" && base != "index" {
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
	if a.isDeleteOnly {
		return "chore"
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
