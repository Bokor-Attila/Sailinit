package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	buildOnce sync.Once
	binPath   string
	buildErr  error
)

// sailinitBinary builds the CLI once per test run. The end-to-end tests below
// need a real process: exit codes and TTY detection cannot be observed from
// inside the test binary.
func sailinitBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "sailinit-bin-*")
		if err != nil {
			buildErr = err
			return
		}
		binPath = filepath.Join(dir, "sailinit")
		out, err := exec.Command("go", "build", "-o", binPath, ".").CombinedOutput()
		if err != nil {
			buildErr = err
			binPath = string(out)
		}
	})
	if buildErr != nil {
		t.Fatalf("could not build sailinit: %v\n%s", buildErr, binPath)
	}
	return binPath
}

type runResult struct {
	stdout string
	stderr string
	code   int
}

// run executes sailinit with stdin closed, which is exactly the shape an agent
// or CI job invokes it in: no terminal attached.
func run(t *testing.T, workdir string, env []string, args ...string) runResult {
	t.Helper()

	cmd := exec.Command(sailinitBinary(t), args...)
	cmd.Dir = workdir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = nil

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("running sailinit failed: %v", err)
	}
	return runResult{stdout: outBuf.String(), stderr: errBuf.String(), code: code}
}

// sandbox returns an isolated SAILINIT_HOME and a Laravel-looking project dir.
func sandbox(t *testing.T) (home string, project string) {
	t.Helper()
	root := t.TempDir()
	// The registry is keyed by absolute path and sailinit does not resolve
	// symlinks, while t.TempDir() hands out a symlinked path on macOS.
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	home = filepath.Join(root, "home")
	project = filepath.Join(root, "project")
	if err := os.MkdirAll(project, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "composer.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	return home, project
}

func env(home string) []string {
	return []string{"SAILINIT_HOME=" + home, "NO_COLOR=1"}
}

func TestExitCodeOKForVersion(t *testing.T) {
	home, project := sandbox(t)
	res := run(t, project, env(home), "--version")
	if res.code != 0 {
		t.Errorf("--version exit = %d, want 0", res.code)
	}
	if !strings.Contains(res.stdout, "sailinit") {
		t.Errorf("expected version on stdout, got %q", res.stdout)
	}
}

func TestExitCodeUsageForUnknownShell(t *testing.T) {
	home, project := sandbox(t)
	res := run(t, project, env(home), "--completion", "powershell")
	if res.code != 2 {
		t.Errorf("unknown shell exit = %d, want 2 (usage)", res.code)
	}
	if res.stdout != "" {
		t.Errorf("usage error must not write to stdout, got %q", res.stdout)
	}
}

func TestExitCodeUsageForUnknownFlag(t *testing.T) {
	home, project := sandbox(t)
	res := run(t, project, env(home), "--nope")
	if res.code != 2 {
		t.Errorf("unknown flag exit = %d, want 2 (usage)", res.code)
	}
}

func TestExitCodeUnhealthyForFailingDoctor(t *testing.T) {
	home, project := sandbox(t)

	// Two projects on one suffix is a FAIL-level diagnostic.
	registry := `{"max_suffix":51,"projects":{"` + project + `":51,"` + project + `-other":51}}`
	if err := os.MkdirAll(home, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "ports.json"), []byte(registry), 0644); err != nil {
		t.Fatal(err)
	}

	res := run(t, project, env(home), "--doctor")
	if res.code != 4 {
		t.Fatalf("failing doctor exit = %d, want 4 (unhealthy); stderr:\n%s", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "[FAIL]") {
		t.Errorf("expected the report on stdout, got %q", res.stdout)
	}
}

func TestExitCodeOKForHealthyDoctor(t *testing.T) {
	home, project := sandbox(t)
	res := run(t, project, env(home), "--doctor")
	if res.code != 0 && res.code != 4 {
		t.Fatalf("doctor exit = %d, want 0 or 4; stderr:\n%s", res.code, res.stderr)
	}
	// Whatever the verdict, the JSON form must stay parseable.
	res = run(t, project, env(home), "--doctor", "-j")
	var parsed map[string]any
	if err := json.Unmarshal([]byte(res.stdout), &parsed); err != nil {
		t.Fatalf("doctor -j stdout not JSON: %v\n%s", err, res.stdout)
	}
}

// The core regression: with no terminal and no -y, sailinit used to print a
// prompt, read EOF, treat it as "no", and exit 0 having changed nothing.
func TestNoTTYAbortsInsteadOfSilentlySucceeding(t *testing.T) {
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	home := filepath.Join(root, "home")
	project := filepath.Join(root, "empty") // no composer.json, no artisan
	if err := os.MkdirAll(project, 0755); err != nil {
		t.Fatal(err)
	}

	res := run(t, project, env(home), "--dry-run")

	if res.code != 3 {
		t.Fatalf("no-TTY run exit = %d, want 3 (aborted); stdout:\n%s\nstderr:\n%s", res.code, res.stdout, res.stderr)
	}
	if !strings.Contains(res.stderr, "no TTY") {
		t.Errorf("expected an explanation on stderr, got %q", res.stderr)
	}
	if _, err := os.Stat(filepath.Join(home, "ports.json")); err == nil {
		t.Error("aborted run must not have written to the registry")
	}
}

func TestNoTTYAbortsOnFirstRunSuffixPrompt(t *testing.T) {
	home, project := sandbox(t)

	res := run(t, project, env(home), "--dry-run")

	if res.code != 3 {
		t.Fatalf("first-run no-TTY exit = %d, want 3 (aborted); stderr:\n%s", res.code, res.stderr)
	}
	if !strings.Contains(res.stderr, "starting suffix") {
		t.Errorf("expected the starting-suffix reason on stderr, got %q", res.stderr)
	}
	if _, err := os.Stat(filepath.Join(project, ".env")); err == nil {
		t.Error("aborted run must not have written .env")
	}
}

func TestYesFlagProceedsWithoutTTY(t *testing.T) {
	home, project := sandbox(t)

	res := run(t, project, env(home), "--dry-run", "-y")

	if res.code != 0 {
		t.Fatalf("-y dry-run exit = %d, want 0; stderr:\n%s", res.code, res.stderr)
	}
	if !strings.Contains(res.stderr, "48") {
		t.Errorf("expected the default suffix 48 to be used, stderr:\n%s", res.stderr)
	}
}

func TestPortsPlainOutput(t *testing.T) {
	home, project := sandbox(t)
	registry := `{"max_suffix":51,"projects":{"` + project + `":51}}`
	if err := os.MkdirAll(home, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "ports.json"), []byte(registry), 0644); err != nil {
		t.Fatal(err)
	}

	res := run(t, project, env(home), "--ports")
	if res.code != 0 {
		t.Fatalf("--ports exit = %d, want 0; stderr:\n%s", res.code, res.stderr)
	}

	got := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(res.stdout), "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			t.Fatalf("line %q is not KEY=VALUE", line)
		}
		got[parts[0]] = parts[1]
	}

	for key, want := range map[string]string{
		"APP_PORT":        "8051",
		"FORWARD_DB_PORT": "3351",
		"VITE_PORT":       "5151",
	} {
		if got[key] != want {
			t.Errorf("%s = %q, want %q", key, got[key], want)
		}
	}
	if len(got) < 12 {
		t.Errorf("expected every port, got only %d entries", len(got))
	}
}

