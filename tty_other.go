//go:build !darwin && !linux

package main

import "os"

// isTerminalFile falls back to the character-device heuristic on platforms
// sailinit does not ship binaries for.
func isTerminalFile(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
