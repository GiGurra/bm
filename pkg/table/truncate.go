package table

import (
	"path"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"
)

// StringWidth returns the display width of a string in terminal cells.
// Accounts for wide characters (CJK, emojis, etc.) AND ANSI escape codes.
func StringWidth(s string) int {
	return lipgloss.Width(s)
}

// TruncateWithEllipsis truncates a string to fit within maxWidth display cells,
// adding "…" if truncated. Handles wide characters correctly.
func TruncateWithEllipsis(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}

	width := lipgloss.Width(s)
	if width <= maxWidth {
		return s
	}

	if maxWidth == 1 {
		return "…"
	}

	// Truncate by iterating runes and tracking display width
	result := make([]rune, 0, len(s))
	currentWidth := 0
	targetWidth := maxWidth - 1 // Reserve 1 cell for ellipsis

	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if currentWidth+rw > targetWidth {
			break
		}
		result = append(result, r)
		currentWidth += rw
	}

	return string(result) + "…"
}

// TruncateFromStart truncates a string from the beginning, keeping the end.
// Adds "…" prefix if truncated. Useful for paths where the end is more relevant.
func TruncateFromStart(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}

	width := lipgloss.Width(s)
	if width <= maxWidth {
		return s
	}

	if maxWidth == 1 {
		return "…"
	}

	// We need to keep the END of the string, so work backwards
	runes := []rune(s)
	result := make([]rune, 0, len(runes))
	currentWidth := 0
	targetWidth := maxWidth - 1 // Reserve 1 cell for ellipsis

	// Iterate from end to beginning
	for i := len(runes) - 1; i >= 0; i-- {
		r := runes[i]
		rw := runewidth.RuneWidth(r)
		if currentWidth+rw > targetWidth {
			break
		}
		result = append([]rune{r}, result...)
		currentWidth += rw
	}

	return "…" + string(result)
}

// ShortenPath shortens a file path to fit within maxWidth display cells.
// It prioritizes keeping the filename visible and shortens directory components.
func ShortenPath(p string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}

	// Normalize path separators to forward slashes
	p = filepath.ToSlash(p)

	if lipgloss.Width(p) <= maxWidth {
		return p
	}

	// Use path package (not filepath) since we've normalized to forward slashes.
	// filepath.Base/Dir use OS-specific separators which breaks on Windows.
	filename := path.Base(p)
	filenameWidth := lipgloss.Width(filename)

	// If filename fits, try to add some path context
	if filenameWidth <= maxWidth {
		dir := path.Dir(p)
		if dir == "." || dir == "/" {
			return filename
		}

		// Calculate available space for directory prefix
		// Format: "…/dirname/filename" or just "…/filename"
		available := maxWidth - filenameWidth - 2 // -2 for "…/"

		if available <= 0 {
			return filename
		}

		// Try to fit last directory component
		parts := strings.Split(dir, "/")
		lastDir := parts[len(parts)-1]
		lastDirWidth := lipgloss.Width(lastDir)

		if lastDirWidth+1 <= available { // +1 for "/"
			return "…/" + lastDir + "/" + filename
		}

		// Just use ellipsis prefix
		return "…/" + filename
	}

	// Filename is too long, truncate it
	return TruncateWithEllipsis(filename, maxWidth)
}

// ScrollWindow returns a sliding window view of a string for scroll animation.
// At offset 0, truncates from end (like TruncateWithEllipsis).
// At offset > 0, shows content starting from that display-cell offset with ellipsis indicators.
func ScrollWindow(s string, maxWidth int, offset int) string {
	if maxWidth <= 0 {
		return ""
	}

	totalWidth := lipgloss.Width(s)
	if totalWidth <= maxWidth {
		return s
	}

	if offset <= 0 {
		return TruncateWithEllipsis(s, maxWidth)
	}

	// For very small widths, fall back to simple truncation
	if maxWidth < 4 {
		return TruncateFromStart(s, maxWidth)
	}

	runes := []rune(s)

	// Skip offset display cells from the start
	skipped := 0
	startIdx := len(runes)
	for i, r := range runes {
		if skipped >= offset {
			startIdx = i
			break
		}
		skipped += runewidth.RuneWidth(r)
	}

	if startIdx >= len(runes) {
		return TruncateFromStart(s, maxWidth)
	}

	// Reserve 1 cell for left ellipsis "…"
	contentWidth := maxWidth - 1

	// Check if remaining content fits
	result := make([]rune, 0)
	currentWidth := 0
	fitsEntirely := true
	for i := startIdx; i < len(runes); i++ {
		rw := runewidth.RuneWidth(runes[i])
		if currentWidth+rw > contentWidth {
			fitsEntirely = false
			break
		}
		result = append(result, runes[i])
		currentWidth += rw
	}

	if fitsEntirely {
		return "…" + string(result)
	}

	// Need right ellipsis too — rebuild with smaller content area
	contentWidth = maxWidth - 2 // 1 for left "…", 1 for right "…"
	result = result[:0]
	currentWidth = 0
	for i := startIdx; i < len(runes); i++ {
		rw := runewidth.RuneWidth(runes[i])
		if currentWidth+rw > contentWidth {
			break
		}
		result = append(result, runes[i])
		currentWidth += rw
	}
	return "…" + string(result) + "…"
}

// MaxScrollOffset returns the maximum scroll offset (in display cells) for ScrollWindow.
// Returns 0 if the string fits within maxWidth.
func MaxScrollOffset(s string, maxWidth int) int {
	totalWidth := lipgloss.Width(s)
	if totalWidth <= maxWidth {
		return 0
	}
	// When scrolled, left ellipsis uses 1 cell, so at max offset
	// we show "…" + last (maxWidth-1) cells of content
	maxOffset := totalWidth - (maxWidth - 1)
	if maxOffset < 0 {
		return 0
	}
	return maxOffset
}

// PadRight pads a string to the specified display width with spaces on the right.
// If the string is wider than width, it is truncated.
func PadRight(s string, width int) string {
	if width <= 0 {
		return ""
	}

	strWidth := lipgloss.Width(s)
	if strWidth >= width {
		return TruncateWithEllipsis(s, width)
	}

	return s + strings.Repeat(" ", width-strWidth)
}

// PadLeft pads a string to the specified display width with spaces on the left.
// If the string is wider than width, it is truncated.
func PadLeft(s string, width int) string {
	if width <= 0 {
		return ""
	}

	strWidth := lipgloss.Width(s)
	if strWidth >= width {
		return TruncateWithEllipsis(s, width)
	}

	return strings.Repeat(" ", width-strWidth) + s
}

// PadCenter centers a string within the specified display width.
// If the string is wider than width, it is truncated.
func PadCenter(s string, width int) string {
	if width <= 0 {
		return ""
	}

	strWidth := lipgloss.Width(s)
	if strWidth >= width {
		return TruncateWithEllipsis(s, width)
	}

	totalPadding := width - strWidth
	leftPadding := totalPadding / 2
	rightPadding := totalPadding - leftPadding

	return strings.Repeat(" ", leftPadding) + s + strings.Repeat(" ", rightPadding)
}
