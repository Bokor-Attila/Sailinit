package main

import (
	"fmt"
	"io"
	"os"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

// stdout carries data: JSON payloads, the --port value, completion scripts,
// the --list/--status tables and the --doctor report. stderr carries every
// human-facing message, so piping stdout into a parser stays safe even when
// something goes wrong. Both are variables so tests can capture them.
var (
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

var colorsEnabled = isTerminal(os.Stdout)
var errColorsEnabled = isTerminal(os.Stderr)

func isTerminal(f *os.File) bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	return isTerminalFile(f)
}

// colorize wraps text for the stdout stream.
func colorize(color, text string) string {
	if !colorsEnabled {
		return text
	}
	return color + text + colorReset
}

// colorizeErr wraps text for the stderr stream, which may be a terminal even
// when stdout is being piped (or the other way round).
func colorizeErr(color, text string) string {
	if !errColorsEnabled {
		return text
	}
	return color + text + colorReset
}

func printSuccess(msg string) {
	fmt.Fprintln(stderr, colorizeErr(colorGreen, msg))
}

func printWarning(msg string) {
	fmt.Fprintln(stderr, colorizeErr(colorYellow, msg))
}

func printError(msg string) {
	fmt.Fprintln(stderr, colorizeErr(colorRed, msg))
}

func printInfo(msg string) {
	fmt.Fprintln(stderr, colorizeErr(colorCyan, msg))
}

func printHeader(msg string) {
	fmt.Fprintln(stderr, colorizeErr(colorBold, msg))
}
