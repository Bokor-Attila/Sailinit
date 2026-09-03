package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// browserOpener returns the command used to open a URL on this platform.
// It is a var so tests can assert the platform mapping without launching a browser.
var browserOpener = func() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return "open", nil
	case "linux":
		return "xdg-open", nil
	default:
		return "", fmt.Errorf("opening a browser is not supported on %s", runtime.GOOS)
	}
}

// containerRunning reports whether getContainerStatus described a live project.
// The status string may carry ANSI colors, so it is matched by substring.
func containerRunning(status string) bool {
	return strings.Contains(status, "running")
}

// projectURL returns the local URL for a registered project.
func projectURL(projectDir string) (string, error) {
	suffix, known, _, err := getSuggestedSuffix(projectDir)
	if err != nil {
		return "", err
	}
	if !known {
		return "", fmt.Errorf("this project is not registered yet; run sailinit here first")
	}

	ports := CalculatePorts(suffix)
	return fmt.Sprintf("http://localhost:%d", ports["APP_PORT"]), nil
}

// runOpen opens the current project's application URL in the default browser.
func runOpen(projectDir string, dryRun bool) error {
	url, err := projectURL(projectDir)
	if err != nil {
		return err
	}

	if dryRun {
		printInfo(fmt.Sprintf("[dry-run] Would open %s", url))
		return nil
	}

	// A stopped project still gets a tab, so it is ready as sail up finishes.
	if _, err := os.Stat(filepath.Join(projectDir, "vendor", "bin", "sail")); err == nil {
		if status := getContainerStatus(projectDir); !containerRunning(status) {
			printWarning("Containers are not running; start them with sail up -d.")
		}
	}

	opener, err := browserOpener()
	if err != nil {
		// No opener is not fatal: printing the URL still gets the user there.
		printWarning(err.Error())
		fmt.Println(url)
		return nil
	}

	printInfo(fmt.Sprintf("Opening %s", url))
	if err := execRunner(opener, projectDir, "", url); err != nil {
		// xdg-open is missing on minimal Linux installs; fall back to printing.
		printWarning(fmt.Sprintf("Could not launch %s: %v", opener, err))
		fmt.Println(url)
	}
	return nil
}
