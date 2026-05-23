package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

func isLargeDiff(a analysis) bool {
	return len(a.filesAdded)+len(a.filesModified)+len(a.filesDeleted) > 2
}

func subjectLine(s string) string {
	if idx := strings.Index(s, "\n"); idx >= 0 {
		return s[:idx]
	}
	return s
}

func bodyPreview(s string) string {
	idx := strings.Index(s, "\n\n")
	if idx < 0 {
		return ""
	}
	body := strings.TrimSpace(s[idx+2:])
	lines := strings.Split(body, "\n")
	if len(lines) == 0 {
		return ""
	}
	first := lines[0]
	if len(first) > 60 {
		first = first[:57] + "..."
	}
	return first
}

func withBody(subject string, a analysis) string {
	body := buildBody(a)
	if body == "" {
		return subject
	}
	return subject + "\n\n" + body
}

func safeUpdateVerb(name string) string {
	reserved := map[string]bool{
		"update": true, "fix": true, "refactor": true, "feat": true,
		"chore": true, "delete": true, "remove": true, "add": true,
	}
	if reserved[name] {
		return "clean up " + name
	}
	return "update " + name
}

func wrapFuncList(prefix string, names []string) string {
	const maxWidth = 72
	line := prefix + " " + names[0]
	var result []string
	for _, name := range names[1:] {
		next := line + ", " + name
		if len(next) > maxWidth {
			result = append(result, line+",")
			line = "  " + name
		} else {
			line = next
		}
	}
	result = append(result, line)
	return strings.Join(result, "\n")
}

func buildBody(a analysis) string {
	hasSignificantFuncs := len(a.addedFuncs)+len(a.removedFuncs) > 2
	if !isLargeDiff(a) && !hasSignificantFuncs {
		return ""
	}

	var lines []string

	if len(a.addedFuncs) > 0 {
		lines = append(lines, wrapFuncList("adds", a.addedFuncs))
	} else if len(a.filesAdded) > 0 {
		names := make([]string, 0, len(a.filesAdded))
		for _, f := range a.filesAdded {
			names = append(names, cleanFileName(f))
		}
		lines = append(lines, wrapFuncList("adds", names))
	}

	if len(a.sqlTables) > 0 {
		lines = append(lines, wrapFuncList("tables", a.sqlTables))
	}

	if len(a.removedFuncs) > 0 {
		lines = append(lines, wrapFuncList("removes", a.removedFuncs))
	} else if len(a.filesDeleted) > 0 && len(lines) > 0 {
		names := make([]string, 0, len(a.filesDeleted))
		for _, f := range a.filesDeleted {
			names = append(names, cleanFileName(f))
		}
		lines = append(lines, wrapFuncList("removes", names))
	}

	if len(a.filesModified) > 0 && len(a.addedFuncs) == 0 && len(a.removedFuncs) == 0 {
		names := make([]string, 0, len(a.filesModified))
		for _, f := range a.filesModified {
			names = append(names, cleanFileName(f))
		}
		lines = append(lines, wrapFuncList("updates", names))
	}

	return strings.Join(lines, "\n")
}

func generateSuggestions(a analysis) []string {
	var suggestions []string

	s1 := buildPrimary(a)
	s2 := buildAlternate(a)
	s3 := buildBrief(a)
	s4 := buildSummary(a)

	suggestions = append(suggestions, withBody(s1, a))
	if s2 != "" && s2 != s1 {
		suggestions = append(suggestions, withBody(s2, a))
	}
	if s3 != "" && s3 != s1 && s3 != s2 {
		suggestions = append(suggestions, withBody(s3, a))
	}

	if len(suggestions) < 2 {
		suggestions = append(suggestions, withBody(buildFallback(a), a))
	}

	if s4 != "" && s4 != s1 && s4 != s2 && s4 != s3 {
		suggestions = append(suggestions, withBody(s4, a))
	}

	return suggestions
}

func capMessage(s string) string {
	runes := []rune(s)
	if len(runes) <= 72 {
		return s
	}
	return string(runes[:69]) + "..."
}

func buildPrimary(a analysis) string {
	t := a.commitType
	scope := a.primaryScope
	desc := buildDescription(a)

	var msg string
	if scope != "" {
		msg = fmt.Sprintf("%s(%s): %s", t, scope, desc)
	} else {
		msg = fmt.Sprintf("%s: %s", t, desc)
	}
	return capMessage(msg)
}

