package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// captureStreams runs fn with the package stdout/stderr writers redirected and
// returns what each one received.
func captureStreams(t *testing.T, fn func()) (string, string) {
	t.Helper()

	origOut, origErr := stdout, stderr
	origOutColors, origErrColors := colorsEnabled, errColorsEnabled
	var outBuf, errBuf bytes.Buffer
	stdout, stderr = &outBuf, &errBuf
	colorsEnabled, errColorsEnabled = false, false
	defer func() {
		stdout, stderr = origOut, origErr
		colorsEnabled, errColorsEnabled = origOutColors, origErrColors
	}()

	fn()
	return outBuf.String(), errBuf.String()
}

func TestMessagesGoToStderr(t *testing.T) {
	cases := []struct {
		name string
		fn   func(string)
	}{
		{"error", printError},
		{"warning", printWarning},
		{"info", printInfo},
		{"success", printSuccess},
		{"header", printHeader},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, errOut := captureStreams(t, func() { tc.fn("a message") })
			if out != "" {
				t.Errorf("print%s wrote %q to stdout; stdout must stay data-only", tc.name, out)
			}
			if !strings.Contains(errOut, "a message") {
				t.Errorf("print%s did not reach stderr, got %q", tc.name, errOut)
			}
		})
	}
}

// An error while rendering must not land in the middle of the JSON payload.
func TestDoctorJSONStdoutStaysParseableWithStderrNoise(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()
	restore, _ := setupMockRunners(t)
	defer restore()

	proj := makeLaravelProject(t, tempDir+"/blog", 8051)
	state := &PortState{MaxSuffix: 51, Projects: map[string]int{proj: 51}}
	if err := state.save(); err != nil {
		t.Fatal(err)
	}

	out, errOut := captureStreams(t, func() {
		printWarning("noise that used to corrupt stdout")
		runDoctor(proj, true)
		printError("more noise")
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("stdout was not pure JSON: %v\n%s", err, out)
	}
	if !strings.Contains(errOut, "noise that used to corrupt stdout") {
		t.Errorf("expected the noise on stderr, got %q", errOut)
	}
}

func TestListJSONStdoutIsPureJSON(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()
	restore, _ := setupMockRunners(t)
	defer restore()

	proj := makeLaravelProject(t, tempDir+"/shop", 8052)
	state := &PortState{MaxSuffix: 52, Projects: map[string]int{proj: 52}}
	if err := state.save(); err != nil {
		t.Fatal(err)
	}

	out, _ := captureStreams(t, func() {
		printInfo("progress chatter")
		handleList(true)
	})

	var parsed []map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("--list --json stdout was not pure JSON: %v\n%s", err, out)
	}
	if len(parsed) != 1 {
		t.Fatalf("expected 1 project, got %d", len(parsed))
	}
}

func TestDoctorHumanReportGoesToStdout(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()
	restore, _ := setupMockRunners(t)
	defer restore()

	proj := makeLaravelProject(t, tempDir+"/blog", 8051)
	state := &PortState{MaxSuffix: 51, Projects: map[string]int{proj: 51}}
	if err := state.save(); err != nil {
		t.Fatal(err)
	}

	out, _ := captureStreams(t, func() { runDoctor(proj, false) })

	// Header, check lines and verdict are one report; none of it may be split
	// onto stderr.
	for _, want := range []string{"sailinit doctor", "Registry", "problems"} {
		if !strings.Contains(strings.ToLower(out), strings.ToLower(want)) {
			t.Errorf("expected %q in the stdout report, got:\n%s", want, out)
		}
	}
}

// JSON payloads must never carry ANSI codes. This has to force colours on:
// when stdout is a pipe, colorize() is already a no-op, so a piped run cannot
// detect the bug this guards against.
func TestJSONNeverContainsANSIWhenColorsEnabled(t *testing.T) {
	tempDir, cleanup := setupTestState(t)
	defer cleanup()
	restore, _ := setupMockRunners(t)
	defer restore()

	proj := makeLaravelProject(t, tempDir+"/blog", 8051)
	state := &PortState{MaxSuffix: 51, Projects: map[string]int{proj: 51, tempDir + "/gone": 52}}
	if err := state.save(); err != nil {
		t.Fatal(err)
	}

	cases := map[string]func(){
		"--list -j":   func() { handleList(true) },
		"--status -j": func() { showProjectStatus(true) },
		"--ports -j":  func() { handlePorts(proj, true) },
		"--doctor -j": func() { runDoctor(proj, true) },
	}

	for name, fn := range cases {
		t.Run(name, func(t *testing.T) {
			origOut, origColors := stdout, colorsEnabled
			var buf bytes.Buffer
			stdout = &buf
			colorsEnabled = true
			defer func() { stdout, colorsEnabled = origOut, origColors }()

			fn()

			// encoding/json escapes the ESC byte as \u001b, so the raw text
			// has to be checked for both spellings.
			raw := buf.String()
			for _, esc := range []string{"\033[", `\u001b`} {
				if strings.Contains(raw, esc) {
					t.Errorf("%s emitted ANSI codes inside JSON:\n%q", name, raw)
				}
			}
			var parsed any
			if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
				t.Errorf("%s did not emit valid JSON: %v\n%s", name, err, raw)
			}
		})
	}
}