func TestPortsJSONOutput(t *testing.T) {
	home, project := sandbox(t)
	registry := `{"max_suffix":51,"projects":{"` + project + `":51}}`
	if err := os.MkdirAll(home, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "ports.json"), []byte(registry), 0644); err != nil {
		t.Fatal(err)
	}

	res := run(t, project, env(home), "--ports", "-j")
	var out struct {
		Path   string         `json:"path"`
		Suffix int            `json:"suffix"`
		Ports  map[string]int `json:"ports"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatalf("--ports -j stdout not JSON: %v\n%s", err, res.stdout)
	}
	if out.Suffix != 51 {
		t.Errorf("suffix = %d, want 51", out.Suffix)
	}
	if out.Ports["APP_PORT"] != 8051 {
		t.Errorf("APP_PORT = %d, want 8051", out.Ports["APP_PORT"])
	}
}

// --ports must reflect a custom base port, which is the whole reason callers
// should not compute 8000+suffix themselves.
func TestPortsHonoursCustomBasePort(t *testing.T) {
	home, project := sandbox(t)
	registry := `{"max_suffix":51,"projects":{"` + project + `":51}}`
	if err := os.MkdirAll(home, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "ports.json"), []byte(registry), 0644); err != nil {
		t.Fatal(err)
	}

	e := append(env(home), "SAILINIT_BASE_APP_PORT=9000")
	res := run(t, project, e, "--ports", "-j")

	var out struct {
		Ports map[string]int `json:"ports"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, res.stdout)
	}
	if out.Ports["APP_PORT"] != 9051 {
		t.Errorf("APP_PORT = %d, want 9051 with a custom base", out.Ports["APP_PORT"])
	}
}