func buildAlternate(a analysis) string {
	t := a.commitType
	desc := buildDescriptionAlt(a)
	if desc == "" {
		return ""
	}
	return capMessage(fmt.Sprintf("%s: %s", t, desc))
}

func buildBrief(a analysis) string {
	if a.isDeleteOnly {
		if len(a.filesDeleted) == 1 {
			return fmt.Sprintf("chore: delete %s", cleanFileName(a.filesDeleted[0]))
		}
		return fmt.Sprintf("chore: remove %d unused files", len(a.filesDeleted))
	}

	if a.isNewFiles && len(a.filesModified) == 0 && len(a.filesDeleted) == 0 {
		if len(a.filesAdded) == 1 {
			return fmt.Sprintf("chore: add %s", cleanFileName(a.filesAdded[0]))
		}
		return fmt.Sprintf("chore: add %d new files", len(a.filesAdded))
	}

	if a.isDepUpdate {
		return "chore: update dependencies"
	}
	if a.isMigration {
		return "feat: add database migration"
	}
	if a.isDocsOnly {
		return "docs: update documentation"
	}
	if a.isTestOnly {
		return "test: update test suite"
	}
	if a.isConfigOnly {
		return "chore: update configuration"
	}
	if contains(a.patterns, "remove-debug-logs") {
		return "chore: remove debug logs"
	}

	if len(a.filesModified) == 1 && len(a.filesAdded) == 0 && len(a.filesDeleted) == 0 {
		name := cleanFileName(a.filesModified[0])
		if len(a.addedFuncs) > 0 {
			return fmt.Sprintf("%s: add %s to %s", a.commitType, joinNames(firstN(a.addedFuncs, 1)), name)
		}
		if len(a.removedFuncs) > 0 {
			return fmt.Sprintf("%s: remove %s from %s", a.commitType, joinNames(firstN(a.removedFuncs, 1)), name)
		}
		return fmt.Sprintf("%s: update %s", a.commitType, name)
	}

	n := len(a.filesModified) + len(a.filesAdded) + len(a.filesDeleted)
	if n > 1 && len(a.addedFuncs) == 0 && len(a.removedFuncs) == 0 {
		return fmt.Sprintf("%s: update %d files", a.commitType, n)
	}

	return ""
}

func buildFallback(a analysis) string {
	if a.primaryScope != "" {
		return fmt.Sprintf("%s: update %s", a.commitType, a.primaryScope)
	}
	return fmt.Sprintf("%s: update source files", a.commitType)
}

func buildSummary(a analysis) string {
	var parts []string

	for _, f := range firstN(a.filesAdded, 2) {
		parts = append(parts, "add "+cleanFileName(f))
	}

	if len(a.filesModified) > 0 {
		if len(a.addedFuncs) > 0 {
			funcs := firstN(a.addedFuncs, 2)
			parts = append(parts, "add "+joinNames(funcs))
		} else if len(a.removedFuncs) > 0 {
			funcs := firstN(a.removedFuncs, 2)
			parts = append(parts, "remove "+joinNames(funcs))
		} else {
			for _, f := range firstN(a.filesModified, 2) {
				parts = append(parts, safeUpdateVerb(cleanFileName(f)))
			}
		}
	}

	for _, f := range firstN(a.filesDeleted, 1) {
		parts = append(parts, "remove "+cleanFileName(f))
	}

	if len(parts) == 0 {
		return ""
	}

	seen := map[string]bool{}
	unique := parts[:0]
	for _, p := range parts {
		if !seen[p] {
			seen[p] = true
			unique = append(unique, p)
		}
	}

	msg := fmt.Sprintf("%s: %s", a.commitType, strings.Join(unique, ", "))
	return capMessage(msg)
}

