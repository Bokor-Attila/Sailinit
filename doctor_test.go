package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// findDiag returns the diagnostic with the given name.
func findDiag(t *testing.T, diags []Diagnostic, name string) Diagnostic {
	t.Helper()
	for _, d := range diags {
		if d.Name == name {
			return d
		}
	}
	t.Fatalf("no diagnostic named %q in %v", name, diagNames(diags))
	return Diagnostic{}
}

func diagNames(diags []Diagnostic) []string {
	var names []string
	for _, d := range diags {
		names = append(names, d.Name)
	}
	return names
}

// collectDiags runs the same checks runDoctor does, without the printing.
func collectDiags(projectDir string) []Diagnostic {
	var diags []Diagnostic
	diags = append(diags, checkRegistry()...)
	diags = append(diags, checkDuplicateSuffixes())
	diags = append(diags, checkOrphans())
	diags = append(diags, checkCurrentProject(projectDir)...)
	return diags
}

// makeLaravelProject creates a directory that looks like a Laravel project.
func makeLaravelProject(t *testing.T, dir string, envAppPort int) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "composer.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if envAppPort > 0 {
		env := fmt.Sprintf("APP_NAME=Test\nAPP_PORT=%d\n", envAppPort)
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(env), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestDoctorCleanRegistryPasses(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()

	proj := makeLaravelProject(t, filepath.Join(tempDir, "blog"), 8051)
	state := &PortState{MaxSuffix: 51, Projects: map[string]int{proj: 51}}
	if err := state.save(); err != nil {
		t.Fatal(err)
	}

	diags := collectDiags(proj)
	for _, d := range diags {
		if d.Status == statusFail {
			t.Errorf("clean registry produced a FAIL: %s: %s", d.Name, d.Detail)
		}
	}
	if got := findDiag(t, diags, ".env ports"); got.Status != statusOK {
		t.Errorf(".env ports = %s (%s), want OK", got.Status, got.Detail)
	}
}

func TestDoctorDetectsDuplicateSuffixes(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()

	a := makeLaravelProject(t, filepath.Join(tempDir, "alpha"), 8051)
	b := makeLaravelProject(t, filepath.Join(tempDir, "beta"), 8051)
	state := &PortState{MaxSuffix: 51, Projects: map[string]int{a: 51, b: 51}}
	if err := state.save(); err != nil {
		t.Fatal(err)
	}

	d := checkDuplicateSuffixes()
	if d.Status != statusFail {
		t.Fatalf("duplicate suffixes = %s, want FAIL (detail: %s)", d.Status, d.Detail)
	}
	// The message must name both colliding projects, or it is not actionable.
	if !strings.Contains(d.Detail, a) || !strings.Contains(d.Detail, b) {
		t.Errorf("detail %q should name both %s and %s", d.Detail, a, b)
	}
	if !strings.Contains(d.Detail, "51") {
		t.Errorf("detail %q should name the shared suffix 51", d.Detail)
	}
}

func TestDoctorDetectsEnvDrift(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()

	// Registry says suffix 51 (APP_PORT 8051), .env says APP_PORT 8099.
	proj := makeLaravelProject(t, filepath.Join(tempDir, "drifted"), 8099)
	state := &PortState{MaxSuffix: 51, Projects: map[string]int{proj: 51}}
	if err := state.save(); err != nil {
		t.Fatal(err)
	}

	d := findDiag(t, collectDiags(proj), ".env ports")
	if d.Status != statusFail {
		t.Fatalf(".env drift = %s, want FAIL (detail: %s)", d.Status, d.Detail)
	}
	// Both conflicting ports must appear, so the user can see which is which.
	if !strings.Contains(d.Detail, "8099") || !strings.Contains(d.Detail, "8051") {
		t.Errorf("detail %q should name both 8099 and 8051", d.Detail)
	}
}

