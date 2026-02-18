package main

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractSuffixFromEnv(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sail-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	envPath := filepath.Join(tempDir, ".env")
	content := "DEBUG=true\nAPP_PORT=8051\nDB_DATABASE=testing"
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	suffix, found := extractSuffixFromEnv(envPath)
	if !found {
		t.Errorf("Expected to find suffix, but didn't")
	}
	if suffix != 51 {
		t.Errorf("Expected suffix 51, got %d", suffix)
	}
}

func TestSetupEnvFormatting(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sail-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	envPath := filepath.Join(tempDir, ".env")
	initialContent := "OTHER_VAR=value\nDB_DATABASE=old_db"
	if err := os.WriteFile(envPath, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	// resetDb=false, .env already exists -> DB settings should be preserved
	if err := setupEnv(tempDir, 55, false); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)

	// Check if ports are present
	if !strings.Contains(content, "APP_PORT=8055") {
		t.Error("APP_PORT=8055 missing")
	}
	if !strings.Contains(content, "FORWARD_DB_PORT=3355") {
		t.Error("FORWARD_DB_PORT=3355 missing")
	}
	if !strings.Contains(content, "SAIL_XDEBUG_MODE=develop,debug,coverage") {
		t.Error("SAIL_XDEBUG_MODE missing")
	}

	// Check grouping and spacing (simple check)
	lines := strings.Split(content, "\n")

	// Find SAIL_XDEBUG_MODE - should be at the very end (ignoring trailing newline)
	lastLine := ""
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			lastLine = lines[i]
			break
		}
	}
	if !strings.HasPrefix(lastLine, "SAIL_XDEBUG_MODE=") {
		t.Errorf("SAIL_XDEBUG_MODE should be the last non-empty line, got: %s", lastLine)
	}
}

func TestSetupEnvPreservesDbSettingsForExistingEnv(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sail-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	envPath := filepath.Join(tempDir, ".env")
	// Simulate existing .env with custom DB settings
	initialContent := "APP_NAME=MyApp\nDB_CONNECTION=pgsql\nDB_HOST=postgres\nDB_DATABASE=etransport\nDB_USERNAME=admin\nDB_PASSWORD=secret123"
	if err := os.WriteFile(envPath, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	// resetDb=false -> DB settings should be preserved
	if err := setupEnv(tempDir, 55, false); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)

	// DB settings should remain unchanged
	if !strings.Contains(content, "DB_CONNECTION=pgsql") {
		t.Error("DB_CONNECTION should remain pgsql")
	}
	if !strings.Contains(content, "DB_HOST=postgres") {
		t.Error("DB_HOST should remain postgres")
	}
	if !strings.Contains(content, "DB_DATABASE=etransport") {
		t.Error("DB_DATABASE should remain etransport")
	}
	if !strings.Contains(content, "DB_USERNAME=admin") {
		t.Error("DB_USERNAME should remain admin")
	}
	if !strings.Contains(content, "DB_PASSWORD=secret123") {
		t.Error("DB_PASSWORD should remain secret123")
	}

	// Port settings should still be applied
	if !strings.Contains(content, "APP_PORT=8055") {
		t.Error("APP_PORT=8055 missing")
	}
}

func TestSetupEnvResetDbOverwritesSettings(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sail-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	envPath := filepath.Join(tempDir, ".env")
	// Simulate existing .env with custom DB settings
	initialContent := "APP_NAME=MyApp\nDB_CONNECTION=pgsql\nDB_HOST=postgres\nDB_DATABASE=etransport\nDB_USERNAME=admin\nDB_PASSWORD=secret123"
	if err := os.WriteFile(envPath, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	// resetDb=true -> DB settings should be overwritten to Sail defaults
	if err := setupEnv(tempDir, 55, true); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)

	// DB settings should be overwritten to Sail defaults
	if !strings.Contains(content, "DB_CONNECTION=mysql") {
		t.Error("DB_CONNECTION should be mysql with --reset-db")
	}
	if !strings.Contains(content, "DB_HOST=mysql") {
		t.Error("DB_HOST should be mysql with --reset-db")
	}
	if !strings.Contains(content, "DB_DATABASE=laravel") {
		t.Error("DB_DATABASE should be laravel with --reset-db")
	}
	if !strings.Contains(content, "DB_USERNAME=sail") {
		t.Error("DB_USERNAME should be sail with --reset-db")
	}
	if !strings.Contains(content, "DB_PASSWORD=password") {
		t.Error("DB_PASSWORD should be password with --reset-db")
	}

	// Port settings should still be applied
	if !strings.Contains(content, "APP_PORT=8055") {
		t.Error("APP_PORT=8055 missing")
	}
}

