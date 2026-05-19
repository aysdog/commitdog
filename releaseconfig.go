package main

import (
	"fmt"
	"os"
	"strings"
)

type projectConfig struct {
	configured bool
	platform   string
	primary    string
	mirrors    []string
	targets    []string
}

func (p projectConfig) effectivePrimary() string {
	if p.primary != "" {
		return p.primary
	}
	return p.platform
}

type buildTarget struct {
	goos   string
	goarch string
	suffix string
}

var allBuildTargets = []buildTarget{
	{"linux", "amd64", ""},
	{"linux", "arm64", ""},
	{"darwin", "amd64", ""},
	{"darwin", "arm64", ""},
	{"windows", "amd64", ".exe"},
}

func targetKey(t buildTarget) string {
	return t.goos + "/" + t.goarch
}

func defaultTargetKeys() []string {
	keys := make([]string, len(allBuildTargets))
	for i, t := range allBuildTargets {
		keys[i] = targetKey(t)
	}
	return keys
}

func loadProjectConfig() projectConfig {
	data, err := os.ReadFile(".commitdog")
	if err != nil {
		return projectConfig{}
	}

	cfg := projectConfig{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		key, val, ok := parseKV(line)
		if !ok {
			continue
		}
		switch key {
		case "configured":
			cfg.configured = val == "true"
		case "platform":
			cfg.platform = val
		case "primary":
			cfg.primary = val
		case "mirrors":
			inner := strings.TrimPrefix(val, "[")
			inner = strings.TrimSuffix(inner, "]")
			for _, part := range strings.Split(inner, ",") {
				m := strings.Trim(strings.TrimSpace(part), "\"")
				if m != "" {
					cfg.mirrors = append(cfg.mirrors, m)
				}
			}
		case "targets":
			inner := strings.TrimPrefix(val, "[")
			inner = strings.TrimSuffix(inner, "]")
			for _, part := range strings.Split(inner, ",") {
				t := strings.Trim(strings.TrimSpace(part), "\"")
				if t != "" {
					cfg.targets = append(cfg.targets, t)
				}
			}
		}
	}
	return cfg
}

func saveProjectConfig(cfg projectConfig) error {
	quoted := make([]string, len(cfg.targets))
	for i, t := range cfg.targets {
		quoted[i] = "\"" + t + "\""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("configured = %v\n", cfg.configured))
	primary := cfg.effectivePrimary()
	if primary != "" {
		sb.WriteString(fmt.Sprintf("primary = \"%s\"\n", primary))
	}
	var cleanMirrors []string
	for _, m := range cfg.mirrors {
		if m != primary {
			cleanMirrors = append(cleanMirrors, m)
		}
	}
	if len(cleanMirrors) > 0 {
		mirrorQuoted := make([]string, len(cleanMirrors))
		for i, m := range cleanMirrors {
			mirrorQuoted[i] = "\"" + m + "\""
		}
		sb.WriteString(fmt.Sprintf("mirrors = [%s]\n", strings.Join(mirrorQuoted, ", ")))
	}
	if len(cfg.targets) > 0 {
		sb.WriteString(fmt.Sprintf("targets = [%s]\n", strings.Join(quoted, ", ")))
	}

	return os.WriteFile(".commitdog", []byte(sb.String()), 0644)
}

func runReleaseConfig() {
	cfg := loadProjectConfig()

	selected := map[string]bool{}
	for _, t := range cfg.targets {
		selected[t] = true
	}

	fmt.Println()
	fmt.Println("  select build targets:")
	fmt.Println()
	fmt.Println("  enter numbers separated by spaces, e.g. 1, 2, 3")

	for {
		fmt.Println()
		for i, t := range allBuildTargets {
			mark := " "
			if selected[targetKey(t)] {
				mark = "✓"
			}
			fmt.Printf("  %d  %-18s %s\n", i+1, targetKey(t), mark)
		}

		fmt.Println()
		fmt.Printf("  [1-%d] toggle, [a] all, [enter] confirm, [q] quit › ", len(allBuildTargets))

		input := strings.TrimSpace(readLine())

		if input == "" {
			break
		}
		if input == "q" {
			fmt.Println("  aborted.")
			return
		}
		if input == "a" {
			allOn := true
			for _, t := range allBuildTargets {
				if !selected[targetKey(t)] {
					allOn = false
					break
				}
			}
			for _, t := range allBuildTargets {
				selected[targetKey(t)] = !allOn
			}
			continue
		}

		for _, token := range strings.Fields(input) {
			token = strings.Trim(token, ",")
			for i, t := range allBuildTargets {
				if token == fmt.Sprintf("%d", i+1) {
					key := targetKey(t)
					selected[key] = !selected[key]
					break
				}
			}
		}
	}

	var chosen []string
	for _, t := range allBuildTargets {
		if selected[targetKey(t)] {
			chosen = append(chosen, targetKey(t))
		}
	}

	if len(chosen) == 0 {
		fmt.Println("  no targets selected — aborted.")
		return
	}

	cfg.configured = true
	cfg.targets = chosen

	if err := saveProjectConfig(cfg); err != nil {
		fatal("could not save .commitdog: %v", err)
	}

	fmt.Println()
	fmt.Println("  ✓ saved to .commitdog")
	fmt.Println()
}

func ensureReleaseConfig() []buildTarget {
	cfg := loadProjectConfig()

	if cfg.configured {
		return resolveTargets(cfg.targets)
	}

	fmt.Println()
	fmt.Println("  no release targets configured.")
	fmt.Println()
	fmt.Println("  1  configure now")
	fmt.Println("  2  use defaults and don't ask again")
	fmt.Println()
	fmt.Printf("  [1/2/q] pick › ")

	for {
		switch strings.TrimSpace(readLine()) {
		case "1":
			runReleaseConfig()
			cfg = loadProjectConfig()
			if !cfg.configured {
				os.Exit(0)
			}
			return resolveTargets(cfg.targets)
		case "2":
			cfg.configured = true
			cfg.targets = defaultTargetKeys()
			if err := saveProjectConfig(cfg); err != nil {
				fatal("could not save .commitdog: %v", err)
			}
			return allBuildTargets
		case "q", "":
			fmt.Println("  aborted.")
			os.Exit(0)
		default:
			fmt.Printf("  1, 2, or q › ")
		}
	}
}

func resolveTargets(keys []string) []buildTarget {
	keySet := map[string]bool{}
	for _, k := range keys {
		keySet[k] = true
	}
	var result []buildTarget
	for _, t := range allBuildTargets {
		if keySet[targetKey(t)] {
			result = append(result, t)
		}
	}
	return result
}
