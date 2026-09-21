package main

import (
	"strings"
	"testing"
)

type exitPanic struct{ code int }

// stubTTY points the prompt helpers at a canned answer and records any exit.
func stubTTY(t *testing.T, interactive bool, answer string) *int {
	t.Helper()

	origInteractive := isInteractive
	origInput := promptInput
	origReader := promptReader
	origExit := exitFunc

	var code int
	isInteractive = func() bool { return interactive }
	promptInput = strings.NewReader(answer)
	promptReader = nil
	exitFunc = func(c int) { code = c; panic(exitPanic{c}) }

	t.Cleanup(func() {
		isInteractive = origInteractive
		promptInput = origInput
		promptReader = origReader
		exitFunc = origExit
	})
	return &code
}

// catchExit runs fn and reports the exit code it requested, if any.
func catchExit(fn func()) (code int, exited bool) {
	defer func() {
		if r := recover(); r != nil {
			if ep, ok := r.(exitPanic); ok {
				code, exited = ep.code, true
				return
			}
			panic(r)
		}
	}()
	fn()
	return 0, false
}

func TestConfirmOrAbortAnswers(t *testing.T) {
	cases := []struct {
		answer string
		want   bool
	}{
		{"y\n", true},
		{"Y\n", true},
		{"yes\n", true},
		{"n\n", false},
		{"\n", false},
		{"anything else\n", false},
		{"", false}, // EOF on a terminal (Ctrl-D) is a refusal, not consent
	}

	for _, tc := range cases {
		t.Run(strings.TrimSpace(tc.answer), func(t *testing.T) {
			stubTTY(t, true, tc.answer)
			var got bool
			code, exited := catchExit(func() {
				got = confirmOrAbort("something", "a hint", false)
			})
			if exited {
				t.Fatalf("unexpected exit(%d) for an interactive answer", code)
			}
			if got != tc.want {
				t.Errorf("confirmOrAbort(%q) = %v, want %v", tc.answer, got, tc.want)
			}
		})
	}
}

func TestConfirmOrAbortAssumeYesSkipsPrompt(t *testing.T) {
	// Not interactive and no input: -y must still go through.
	stubTTY(t, false, "")
	code, exited := catchExit(func() {
		if !confirmOrAbort("something", "a hint", true) {
			t.Error("assumeYes must return true")
		}
	})
	if exited {
		t.Fatalf("assumeYes must not exit, got exit(%d)", code)
	}
}

func TestConfirmOrAbortExitsWithoutTTY(t *testing.T) {
	stubTTY(t, false, "")
	out, errOut := captureStreams(t, func() {
		code, exited := catchExit(func() { confirmOrAbort("ports 8051 in use", "re-run with -y", false) })
		if !exited {
			t.Fatal("expected an exit when there is no TTY")
		}
		if code != exitAborted {
			t.Errorf("exit code = %d, want %d (aborted)", code, exitAborted)
		}
	})
	if out != "" {
		t.Errorf("abort message must not touch stdout, got %q", out)
	}
	for _, want := range []string{"no TTY", "ports 8051 in use", "re-run with -y"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("expected %q in the abort message, got %q", want, errOut)
		}
	}
}

func TestPromptLineOrAbortExitsWithoutTTY(t *testing.T) {
	stubTTY(t, false, "")
	captureStreams(t, func() {
		code, exited := catchExit(func() {
			promptLineOrAbort("Suffix: ", "the starting suffix is unset", "re-run with -y")
		})
		if !exited {
			t.Fatal("expected an exit when there is no TTY")
		}
		if code != exitAborted {
			t.Errorf("exit code = %d, want %d (aborted)", code, exitAborted)
		}
	})
}

func TestPromptLineOrAbortReturnsAnswer(t *testing.T) {
	stubTTY(t, true, "  71  \n")
	captureStreams(t, func() {
		got := promptLineOrAbort("Suffix: ", "need", "hint")
		if got != "71" {
			t.Errorf("promptLineOrAbort = %q, want %q", got, "71")
		}
	})
}

func TestPromptLineOrAbortEmptyAnswerMeansDefault(t *testing.T) {
	stubTTY(t, true, "\n")
	captureStreams(t, func() {
		code, exited := catchExit(func() {
			if got := promptLineOrAbort("Suffix: ", "need", "hint"); got != "" {
				t.Errorf("expected an empty answer, got %q", got)
			}
		})
		if exited {
			t.Fatalf("pressing Enter must not abort, got exit(%d)", code)
		}
	})
}

func TestPromptsWriteToStderrOnly(t *testing.T) {
	stubTTY(t, true, "y\n")
	out, errOut := captureStreams(t, func() { confirmOrAbort("need", "hint", false) })
	if out != "" {
		t.Errorf("prompt leaked to stdout: %q", out)
	}
	if !strings.Contains(errOut, "Continue anyway?") {
		t.Errorf("prompt missing from stderr, got %q", errOut)
	}
}