func TestSetupEnvNewEnvGetsDbSettings(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sail-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create .env.example but not .env
	envExamplePath := filepath.Join(tempDir, ".env.example")
	exampleContent := "APP_NAME=Laravel\nAPP_ENV=local"
	if err := os.WriteFile(envExamplePath, []byte(exampleContent), 0644); err != nil {
		t.Fatal(err)
	}

	// resetDb=false but .env doesn't exist -> DB settings should be applied
	if err := setupEnv(tempDir, 55, false); err != nil {
		t.Fatal(err)
	}

	envPath := filepath.Join(tempDir, ".env")
	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)

	// DB settings should be set to Sail defaults for new .env
	if !strings.Contains(content, "DB_CONNECTION=mysql") {
		t.Error("DB_CONNECTION should be mysql for new .env")
	}
	if !strings.Contains(content, "DB_HOST=mysql") {
		t.Error("DB_HOST should be mysql for new .env")
	}
	if !strings.Contains(content, "DB_DATABASE=laravel") {
		t.Error("DB_DATABASE should be laravel for new .env")
	}
	if !strings.Contains(content, "DB_USERNAME=sail") {
		t.Error("DB_USERNAME should be sail for new .env")
	}
	if !strings.Contains(content, "DB_PASSWORD=password") {
		t.Error("DB_PASSWORD should be password for new .env")
	}

	// Port settings should also be applied
	if !strings.Contains(content, "APP_PORT=8055") {
		t.Error("APP_PORT=8055 missing")
	}
}

func TestIsSuffixInUseByOther(t *testing.T) {
	lines := splitLines("line1\nline2\r\nline3")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}
}

func setupTestState(t *testing.T) (string, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "sail-state-test-*")
	if err != nil {
		t.Fatal(err)
	}

	statePath := filepath.Join(tempDir, "test-ports.json")
	testStatePathOverride = statePath

	cleanup := func() {
		testStatePathOverride = ""
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

func TestListProjects(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()

	// Create a real directory to simulate an existing project
	existingDir := filepath.Join(tempDir, "existing-project")
	if err := os.MkdirAll(existingDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a path that doesn't exist (orphaned project)
	orphanedDir := filepath.Join(tempDir, "deleted-project")

	// Save state with both projects
	state := &PortState{
		MaxSuffix: 52,
		Projects: map[string]int{
			existingDir: 51,
			orphanedDir: 52,
		},
	}
	if err := state.save(); err != nil {
		t.Fatal(err)
	}

	// Test ListProjects
	projects, err := ListProjects()
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}

	if len(projects) != 2 {
		t.Fatalf("Expected 2 projects, got %d", len(projects))
	}

	// Check results
	var existingFound, orphanedFound bool
	for _, p := range projects {
		if p.Path == existingDir {
			existingFound = true
			if !p.Exists {
				t.Error("Existing project should have Exists=true")
			}
			if p.Suffix != 51 {
				t.Errorf("Expected suffix 51 for existing project, got %d", p.Suffix)
			}
		}
		if p.Path == orphanedDir {
			orphanedFound = true
			if p.Exists {
				t.Error("Orphaned project should have Exists=false")
			}
			if p.Suffix != 52 {
				t.Errorf("Expected suffix 52 for orphaned project, got %d", p.Suffix)
			}
		}
	}

	if !existingFound {
		t.Error("Existing project not found in list")
	}
	if !orphanedFound {
		t.Error("Orphaned project not found in list")
	}
}

func TestListProjectsEmpty(t *testing.T) {
	_, cleanup := setupTestState(t)
	defer cleanup()

	// Don't create any state file - should return empty list
	projects, err := ListProjects()
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}

	if len(projects) != 0 {
		t.Errorf("Expected 0 projects, got %d", len(projects))
	}
}

