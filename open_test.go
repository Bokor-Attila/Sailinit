package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withOpener swaps the platform browser opener for one test.
func withOpener(t *testing.T, name string, err error) {
	t.Helper()
	orig := browserOpener
	browserOpener = func() (string, error) { return name, err }
	t.Cleanup(func() { browserOpener = orig })
}

func TestProjectURLUsesRegisteredSuffix(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()

	proj := makeLaravelProject(t, filepath.Join(tempDir, "blog"), 8051)
	state := &PortState{MaxSuffix: 51, Projects: map[string]int{proj: 51}}
	if err := state.save(); err != nil {
		t.Fatal(err)
	}

	url, err := projectURL(proj)
	if err != nil {
		t.Fatalf("projectURL: %v", err)
	}
	if url != "http://localhost:8051" {
		t.Errorf("url = %q, want http://localhost:8051", url)
	}
}

func TestProjectURLRejectsUnregisteredProject(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()

	// A directory with no .env and no registry row has no port to open.
	plain := filepath.Join(tempDir, "unknown")
	if err := os.MkdirAll(plain, 0755); err != nil {
		t.Fatal(err)
	}
	state := &PortState{MaxSuffix: 51, Projects: map[string]int{}}
	if err := state.save(); err != nil {
		t.Fatal(err)
	}

	if _, err := projectURL(plain); err == nil {
		t.Fatal("expected an unregistered project to error, got nil")
	}
}

func TestRunOpenLaunchesOpener(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()
	restore, executed := setupMockRunners(t)
	defer restore()
	withOpener(t, "open", nil)

	proj := makeLaravelProject(t, filepath.Join(tempDir, "blog"), 8051)
	state := &PortState{MaxSuffix: 51, Projects: map[string]int{proj: 51}}
	if err := state.save(); err != nil {
		t.Fatal(err)
	}

	if err := runOpen(proj, false); err != nil {
		t.Fatalf("runOpen: %v", err)
	}

	joined := strings.Join(*executed, "\n")
	if !strings.Contains(joined, "open http://localhost:8051") {
		t.Errorf("expected the opener to be invoked with the project URL, got: %q", joined)
	}
}

func TestRunOpenDryRunLaunchesNothing(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()
	restore, executed := setupMockRunners(t)
	defer restore()
	withOpener(t, "open", nil)

	proj := makeLaravelProject(t, filepath.Join(tempDir, "blog"), 8051)
	state := &PortState{MaxSuffix: 51, Projects: map[string]int{proj: 51}}
	if err := state.save(); err != nil {
		t.Fatal(err)
	}

	if err := runOpen(proj, true); err != nil {
		t.Fatalf("runOpen dry-run: %v", err)
	}
	if len(*executed) != 0 {
		t.Errorf("dry-run must not run anything, but ran: %v", *executed)
	}
}

func TestRunOpenFallsBackWhenOpenerMissing(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()
	restore, executed := setupMockRunners(t)
	defer restore()
	// Unsupported platform: no opener available.
	withOpener(t, "", os.ErrNotExist)

	proj := makeLaravelProject(t, filepath.Join(tempDir, "blog"), 8051)
	state := &PortState{MaxSuffix: 51, Projects: map[string]int{proj: 51}}
	if err := state.save(); err != nil {
		t.Fatal(err)
	}

	// Printing the URL is a valid outcome; erroring out is not.
	if err := runOpen(proj, false); err != nil {
		t.Fatalf("missing opener should not be fatal, got: %v", err)
	}
	if len(*executed) != 0 {
		t.Errorf("no opener should have been launched, but ran: %v", *executed)
	}
}

func TestContainerRunning(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{"3 running", true},
		{colorize(colorGreen, "2 running"), true},
		{"stopped", false},
		{colorize(colorDim, "stopped"), false},
		{"no sail", false},
		{"unknown", false},
	}

	for _, tc := range tests {
		if got := containerRunning(tc.status); got != tc.want {
			t.Errorf("containerRunning(%q) = %v, want %v", tc.status, got, tc.want)
		}
	}
}

func TestBrowserOpenerMapsPlatform(t *testing.T) {
	name, err := browserOpener()
	if err != nil {
		t.Skipf("no opener on this platform: %v", err)
	}
	if name != "open" && name != "xdg-open" {
		t.Errorf("opener = %q, want open or xdg-open", name)
	}
}
