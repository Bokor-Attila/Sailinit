package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// isInteractive reports whether sailinit has a terminal to ask questions on.
// It is a variable so tests can simulate a piped stdin.
var isInteractive = func() bool {
	return isTerminalFile(os.Stdin)
}

// exitFunc is os.Exit, indirected so tests can observe the code instead of
// killing the test binary.
var exitFunc = os.Exit

var (
	promptInput  io.Reader = os.Stdin
	promptReader *bufio.Reader
)

func readPromptLine() (string, error) {
	if promptReader == nil {
		promptReader = bufio.NewReader(promptInput)
	}
	line, err := promptReader.ReadString('\n')
	return strings.TrimSpace(line), err
}

// abortNoTTY stops with exitAborted when sailinit would have to prompt but has
// no terminal. It deliberately does not fall back to a default: continuing
// silently (or exiting 0 having done nothing) is how an automated caller ends
// up reporting success for work that never happened.
func abortNoTTY(need, hint string) {
	printError("sailinit: no TTY and -y not given")
	printError(fmt.Sprintf("  needs confirmation: %s", need))
	printError(fmt.Sprintf("  %s", hint))
	exitFunc(exitAborted)
}

// confirmOrAbort asks a [y/N] question. It returns true when the answer is
// yes, false when it is no, and does not return at all when there is nothing
// to ask on.
func confirmOrAbort(need, hint string, assumeYes bool) bool {
	if assumeYes {
		return true
	}
	if !isInteractive() {
		abortNoTTY(need, hint)
		return false
	}

	fmt.Fprint(stderr, "Continue anyway? [y/N]: ")
	answer, err := readPromptLine()
	if err != nil && answer == "" {
		// EOF on a terminal means the user hit Ctrl-D: treat it as a refusal
		// rather than as consent.
		return false
	}
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes")
}

// promptLineOrAbort prints a prompt and returns the trimmed answer, which may
// be empty when the user just pressed Enter. Without a terminal it aborts.
func promptLineOrAbort(prompt, need, hint string) string {
	if !isInteractive() {
		abortNoTTY(need, hint)
		return ""
	}

	fmt.Fprint(stderr, prompt)
	answer, err := readPromptLine()
	if err != nil && answer == "" {
		abortNoTTY(need, hint)
		return ""
	}
	return answer
}