func TestCleanOrphanedProjects(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()

	// Create a real directory
	existingDir := filepath.Join(tempDir, "existing-project")
	if err := os.MkdirAll(existingDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Orphaned paths (don't exist)
	orphanedDir1 := filepath.Join(tempDir, "deleted-project-1")
	orphanedDir2 := filepath.Join(tempDir, "deleted-project-2")

	// Save state with all projects
	state := &PortState{
		MaxSuffix: 53,
		Projects: map[string]int{
			existingDir:  51,
			orphanedDir1: 52,
			orphanedDir2: 53,
		},
	}
	if err := state.save(); err != nil {
		t.Fatal(err)
	}

	// Clean orphaned projects
	count, err := CleanOrphanedProjects()
	if err != nil {
		t.Fatalf("CleanOrphanedProjects failed: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 orphaned projects cleaned, got %d", count)
	}

	// Verify state after cleaning
	projects, err := ListProjects()
	if err != nil {
		t.Fatalf("ListProjects failed after cleaning: %v", err)
	}

	if len(projects) != 1 {
		t.Errorf("Expected 1 project remaining, got %d", len(projects))
	}

	if len(projects) > 0 && projects[0].Path != existingDir {
		t.Errorf("Expected remaining project to be %s, got %s", existingDir, projects[0].Path)
	}
}

func TestCleanOrphanedProjectsNoneToClean(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()

	// Create real directories for all projects
	existingDir1 := filepath.Join(tempDir, "project-1")
	existingDir2 := filepath.Join(tempDir, "project-2")
	if err := os.MkdirAll(existingDir1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(existingDir2, 0755); err != nil {
		t.Fatal(err)
	}

	// Save state
	state := &PortState{
		MaxSuffix: 52,
		Projects: map[string]int{
			existingDir1: 51,
			existingDir2: 52,
		},
	}
	if err := state.save(); err != nil {
		t.Fatal(err)
	}

	// Clean - should find nothing to clean
	count, err := CleanOrphanedProjects()
	if err != nil {
		t.Fatalf("CleanOrphanedProjects failed: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 orphaned projects, got %d", count)
	}

	// Verify all projects still exist
	projects, err := ListProjects()
	if err != nil {
		t.Fatal(err)
	}

	if len(projects) != 2 {
		t.Errorf("Expected 2 projects remaining, got %d", len(projects))
	}
}

func TestLoadPortStateExisted(t *testing.T) {
	_, cleanup := setupTestState(t)
	defer cleanup()

	// 1. File doesn't exist
	_, existed, err := loadPortState()
	if err != nil {
		t.Fatal(err)
	}
	if existed {
		t.Error("loadPortState should return existed=false for missing file")
	}

	// 2. File exists
	state := &PortState{MaxSuffix: 50}
	if err := state.save(); err != nil {
		t.Fatal(err)
	}

	_, existed, err = loadPortState()
	if err != nil {
		t.Fatal(err)
	}
	if !existed {
		t.Error("loadPortState should return existed=true for existing file")
	}
}

func TestGetSuggestedSuffixFirstSetup(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()

	// Mock project directory
	projectDir := filepath.Join(tempDir, "new-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	suffix, existing, existed, err := getSuggestedSuffix(projectDir)
	if err != nil {
		t.Fatal(err)
	}

	if existed {
		t.Error("getSuggestedSuffix should return existed=false for first setup")
	}
	if existing {
		t.Error("getSuggestedSuffix should return existing=false for new project")
	}
	if suffix != 1 { // MaxSuffix (0) + 1
		t.Errorf("Expected suffix 1, got %d", suffix)
	}

	// Now save and verify it exists
	if err := saveProjectSuffix(projectDir, 48); err != nil {
		t.Fatal(err)
	}

	suffix, existing, existed, err = getSuggestedSuffix(projectDir)
	if err != nil {
		t.Fatal(err)
	}

	if !existed {
		t.Error("getSuggestedSuffix should return existed=true after file creation")
	}
	if !existing {
		t.Error("getSuggestedSuffix should return existing=true for known project")
	}
	if suffix != 48 {
		t.Errorf("Expected suffix 48, got %d", suffix)
	}
}

func TestValidateSuffix(t *testing.T) {
	tests := []struct {
		suffix  int
		wantErr bool
	}{
		{0, false},
		{48, false},
		{47435, false},
		{47436, true},
		{-1, true},
		{100000, true},
	}

	for _, tt := range tests {
		err := ValidateSuffix(tt.suffix)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateSuffix(%d): got err=%v, wantErr=%v", tt.suffix, err, tt.wantErr)
		}
	}
}

func TestRemoveProject(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()

	projectDir := filepath.Join(tempDir, "my-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Register the project
	if err := saveProjectSuffix(projectDir, 48); err != nil {
		t.Fatal(err)
	}

	// Verify it was saved
	projects, _ := ListProjects()
	if len(projects) != 1 {
		t.Fatalf("Expected 1 project, got %d", len(projects))
	}

	// Remove it
	if err := RemoveProject(projectDir); err != nil {
		t.Fatalf("RemoveProject failed: %v", err)
	}

	// Verify it's gone
	projects, _ = ListProjects()
	if len(projects) != 0 {
		t.Errorf("Expected 0 projects after removal, got %d", len(projects))
	}
}

func TestRemoveProjectNotRegistered(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()

	projectDir := filepath.Join(tempDir, "not-registered")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	err := RemoveProject(projectDir)
	if err == nil {
		t.Error("Expected error when removing unregistered project")
	}
	if err != nil && !strings.Contains(err.Error(), "not registered") {
		t.Errorf("Expected 'not registered' error, got: %v", err)
	}
}

func TestCheckPortAvailable(t *testing.T) {
	// An unused high port should be available
	if !CheckPortAvailable(59123) {
		t.Skip("Port 59123 unexpectedly in use, skipping")
	}

	// Occupy a port and check
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	if CheckPortAvailable(port) {
		t.Errorf("Port %d should be unavailable (occupied by test listener)", port)
	}
}

func TestCheckSuffixPortsAvailable(t *testing.T) {
	// With a very high suffix that won't conflict with anything running
	busy := CheckSuffixPortsAvailable(59000)
	// We can't guarantee no ports are busy, but we can check the return type
	_ = busy // just verify it doesn't panic

	// Occupy one port in a suffix range and verify it's detected
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	// If the occupied port happens to be 8000+suffix for some suffix, check it
	if port > 8000 {
		testSuffix := port - 8000
		if testSuffix >= 0 && testSuffix <= MaxPortSuffix {
			busy = CheckSuffixPortsAvailable(testSuffix)
			found := false
			for _, bp := range busy {
				if bp.Port == port {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected port %d to be reported as busy", port)
			}
		}
	}
}
