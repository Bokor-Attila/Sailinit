package main

import (
	"fmt"
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

var colorsEnabled = initColorsEnabled()

func initColorsEnabled() bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func colorize(color, text string) string {
	if !colorsEnabled {
		return text
	}
	return color + text + colorReset
}

func printSuccess(msg string) {
	fmt.Println(colorize(colorGreen, msg))
}

func printWarning(msg string) {
	fmt.Println(colorize(colorYellow, msg))
}

func printError(msg string) {
	fmt.Println(colorize(colorRed, msg))
}

func printInfo(msg string) {
	fmt.Println(colorize(colorCyan, msg))
}

func printHeader(msg string) {
	fmt.Println(colorize(colorBold, msg))
}
