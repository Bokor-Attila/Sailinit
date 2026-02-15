package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

	// Create vendor/bin/sail to simulate existing installation
	sailDir := filepath.Join(tempDir, "vendor", "bin")
	if err := os.MkdirAll(sailDir, 0755); err != nil {
		t.Fatal(err)
	}
	sailPath := filepath.Join(sailDir, "sail")
	if err := os.WriteFile(sailPath, []byte("#!/bin/bash\necho sail"), 0755); err != nil {
		t.Fatal(err)
	}

	// With forceInstall=true, should attempt to run docker (which will fail in test env)
	err = runSailInit("84", tempDir, true)

	// We expect an error because docker won't run properly in tests,
	// but the important thing is that it TRIED to run (didn't skip)
	if err == nil {
		t.Error("Expected error when running docker in test environment with forceInstall=true")
	}

	// Verify it's a docker-related error (tried to run) not an early return
	if err != nil && !strings.Contains(err.Error(), "exit status") && !strings.Contains(err.Error(), "executable file not found") {
		t.Errorf("Expected docker execution error, got: %v", err)
	}
}
