package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

func generateSuggestions(a analysis) []string {
	var suggestions []string

	s1 := buildPrimary(a)
	s2 := buildAlternate(a)
	s3 := buildBrief(a)

	suggestions = append(suggestions, s1)
	if s2 != "" && s2 != s1 {
		suggestions = append(suggestions, s2)
	}
	if s3 != "" && s3 != s1 && s3 != s2 {
		suggestions = append(suggestions, s3)
	}

	if len(suggestions) < 2 {
		suggestions = append(suggestions, buildFallback(a))
	}

	return suggestions
}

func buildPrimary(a analysis) string {
	t := a.commitType
	scope := a.primaryScope
	desc := buildDescription(a)

	if scope != "" {
		return fmt.Sprintf("%s(%s): %s", t, scope, desc)
	}
	return fmt.Sprintf("%s: %s", t, desc)
}

func buildAlternate(a analysis) string {
	t := a.commitType
	desc := buildDescriptionAlt(a)
	if desc == "" {
		return ""
	}
	return fmt.Sprintf("%s: %s", t, desc)
}

func buildBrief(a analysis) string {
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

	if len(a.filesModified) == 1 {
		name := cleanFileName(a.filesModified[0])
		return fmt.Sprintf("%s: update %s", a.commitType, name)
	}

	n := len(a.filesModified) + len(a.filesAdded)
	if n > 1 {
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

func buildDescription(a analysis) string {
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

	if len(a.addedFuncs) > 0 && len(a.removedFuncs) == 0 {
		funcs := firstN(a.addedFuncs, 2)
		return fmt.Sprintf("add %s", joinNames(funcs))
	}
	if len(a.removedFuncs) > 0 && len(a.addedFuncs) == 0 {
		funcs := firstN(a.removedFuncs, 2)
		return fmt.Sprintf("remove %s", joinNames(funcs))
	}
	if len(a.addedFuncs) > 0 && len(a.removedFuncs) > 0 {
		return fmt.Sprintf("replace %s with %s",
			joinNames(firstN(a.removedFuncs, 1)),
			joinNames(firstN(a.addedFuncs, 1)),
		)
	}

	if contains(a.patterns, "error-handling") {
		scope := a.primaryScope
		if scope != "" {
			return fmt.Sprintf("handle errors in %s", scope)
		}
		return "improve error handling"
	}
	if contains(a.patterns, "tests") {
		return "add test cases"
	}
	if contains(a.patterns, "logging") {
		return "add logging"
	}
	if contains(a.patterns, "comments") {
		return "add inline documentation"
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
		return fmt.Sprintf("update %s", cleanFileName(a.filesModified[0]))
	}

	return fmt.Sprintf("update %s", a.primaryScope)
}

func buildDescriptionAlt(a analysis) string {
	if a.isTestOnly {
		return fmt.Sprintf("improve test coverage in %s", a.primaryScope)
	}
	if a.isDocsOnly {
		return fmt.Sprintf("improve %s documentation", a.primaryScope)
	}
	if a.isMigration {
		return fmt.Sprintf("migrate %s schema", a.primaryScope)
	}
	if len(a.addedFuncs) > 0 {
		funcs := firstN(a.addedFuncs, 1)
		if a.primaryScope != "" {
			return fmt.Sprintf("implement %s in %s", joinNames(funcs), a.primaryScope)
		}
		return fmt.Sprintf("implement %s", joinNames(funcs))
	}
	if contains(a.patterns, "error-handling") {
		return fmt.Sprintf("add error handling for %s", a.primaryScope)
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