func buildDescription(a analysis) string {
	if len(a.detectedVersions) > 0 {
		latest := a.detectedVersions[len(a.detectedVersions)-1]
		return fmt.Sprintf("update version references to %s", latest)
	}

	if len(a.sqlTables) > 0 && len(a.addedFuncs) == 0 {
		if len(a.sqlTables) == 1 {
			return fmt.Sprintf("add %s table", a.sqlTables[0])
		}
		return fmt.Sprintf("add %s tables", joinNames(firstN(a.sqlTables, 3)))
	}

	if len(a.sqlTables) > 0 && len(a.addedFuncs) > 0 {
		return fmt.Sprintf("add %s and %s table", joinNames(firstN(a.addedFuncs, 2)), a.sqlTables[0])
	}

	if len(a.htmlElements) > 0 {
		elems := firstN(a.htmlElements, 2)
		return fmt.Sprintf("add %s section", joinNames(elems))
	}

	if len(a.cssClasses) > 0 {
		classes := firstN(a.cssClasses, 2)
		return fmt.Sprintf("add %s styles", joinNames(classes))
	}

	if len(a.shellFuncs) > 0 {
		funcs := firstN(a.shellFuncs, 2)
		return fmt.Sprintf("add %s", joinNames(funcs))
	}

	if len(a.addedVars) > 0 && len(a.removedVars) > 0 {
		return fmt.Sprintf("update %s", joinNames(firstN(a.removedVars, 2)))
	}

	if isLargeDiff(a) && len(a.addedFuncs) == 0 && len(a.removedFuncs) == 0 {
		if len(a.filesAdded) > 0 && len(a.filesModified) > 0 {
			return fmt.Sprintf("add and update %s", a.primaryScope)
		}
		if len(a.filesAdded) > 0 {
			return fmt.Sprintf("add %s", a.primaryScope)
		}
		if len(a.filesModified) > 0 {
			return fmt.Sprintf("update %s", a.primaryScope)
		}
	}

	if a.isDeleteOnly {
		if len(a.filesDeleted) == 1 {
			return fmt.Sprintf("remove %s", cleanFileName(a.filesDeleted[0]))
		}
		names := make([]string, 0, len(a.filesDeleted))
		for _, f := range a.filesDeleted {
			names = append(names, cleanFileName(f))
		}
		if len(names) <= 3 {
			return fmt.Sprintf("remove %s", joinNames(names))
		}
		return fmt.Sprintf("remove %d unused files", len(a.filesDeleted))
	}

	if a.isNewFiles && len(a.addedFuncs) == 0 && len(a.filesModified) == 0 {
		if len(a.filesAdded) == 1 {
			name := cleanFileName(a.filesAdded[0])
			dir := filepath.Dir(a.filesAdded[0])
			if dir != "." && dir != "" {
				return fmt.Sprintf("add %s to %s", name, dir)
			}
			return fmt.Sprintf("add %s", name)
		}
		return fmt.Sprintf("add %d new files", len(a.filesAdded))
	}

	if a.isDepUpdate && len(a.addedFuncs) == 0 {
		return "update project dependencies"
	}
	if a.isMigration {
		scope := a.primaryScope
		if scope != "" {
			return fmt.Sprintf("add %s migration", scope)
		}
		return "add database migration"
	}
	if a.isDocsOnly {
		if len(a.filesModified) == 1 {
			return fmt.Sprintf("update %s", cleanFileName(a.filesModified[0]))
		}
		return "update documentation"
	}
	if a.isTestOnly {
		funcs := firstN(a.addedFuncs, 2)
		if len(funcs) > 0 {
			return fmt.Sprintf("add tests for %s", joinNames(funcs))
		}
		if len(a.filesModified) == 1 {
			return fmt.Sprintf("add tests for %s", cleanFileName(a.filesModified[0]))
		}
		return "add test coverage"
	}
	if a.isConfigOnly {
		if len(a.filesModified) == 1 {
			return fmt.Sprintf("update %s config", cleanFileName(a.filesModified[0]))
		}
		return "update configuration"
	}
	if contains(a.patterns, "remove-debug-logs") && len(a.addedFuncs) == 0 {
		return "remove debug logs"
	}

	if len(a.renamedFuncs) > 0 && len(a.addedFuncs) == len(a.renamedFuncs) {
		if len(a.renamedFuncs) == 1 {
			return fmt.Sprintf("rename %s to %s", a.renamedFuncs[0][0], a.renamedFuncs[0][1])
		}
		return fmt.Sprintf("rename %d functions", len(a.renamedFuncs))
	}

	if len(a.addedVars) > 0 && len(a.removedVars) == 0 && len(a.addedFuncs) == 0 {
		vars := firstN(a.addedVars, 2)
		if len(a.filesModified) == 1 {
			return fmt.Sprintf("add %s to %s", joinNames(vars), cleanFileName(a.filesModified[0]))
		}
		return fmt.Sprintf("add %s", joinNames(vars))
	}

	if len(a.removedVars) > 0 && len(a.addedVars) == 0 && len(a.addedFuncs) == 0 && len(a.removedFuncs) == 0 {
		vars := firstN(a.removedVars, 2)
		return fmt.Sprintf("remove %s", joinNames(vars))
	}

	if len(a.addedFuncs) > 0 && len(a.removedFuncs) == 0 {
		if len(a.filesModified) == 1 {
			funcs := firstN(a.addedFuncs, 3)
			return fmt.Sprintf("add %s to %s", joinNames(funcs), cleanFileName(a.filesModified[0]))
		}
		names := make([]string, 0, len(a.filesModified))
		for _, f := range firstN(a.filesModified, 3) {
			names = append(names, cleanFileName(f))
		}
		if len(a.filesModified) > 3 {
			return fmt.Sprintf("add functions to %s and %d more files", joinNames(names), len(a.filesModified)-3)
		}
		return fmt.Sprintf("add functions to %s", joinNames(names))
	}

	if len(a.removedFuncs) > 0 && len(a.addedFuncs) == 0 {
		if len(a.filesModified) == 1 {
			funcs := firstN(a.removedFuncs, 3)
			return fmt.Sprintf("remove %s from %s", joinNames(funcs), cleanFileName(a.filesModified[0]))
		}
		names := make([]string, 0, len(a.filesModified))
		for _, f := range firstN(a.filesModified, 3) {
			names = append(names, cleanFileName(f))
		}
		return fmt.Sprintf("remove functions from %s", joinNames(names))
	}

	if len(a.addedFuncs) > 0 && len(a.removedFuncs) > 0 {
		if len(a.addedFuncs) == 1 && len(a.removedFuncs) == 1 {
			return fmt.Sprintf("replace %s with %s", a.removedFuncs[0], a.addedFuncs[0])
		}
		if len(a.filesModified) == 1 {
			return fmt.Sprintf("refactor %s", cleanFileName(a.filesModified[0]))
		}
		names := make([]string, 0, len(a.filesModified))
		for _, f := range firstN(a.filesModified, 3) {
			names = append(names, cleanFileName(f))
		}
		return fmt.Sprintf("refactor %s", joinNames(names))
	}

	if contains(a.patterns, "error-handling") {
		if len(a.errFuncs) > 0 {
			if len(a.filesModified) == 1 {
				return fmt.Sprintf("handle %s error in %s", a.errFuncs[0], cleanFileName(a.filesModified[0]))
			}
			return fmt.Sprintf("handle %s error", a.errFuncs[0])
		}
		if len(a.filesModified) == 1 {
			return fmt.Sprintf("handle errors in %s", cleanFileName(a.filesModified[0]))
		}
		if a.primaryScope != "" {
			return fmt.Sprintf("handle errors in %s", a.primaryScope)
		}
		return "improve error handling"
	}
	if contains(a.patterns, "tests") {
		if a.primaryScope != "" {
			return fmt.Sprintf("add tests for %s", a.primaryScope)
		}
		return "add test cases"
	}
	if contains(a.patterns, "logging") {
		if a.primaryScope != "" {
			return fmt.Sprintf("add logging to %s", a.primaryScope)
		}
		return "add logging"
	}
	if contains(a.patterns, "comments") {
		return "add inline documentation"
	}

	if a.removedPrints > 0 && len(a.addedFuncs) == 0 && len(a.removedFuncs) == 0 {
		if len(a.filesModified) == 1 {
			return "remove command header from " + cleanFileName(a.filesModified[0])
		}
		return "remove command headers"
	}

	if len(a.filesAdded) > 0 && len(a.filesModified) > 0 {
		if len(a.filesAdded) == 1 {
			return fmt.Sprintf("add %s and update %s", cleanFileName(a.filesAdded[0]), a.primaryScope)
		}
		return fmt.Sprintf("add %d files to %s", len(a.filesAdded), a.primaryScope)
	}

	if len(a.filesAdded) > 0 {
		if len(a.filesAdded) == 1 {
			return fmt.Sprintf("add %s", cleanFileName(a.filesAdded[0]))
		}
		return fmt.Sprintf("add %d new files", len(a.filesAdded))
	}
	if len(a.filesDeleted) > 0 {
		if len(a.filesDeleted) == 1 {
			return fmt.Sprintf("remove %s", cleanFileName(a.filesDeleted[0]))
		}
		return fmt.Sprintf("remove %d files", len(a.filesDeleted))
	}
	if len(a.filesModified) == 1 {
		return safeUpdateVerb(cleanFileName(a.filesModified[0]))
	}

	return safeUpdateVerb(a.primaryScope)
}