func TestDoctorDetectsOrphanedProjects(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()

	gone := filepath.Join(tempDir, "deleted")
	state := &PortState{MaxSuffix: 52, Projects: map[string]int{gone: 52}}
	if err := state.save(); err != nil {
		t.Fatal(err)
	}

	d := checkOrphans()
	if d.Status != statusWarn {
		t.Fatalf("orphans = %s, want WARN (detail: %s)", d.Status, d.Detail)
	}
	if !strings.Contains(d.Fix, "--clean") {
		t.Errorf("fix %q should suggest --clean", d.Fix)
	}
}

func TestDoctorDetectsStaleMaxSuffix(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()

	// A project uses 80 but max_suffix says 51, so the next allocation
	// would hand out 52 and eventually collide.
	proj := makeLaravelProject(t, filepath.Join(tempDir, "app"), 8080)
	state := &PortState{MaxSuffix: 51, Projects: map[string]int{proj: 80}}
	if err := state.save(); err != nil {
		t.Fatal(err)
	}

	d := findDiag(t, checkRegistry(), "Registry max suffix")
	if d.Status != statusFail {
		t.Fatalf("stale max suffix = %s, want FAIL (detail: %s)", d.Status, d.Detail)
	}
}

func TestDoctorSkipsCurrentProjectChecksOutsideLaravel(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()

	plain := filepath.Join(tempDir, "not-laravel")
	if err := os.MkdirAll(plain, 0755); err != nil {
		t.Fatal(err)
	}

	if diags := checkCurrentProject(plain); diags != nil {
		t.Errorf("expected no current-project diagnostics outside a Laravel project, got %v", diagNames(diags))
	}
}

func TestDoctorWarnsUnregisteredLaravelProject(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()

	proj := makeLaravelProject(t, filepath.Join(tempDir, "fresh"), 0)
	state := &PortState{MaxSuffix: 51, Projects: map[string]int{}}
	if err := state.save(); err != nil {
		t.Fatal(err)
	}

	d := findDiag(t, checkCurrentProject(proj), "Current project")
	if d.Status != statusWarn {
		t.Errorf("unregistered project = %s, want WARN (detail: %s)", d.Status, d.Detail)
	}
}

func TestRunDoctorExitStatus(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()
	restore, _ := setupMockRunners(t)
	defer restore()

	proj := makeLaravelProject(t, filepath.Join(tempDir, "blog"), 8051)
	clean := &PortState{MaxSuffix: 51, Projects: map[string]int{proj: 51}}
	if err := clean.save(); err != nil {
		t.Fatal(err)
	}
	if !runDoctor(proj, true) {
		t.Error("clean registry should report healthy")
	}

	// Introduce a duplicate suffix; the same run must now report unhealthy.
	other := makeLaravelProject(t, filepath.Join(tempDir, "shop"), 8051)
	broken := &PortState{MaxSuffix: 51, Projects: map[string]int{proj: 51, other: 51}}
	if err := broken.save(); err != nil {
		t.Fatal(err)
	}
	if runDoctor(proj, true) {
		t.Error("duplicate suffixes should report unhealthy")
	}
}

func TestRunDoctorJSONShape(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()
	restore, _ := setupMockRunners(t)
	defer restore()

	proj := makeLaravelProject(t, filepath.Join(tempDir, "blog"), 8051)
	state := &PortState{MaxSuffix: 51, Projects: map[string]int{proj: 51}}
	if err := state.save(); err != nil {
		t.Fatal(err)
	}

	var healthy bool
	output, _ := captureStreams(t, func() {
		healthy = runDoctor(proj, true)
	})

	var out struct {
		Healthy     bool         `json:"healthy"`
		Diagnostics []Diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(output), &out); err != nil {
		t.Fatalf("doctor --json did not emit valid JSON: %v\n%s", err, output)
	}
	if out.Healthy != healthy {
		t.Errorf("json healthy = %v, but runDoctor returned %v", out.Healthy, healthy)
	}
	if len(out.Diagnostics) == 0 {
		t.Error("expected at least one diagnostic in the JSON output")
	}
}

func readAll(t *testing.T, r *os.File) string {
	t.Helper()
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return sb.String()
}
