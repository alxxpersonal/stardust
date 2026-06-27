// Package tui is the interactive multi-tab terminal UI.
package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/alxxpersonal/stardust/internal/tui/components"
	"github.com/alxxpersonal/stardust/internal/ui"
)

// --- Theme Colors ---

var (
	ColorPrimary    = ui.Primary
	ColorSecondary  = ui.Secondary
	ColorAccent     = ui.Accent
	ColorBackground = ui.Bg
	ColorText       = ui.Text
	ColorMuted      = ui.Muted
	ColorSuccess    = lipgloss.Color("#34d399")
	ColorError      = lipgloss.Color("#ff5555")
	ColorWarning    = lipgloss.Color("#f59e0b")
	ColorBorder     = ui.Border
	ColorCard       = ui.CodeBg
	ColorGlow       = ui.Accent
)

// --- Reusable Styles ---

var (
	BannerStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	BannerAccentStyle = lipgloss.NewStyle().
				Foreground(ColorSecondary)

	TabActiveStyle = lipgloss.NewStyle().
			Foreground(ColorBackground).
			Background(ColorPrimary).
			Bold(true).
			Padding(0, 1)

	TabInactiveStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Padding(0, 1)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			PaddingTop(1)

	SelectedStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	NormalStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)

	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	AccentStyle = lipgloss.NewStyle().
			Foreground(ColorAccent)

	CardStyle = lipgloss.NewStyle().
			Foreground(ColorCard)

	HeaderStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true).
			PaddingBottom(1)

	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	TypeBadgeStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Border(lipgloss.NormalBorder()).
			BorderForeground(ColorBorder).
			BorderTop(false).
			BorderBottom(false).
			Bold(true).
			Padding(0, 1)

	DividerStyle = lipgloss.NewStyle().
			Foreground(ColorBorder)

	MetaKeyStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)

	MetaValueStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	MetaPunctStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	ScoreHighStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	ScoreMedStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	ScoreLowStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	GlowStyle = lipgloss.NewStyle().
			Foreground(ColorGlow).
			Bold(true)

	UserMsgStyle = lipgloss.NewStyle().
			Background(ColorCard).
			Foreground(ColorText).
			PaddingLeft(1).
			PaddingRight(1)

	ToolNameStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)

	ToolArgsStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	ThinkingStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true)

	InputPrefixStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Bold(true)

	sectionHeaderStyle = lipgloss.NewStyle().
				Foreground(ColorBackground).
				Background(ColorPrimary).
				Bold(true).
				Padding(0, 1)
)

var (
	pillStyle = lipgloss.NewStyle().
		Foreground(ColorBackground).
		Background(ColorPrimary).
		Bold(true).
		Padding(0, 1)
)

// Divider returns a horizontal line.
func Divider(width int) string {
	if width <= 0 {
		return ""
	}
	return DividerStyle.Render(strings.Repeat("─", width))
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

func commonHints() []components.HintItem {
	return []components.HintItem{
		{Key: "tab", Desc: "tabs"},
		{Key: "1-6", Desc: "jump"},
		{Key: "ctrl+c", Desc: "quit"},
	}
}

func withCommonHints(hints ...components.HintItem) []components.HintItem {
	out := make([]components.HintItem, 0, len(hints)+3)
	out = append(out, hints...)
	out = append(out, commonHints()...)
	return out
}

func boolWord(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