func buildDescriptionAlt(a analysis) string {
	if a.isDeleteOnly {
		if len(a.filesDeleted) == 1 {
			return fmt.Sprintf("clean up %s", cleanFileName(a.filesDeleted[0]))
		}
		return "clean up deleted files"
	}
	if a.isTestOnly {
		return fmt.Sprintf("improve test coverage in %s", a.primaryScope)
	}
	if a.isDocsOnly {
		return fmt.Sprintf("improve %s documentation", a.primaryScope)
	}
	if a.isMigration {
		return fmt.Sprintf("migrate %s schema", a.primaryScope)
	}

	if len(a.addedFuncs) > 0 && len(a.removedFuncs) == 0 {
		if len(a.filesModified) == 1 {
			funcs := firstN(a.addedFuncs, 2)
			if a.primaryScope != "" {
				return fmt.Sprintf("implement %s in %s", joinNames(funcs), a.primaryScope)
			}
			return fmt.Sprintf("implement %s", joinNames(funcs))
		}
		names := make([]string, 0, len(a.filesModified))
		for _, f := range firstN(a.filesModified, 3) {
			names = append(names, cleanFileName(f))
		}
		return fmt.Sprintf("implement %s", joinNames(names))
	}

	if len(a.removedFuncs) > 0 && len(a.addedFuncs) == 0 {
		if len(a.filesModified) == 1 {
			funcs := firstN(a.removedFuncs, 2)
			return fmt.Sprintf("clean up %s", joinNames(funcs))
		}
		return fmt.Sprintf("clean up %s", a.primaryScope)
	}

	if contains(a.patterns, "error-handling") {
		if len(a.errFuncs) > 0 {
			if len(a.filesModified) == 1 {
				return fmt.Sprintf("add error handling for %s in %s", a.errFuncs[0], cleanFileName(a.filesModified[0]))
			}
			return fmt.Sprintf("add error handling for %s", a.errFuncs[0])
		}
		if len(a.filesModified) == 1 {
			return fmt.Sprintf("add error handling for %s", cleanFileName(a.filesModified[0]))
		}
		return fmt.Sprintf("add error handling for %s", a.primaryScope)
	}
	if a.removedPrints > 0 && len(a.addedFuncs) == 0 {
		return "clean up startup output"
	}
	if len(a.filesModified) > 1 {
		return fmt.Sprintf("refactor %s module", a.primaryScope)
	}
	return ""
}

