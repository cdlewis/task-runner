package main

import (
	"fmt"
	"strings"
)

// ANSI color codes
const (
	colorReset   = "\033[0m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
	colorBold    = "\033[1m"
)

// Gradient colors for rainbow effect (red -> yellow -> green -> cyan -> blue -> magenta)
var gradientColors = []string{
	"\033[31m", // red
	"\033[33m", // yellow
	"\033[32m", // green
	"\033[36m", // cyan
	"\033[34m", // blue
	"\033[35m", // magenta
}

// ColorSuccess returns green text
func ColorSuccess(text string) string {
	return colorGreen + text + colorReset
}

// ColorError returns red text
func ColorError(text string) string {
	return colorRed + text + colorReset
}

// ColorWarning returns yellow text
func ColorWarning(text string) string {
	return colorYellow + text + colorReset
}

// ColorInfo returns cyan text
func ColorInfo(text string) string {
	return colorCyan + text + colorReset
}

// ColorBold returns bold text
func ColorBold(text string) string {
	return colorBold + text + colorReset
}

// Gradient applies a rainbow gradient to text
func Gradient(text string) string {
	if len(text) == 0 {
		return text
	}

	runes := []rune(text)
	var result strings.Builder

	for i, r := range runes {
		colorIdx := i % len(gradientColors)
		result.WriteString(gradientColors[colorIdx])
		result.WriteRune(r)
	}
	result.WriteString(colorReset)

	return result.String()
}

// IterationBanner creates a colorful banner for iteration headers
func IterationBanner(n int, timeStr string) string {
	content := fmt.Sprintf(" Iteration %d (%s) ", n, timeStr)

	// Calculate padding for centering
	totalWidth := 40
	contentLen := len([]rune(content))
	leftPad := (totalWidth - contentLen - 2) / 2
	rightPad := totalWidth - contentLen - 2 - leftPad

	// Build the banner with Unicode box-drawing characters
	top := "╔" + strings.Repeat("═", totalWidth-2) + "╗"
	middle := "║" + strings.Repeat(" ", leftPad) + content + strings.Repeat(" ", rightPad) + "║"
	bottom := "╚" + strings.Repeat("═", totalWidth-2) + "╝"

	// Apply gradient to the entire banner
	return fmt.Sprintf("\n%s\n%s\n%s\n",
		Gradient(top),
		Gradient(middle),
		Gradient(bottom))
}
