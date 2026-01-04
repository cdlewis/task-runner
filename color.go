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
	content := fmt.Sprintf("✦ Iteration %d (%s) ✦", n, timeStr)

	// Calculate padding for centering
	totalWidth := 40
	contentLen := len([]rune(content))
	leftPad := (totalWidth - contentLen - 2) / 2
	rightPad := totalWidth - contentLen - 2 - leftPad

	// Build the banner with Unicode box-drawing characters
	top := "╔" + strings.Repeat("═", totalWidth-2) + "╗"
	bottom := "╚" + strings.Repeat("═", totalWidth-2) + "╝"

	// Cyan border, bold white text
	middleFormatted := colorCyan + "║" + colorReset +
		strings.Repeat(" ", leftPad) +
		colorBold + content + colorReset +
		strings.Repeat(" ", rightPad) +
		colorCyan + "║" + colorReset

	return fmt.Sprintf("\n%s%s%s\n%s\n%s%s%s\n",
		colorCyan, top, colorReset,
		middleFormatted,
		colorCyan, bottom, colorReset)
}

// displayWidth calculates the visual width of a string
// Full-width characters (CJK, full-width punctuation) count as 2 columns
func displayWidth(s string) int {
	width := 0
	for _, r := range s {
		if r >= 0x1100 && // Korean Hangul
			(r <= 0x115F || // Hangul Jamo
				(r >= 0x2E80 && r <= 0x9FFF) || // CJK
				(r >= 0xAC00 && r <= 0xD7A3) || // Hangul Syllables
				(r >= 0xF900 && r <= 0xFAFF) || // CJK Compatibility
				(r >= 0xFE10 && r <= 0xFE1F) || // Vertical forms
				(r >= 0xFE30 && r <= 0xFE6F) || // CJK Compatibility Forms
				(r >= 0xFF00 && r <= 0xFF60) || // Full-width forms
				(r >= 0xFFE0 && r <= 0xFFE6)) { // Full-width symbols
			width += 2
		} else {
			width += 1
		}
	}
	return width
}

// StartupBanner creates the startup banner with cat ASCII art
func StartupBanner(taskName, logPath, mode string) string {
	cat := []string{
		"　　　　　   __",
		"　　　　 ／フ   フ",
		"　　　　|  .   .|",
		"　 　　／`ミ__xノ",
		"　 　 /　　 　 |",
		"　　 /　 ヽ　　ﾉ",
		" 　 │　　 | | |",
		"／￣|　　 | | |",
		"| (￣ヽ_ヽ)_)__)",
		"＼二つ",
	}

	// Find the widest line
	maxWidth := 0
	for _, line := range cat {
		if w := displayWidth(line); w > maxWidth {
			maxWidth = w
		}
	}

	// Build labels map: line index -> label
	labels := map[int]string{
		2: ColorBold("Nigel"),
		// line 3 is empty
		4: "Task: " + taskName,
		5: "Logs: " + logPath,
		6: "Mode: " + mode,
	}

	// Remove logs line if no path provided
	if logPath == "" {
		delete(labels, 5)
	}

	var result strings.Builder
	result.WriteString("\n")

	for i, line := range cat {
		result.WriteString(colorCyan)
		result.WriteString(line)
		result.WriteString(colorReset)

		if label, ok := labels[i]; ok {
			// Pad to align labels
			padding := maxWidth - displayWidth(line) + 3
			result.WriteString(strings.Repeat(" ", padding))
			result.WriteString(label)
		}
		result.WriteString("\n")
	}

	return result.String()
}