func cleanFileName(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	name = strings.TrimSuffix(name, "_test")
	name = strings.TrimSuffix(name, ".test")
	name = strings.TrimSuffix(name, ".spec")
	return name
}

func joinNames(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	default:
		return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
	}
}

func generateBranchNameSuggestions(a analysis) []string {
	var suggestions []string
	seen := map[string]bool{}

	add := func(s string) {
		s = sanitizeBranchName(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		suggestions = append(suggestions, s)
	}

	prefix := "feat"
	if a.commitType != "" {
		prefix = a.commitType
	}

	if len(a.addedFuncs) > 0 {
		add(prefix + "/" + toKebab(a.addedFuncs[0]))
	}
	if len(a.renamedFuncs) > 0 {
		add("refactor/" + toKebab(a.renamedFuncs[0][1]))
	}
	if len(a.filesAdded) > 0 {
		name := cleanFileName(a.filesAdded[0])
		add(prefix + "/" + toKebab(name))
	}
	if a.primaryScope != "" {
		add(prefix + "/" + toKebab(a.primaryScope))
	}
	if len(a.filesModified) > 0 {
		name := cleanFileName(a.filesModified[0])
		add(prefix + "/" + toKebab(name))
	}

	for len(suggestions) < 3 {
		add(prefix + "/update-" + toKebab(a.primaryScope))
		if len(suggestions) < 3 {
			break
		}
	}

	if len(suggestions) > 3 {
		suggestions = suggestions[:3]
	}
	return suggestions
}

func toKebab(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prev := '-'
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
			prev = c
		} else if c == '-' || c == '_' || c == ' ' || c == '/' {
			if prev != '-' {
				b.WriteRune('-')
				prev = '-'
			}
		}
	}
	result := strings.Trim(b.String(), "-")
	if len(result) > 40 {
		result = result[:40]
	}
	return result
}

func sanitizeBranchName(s string) string {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) == 2 {
		slug := toKebab(parts[1])
		if slug == "" {
			return ""
		}
		return parts[0] + "/" + slug
	}
	return toKebab(s)
}
