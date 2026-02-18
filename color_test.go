package main

import "testing"

func TestColorizeEnabled(t *testing.T) {
	original := colorsEnabled
	defer func() { colorsEnabled = original }()

	colorsEnabled = true

	result := colorize(colorRed, "error")
	expected := colorRed + "error" + colorReset
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestColorizeDisabled(t *testing.T) {
	original := colorsEnabled
	defer func() { colorsEnabled = original }()

	colorsEnabled = false

	result := colorize(colorRed, "error")
	if result != "error" {
		t.Errorf("Expected plain text %q, got %q", "error", result)
	}
}

func TestColorizeAllColors(t *testing.T) {
	original := colorsEnabled
	defer func() { colorsEnabled = original }()

	colorsEnabled = true

	tests := []struct {
		color    string
		name     string
		expected string
	}{
		{colorRed, "red", colorRed + "text" + colorReset},
		{colorGreen, "green", colorGreen + "text" + colorReset},
		{colorYellow, "yellow", colorYellow + "text" + colorReset},
		{colorCyan, "cyan", colorCyan + "text" + colorReset},
		{colorBold, "bold", colorBold + "text" + colorReset},
		{colorDim, "dim", colorDim + "text" + colorReset},
	}

	for _, tt := range tests {
		result := colorize(tt.color, "text")
		if result != tt.expected {
			t.Errorf("colorize(%s) = %q, want %q", tt.name, result, tt.expected)
		}
	}
}
