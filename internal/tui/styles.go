// Package tui is the interactive multi-tab terminal UI: a search tab over the
// hybrid index, a status tab, and a graph tab. It shares the render package's
// glamour styling with the CLI so both surfaces look identical.
package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Stardust cosmic palette, mirroring internal/render.
var (
	colorPrimary   = lipgloss.Color("#a78bfa")
	colorSecondary = lipgloss.Color("#c4b5fd")
	colorAccent    = lipgloss.Color("#f0abfc")
	colorText      = lipgloss.Color("#e9e7ff")
	colorMuted     = lipgloss.Color("#7c7ca0")
	colorBorder    = lipgloss.Color("#4c4c6d")
	colorBg        = lipgloss.Color("#0a0a12")
)

var (
	titleStyle  = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	mutedStyle  = lipgloss.NewStyle().Foreground(colorMuted)
	textStyle   = lipgloss.NewStyle().Foreground(colorText)
	accentStyle = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	boxStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorBorder).Padding(0, 1)
	pillStyle   = lipgloss.NewStyle().Foreground(colorBg).Background(colorPrimary).Bold(true).Padding(0, 1)
)

// hint renders a keycap pill followed by its label for the status bar.
func hint(key, label string) string {
	return pillStyle.Render(key) + " " + mutedStyle.Render(label)
}

// statusBar joins hints into a single muted bar of the given width.
func statusBar(hints []string, width int) string {
	return lipgloss.NewStyle().Width(width).Render(strings.Join(hints, "   "))
}

// truncate clips s to n display columns, adding an ellipsis when shortened.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

// clipLines keeps at most n lines of s, dropping the rest.
func clipLines(s string, n int) string {
	if n <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n")
}
