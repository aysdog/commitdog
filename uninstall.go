package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func runUninstall() {
	fmt.Println()
	fmt.Println("  ⚠  this will remove commitdog and all its config from your device.")
	fmt.Println()
	fmt.Printf("  are you sure? [y/N] › ")

	input := strings.ToLower(strings.TrimSpace(readLine()))
	if input != "y" && input != "yes" {
		fmt.Println("  aborted.")
		fmt.Println()
		return
	}

	fmt.Println()

	binaryPath, err := binaryLocation()
	if err != nil {
		fmt.Printf("  could not find commitdog binary: %v\n", err)
	} else {
		fmt.Printf("  removing binary: %s\n", binaryPath)
		if err := removeWithElevation(binaryPath); err != nil {
			fmt.Printf("  ✗ could not remove binary: %v\n", err)
			fmt.Printf("    run manually: sudo rm %s\n", binaryPath)
		} else {
			fmt.Printf("  ✓ binary removed\n")
		}
	}

	configDir, err := configDirectory()
	if err == nil && configDir != "" {
		fmt.Printf("  removing config: %s\n", configDir)
		if err := os.RemoveAll(configDir); err != nil {
			fmt.Printf("  ✗ could not remove config: %v\n", err)
			fmt.Printf("    run manually: rm -rf %s\n", configDir)
		} else {
			fmt.Printf("  ✓ config removed\n")
		}
	}

	fmt.Println()
	fmt.Println("  ✓ commitdog uninstalled.")
	fmt.Println()
}

func binaryLocation() (string, error) {
	path, err := exec.LookPath("commitdog")
	if err != nil {
		self, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("could not locate binary")
		}
		return self, nil
	}
	return path, nil
}

func configDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "windows":
		appdata := os.Getenv("APPDATA")
		if appdata != "" {
			return filepath.Join(appdata, "commitdog"), nil
		}
		return filepath.Join(home, "AppData", "Roaming", "commitdog"), nil
	default:
		return filepath.Join(home, ".config", "commitdog"), nil
	}
}

func removeWithElevation(path string) error {
	if err := os.Remove(path); err == nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		return fmt.Errorf("delete %s manually", path)
	}
	cmd := exec.Command("sudo", "rm", path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
