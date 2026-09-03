package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupMockRunners(t *testing.T) (func(), *[]string) {
	t.Helper()
	origRunner := execRunner
	origOutputRunner := execOutputRunner

	var executed []string

	execRunner = func(name string, dir string, stdin string, args ...string) error {
		full := fmt.Sprintf("%s %s %s", name, strings.Join(args, " "), stdin)
		executed = append(executed, strings.TrimSpace(full))
		return nil
	}

	execOutputRunner = func(name string, dir string, args ...string) ([]byte, error) {
		if name == "docker" && len(args) > 0 && args[0] == "info" {
			return []byte("Client: Docker Engine"), nil
		}
		if strings.HasSuffix(name, "sail") && len(args) > 0 && args[0] == "ps" {
			return []byte("running\nrunning"), nil
		}
		return []byte(""), nil
	}

	cleanup := func() {
		execRunner = origRunner
		execOutputRunner = origOutputRunner
	}

	return cleanup, &executed
}

func TestDetectPHPVersion(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "php-detect-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		filename string
		content  string
		expected string
	}{
		{
			filename: "compose.yaml",
			content:  "services:\n  laravel.test:\n    build:\n      context: './vendor/laravel/sail/runtimes/8.3'",
			expected: "83",
		},
		{
			filename: "compose.yml",
			content:  "services:\n  laravel.test:\n    build:\n      context: './vendor/laravel/sail/runtimes/8.3'",
			expected: "83",
		},
		{
			filename: "docker-compose.yaml",
			content:  "services:\n  app:\n    image: 'sail-8.4/app'",
			expected: "84",
		},
		{
			filename: "docker-compose.yml",
			content:  "services:\n  app:\n    image: 'sail-8.4/app'",
			expected: "84",
		},
		{
			filename: "compose.yaml",
			content:  "services:\n  laravel.test:\n    build:\n      context: ./docker/8.1",
			expected: "81",
		},
		{
			filename: "compose.yaml",
			content:  "services:\n  app:\n    image: some-other-image",
			expected: "",
		},
	}

	for _, tt := range tests {
		// Clean up all possible files from previous test
		os.Remove(filepath.Join(tempDir, "compose.yaml"))
		os.Remove(filepath.Join(tempDir, "compose.yml"))
		os.Remove(filepath.Join(tempDir, "docker-compose.yaml"))
		os.Remove(filepath.Join(tempDir, "docker-compose.yml"))

		err := os.WriteFile(filepath.Join(tempDir, tt.filename), []byte(tt.content), 0644)
		if err != nil {
			t.Fatal(err)
		}

		got := detectPHPVersion(tempDir)
		if got != tt.expected {
			t.Errorf("For %s content %q, expected %q, got %q", tt.filename, tt.content, tt.expected, got)
		}
	}
}

func TestRunSailInitSkipsWhenSailExists(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sail-init-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	cleanup, _ := setupMockRunners(t)
	defer cleanup()

	// Create vendor/bin/sail to simulate existing installation
	sailDir := filepath.Join(tempDir, "vendor", "bin")
	if err := os.MkdirAll(sailDir, 0755); err != nil {
		t.Fatal(err)
	}
	sailPath := filepath.Join(sailDir, "sail")
	if err := os.WriteFile(sailPath, []byte("#!/bin/bash\necho sail"), 0755); err != nil {
		t.Fatal(err)
	}

	// With forceInstall=false, should skip and return nil
	err = runSailInit("84", tempDir, false)
	if err != nil {
		t.Errorf("Expected nil error when sail exists and forceInstall=false, got: %v", err)
	}
}

func TestRunSailInitRunsWithFreshFlag(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sail-init-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	cleanup, executed := setupMockRunners(t)
	defer cleanup()

	// Create vendor/bin/sail to simulate existing installation
	sailDir := filepath.Join(tempDir, "vendor", "bin")
	if err := os.MkdirAll(sailDir, 0755); err != nil {
		t.Fatal(err)
	}
	sailPath := filepath.Join(sailDir, "sail")
	if err := os.WriteFile(sailPath, []byte("#!/bin/bash\necho sail"), 0755); err != nil {
		t.Fatal(err)
	}

	// With forceInstall=true, should run mocked runner
	err = runSailInit("84", tempDir, true)
	if err != nil {
		t.Fatalf("Expected nil error with mock runner, got: %v", err)
	}

	if len(*executed) == 0 {
		t.Error("Expected docker command to be executed")
	}
}

func TestVersionDefault(t *testing.T) {
	if version != "dev" {
		t.Errorf("Expected default version %q, got %q", "dev", version)
	}
}

func TestRunSailStopNoSail(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sail-stop-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	cleanup, _ := setupMockRunners(t)
	defer cleanup()

	err = runSailStop(tempDir)
	if err == nil {
		t.Error("Expected error when sail binary doesn't exist")
	}
	if err != nil && !strings.Contains(err.Error(), "sail binary not found") {
		t.Errorf("Expected 'sail binary not found' error, got: %v", err)
	}
}

func TestRunSailDownNoSail(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sail-down-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	cleanup, _ := setupMockRunners(t)
	defer cleanup()

	err = runSailDown(tempDir)
	if err == nil {
		t.Error("Expected error when sail binary doesn't exist")
	}
	if err != nil && !strings.Contains(err.Error(), "sail binary not found") {
		t.Errorf("Expected 'sail binary not found' error, got: %v", err)
	}
}

func TestRunSailUpNoSail(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sail-up-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	cleanup, _ := setupMockRunners(t)
	defer cleanup()

	err = runSailUp(tempDir)
	if err == nil {
		t.Error("Expected error when sail binary doesn't exist")
	}
	if err != nil && !strings.Contains(err.Error(), "sail binary not found") {
		t.Errorf("Expected 'sail binary not found' error, got: %v", err)
	}
}

func TestGetContainerStatusNoSail(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sail-status-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	status := getContainerStatus(tempDir)
	if status != "no sail" {
		t.Errorf("Expected %q, got %q", "no sail", status)
	}
}

func TestCreateNewProjectWithServices(t *testing.T) {
	cleanup, executed := setupMockRunners(t)
	defer cleanup()

	tempDir, err := os.MkdirTemp("", "create-new-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	target := filepath.Join(tempDir, "my-app")

	err = createNewProject(target, "mysql,redis,mailpit")
	if err != nil {
		t.Fatalf("createNewProject failed: %v", err)
	}

	if len(*executed) != 1 {
		t.Fatalf("Expected 1 command executed, got %d", len(*executed))
	}

	cmdStr := (*executed)[0]
	if !strings.Contains(cmdStr, "https://laravel.build/") || !strings.Contains(cmdStr, "with=mysql,redis,mailpit") {
		t.Errorf("Expected command to contain custom services URL, got: %s", cmdStr)
	}
}

func TestCheckDockerDaemon(t *testing.T) {
	cleanup, _ := setupMockRunners(t)
	defer cleanup()

	if err := checkDockerDaemon(); err != nil {
		t.Errorf("Expected checkDockerDaemon to pass with mock runner, got %v", err)
	}
}

func TestGenerateCompletion(t *testing.T) {
	shells := []string{"bash", "zsh", "fish"}
	for _, s := range shells {
		generateCompletion(s)
	}
}