// On a machine with no registry, --ports and --port must report the suffix that
// a real run would assign (the first-run default), not the next free slot.
func TestPortsProjectionMatchesFirstRun(t *testing.T) {
	home, project := sandbox(t)

	res := run(t, project, env(home), "--ports", "-j")
	var out struct {
		Suffix int            `json:"suffix"`
		Ports  map[string]int `json:"ports"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, res.stdout)
	}
	if out.Suffix != 48 {
		t.Errorf("projected suffix = %d, want 48 (the first-run default)", out.Suffix)
	}

	portRes := run(t, project, env(home), "--port")
	if got := strings.TrimSpace(portRes.stdout); got != "8048" {
		t.Errorf("--port = %q, want %q", got, "8048")
	}

	// What a real run assigns must agree with the projection.
	setup := run(t, project, env(home), "--dry-run", "-y")
	if setup.code != 0 {
		t.Fatalf("setup exit = %d; stderr:\n%s", setup.code, setup.stderr)
	}
	if !strings.Contains(setup.stderr, "8048") {
		t.Errorf("setup did not use the projected port 8048; stderr:\n%s", setup.stderr)
	}
}

func writeRegistry(t *testing.T, home string, entries string) {
	t.Helper()
	if err := os.MkdirAll(home, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "ports.json"), []byte(entries), 0644); err != nil {
		t.Fatal(err)
	}
}

func readRegistry(t *testing.T, home string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, "ports.json"))
	if err != nil {
		t.Fatalf("reading registry: %v", err)
	}
	return string(data)
}

func TestCleanDryRunLeavesRegistryUntouched(t *testing.T) {
	home, project := sandbox(t)
	gone := filepath.Join(filepath.Dir(project), "deleted")
	writeRegistry(t, home, `{"max_suffix":52,"projects":{"`+project+`":51,"`+gone+`":52}}`)

	res := run(t, project, env(home), "--clean", "--dry-run")
	if res.code != 0 {
		t.Fatalf("--clean --dry-run exit = %d, want 0; stderr:\n%s", res.code, res.stderr)
	}
	if !strings.Contains(res.stderr, "[dry-run]") || !strings.Contains(res.stderr, gone) {
		t.Errorf("expected a preview naming %s, stderr:\n%s", gone, res.stderr)
	}
	if !strings.Contains(readRegistry(t, home), gone) {
		t.Error("--dry-run must not remove the orphaned entry")
	}
}

func TestCleanWithoutDryRunRemovesEntry(t *testing.T) {
	home, project := sandbox(t)
	gone := filepath.Join(filepath.Dir(project), "deleted")
	writeRegistry(t, home, `{"max_suffix":52,"projects":{"`+project+`":51,"`+gone+`":52}}`)

	if res := run(t, project, env(home), "--clean"); res.code != 0 {
		t.Fatalf("--clean exit = %d; stderr:\n%s", res.code, res.stderr)
	}
	if strings.Contains(readRegistry(t, home), gone) {
		t.Error("--clean should have removed the orphaned entry")
	}
}

func TestRemoveDryRunLeavesRegistryUntouched(t *testing.T) {
	home, project := sandbox(t)
	writeRegistry(t, home, `{"max_suffix":51,"projects":{"`+project+`":51}}`)

	res := run(t, project, env(home), "--remove", "--dry-run")
	if res.code != 0 {
		t.Fatalf("--remove --dry-run exit = %d, want 0; stderr:\n%s", res.code, res.stderr)
	}
	if !strings.Contains(res.stderr, "[dry-run]") {
		t.Errorf("expected a preview, stderr:\n%s", res.stderr)
	}
	if !strings.Contains(readRegistry(t, home), project) {
		t.Error("--dry-run must not remove the project")
	}
}

func TestRemoveWithoutDryRunRemovesProject(t *testing.T) {
	home, project := sandbox(t)
	writeRegistry(t, home, `{"max_suffix":51,"projects":{"`+project+`":51}}`)

	if res := run(t, project, env(home), "--remove"); res.code != 0 {
		t.Fatalf("--remove exit = %d; stderr:\n%s", res.code, res.stderr)
	}
	if strings.Contains(readRegistry(t, home), project) {
		t.Error("--remove should have removed the project")
	}
}

func TestStatusJSON(t *testing.T) {
	home, project := sandbox(t)
	gone := filepath.Join(filepath.Dir(project), "deleted")
	writeRegistry(t, home, `{"max_suffix":52,"projects":{"`+project+`":51,"`+gone+`":52}}`)

	res := run(t, project, env(home), "--status", "-j")
	if res.code != 0 {
		t.Fatalf("--status -j exit = %d; stderr:\n%s", res.code, res.stderr)
	}

	var out []struct {
		Path       string `json:"path"`
		Suffix     int    `json:"suffix"`
		Exists     bool   `json:"exists"`
		AppPort    int    `json:"app_port"`
		Containers struct {
			State   string `json:"state"`
			Running int    `json:"running"`
		} `json:"containers"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatalf("--status -j stdout not JSON: %v\n%s", err, res.stdout)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(out))
	}
	if out[0].AppPort != 8051 || out[0].Suffix != 51 {
		t.Errorf("first entry = suffix %d, app_port %d; want 51/8051", out[0].Suffix, out[0].AppPort)
	}
	if !out[0].Exists {
		t.Error("the existing project should report exists=true")
	}
	if out[1].Containers.State != "missing" {
		t.Errorf("deleted project state = %q, want %q", out[1].Containers.State, "missing")
	}
}

func TestStatusJSONEmptyRegistryIsEmptyArray(t *testing.T) {
	home, project := sandbox(t)

	res := run(t, project, env(home), "--status", "-j")
	var out []any
	if err := json.Unmarshal([]byte(res.stdout), &out); err != nil {
		t.Fatalf("expected [] for an empty registry, got %q (%v)", res.stdout, err)
	}
	if len(out) != 0 {
		t.Errorf("expected an empty array, got %d entries", len(out))
	}
}
