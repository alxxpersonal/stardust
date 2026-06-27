package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/alxxpersonal/stardust/internal/tui/anim"
	"github.com/alxxpersonal/stardust/internal/ui"
)

var bannerLines = []string{
	"  █████████  ███████████   █████████   ███████████   ██████████   █████  █████  █████████  ███████████",
	" ███░░░░░███░█░░░███░░░█  ███░░░░░███ ░░███░░░░░███ ░░███░░░░███ ░░███  ░░███  ███░░░░░███░█░░░███░░░█",
	"░███    ░░░ ░   ░███  ░  ░███    ░███  ░███    ░███  ░███   ░░███ ░███   ░███ ░███    ░░░ ░   ░███  ░",
	"░░█████████     ░███     ░███████████  ░██████████   ░███    ░███ ░███   ░███ ░░█████████     ░███",
	" ░░░░░░░░███    ░███     ░███░░░░░███  ░███░░░░░███  ░███    ░███ ░███   ░███  ░░░░░░░░███    ░███",
	" ███    ░███    ░███     ░███    ░███  ░███    ░███  ░███    ███  ░███   ░███  ███    ░███    ░███",
	"░░█████████     █████    █████   █████ █████   █████ ██████████   ░░████████  ░░█████████     █████",
	" ░░░░░░░░░     ░░░░░    ░░░░░   ░░░░░ ░░░░░   ░░░░░ ░░░░░░░░░░     ░░░░░░░░    ░░░░░░░░░     ░░░░░",
}

// Gradient from bright top to dim bottom.
var bannerGradient = []string{
	"#f0abfc",
	"#e9b8ff",
	"#d8a0fb",
	"#c4b5fd",
	"#a78bfa",
	"#9478e0",
	"#7c7ca0",
	"#6a6a90",
}

// RenderBannerAnimated returns the banner with a rolling cosmic sweep and
// a breathing subtitle. The sweep shifts one ramp stop every eight frames so
// the gradient slowly rolls down the art; headless output keeps the static
// RenderBanner.
func RenderBannerAnimated(frame int) string {
	rendered := "\n"

	maxW := 0
	for _, line := range bannerLines {
		if w := lipgloss.Width(line); w > maxW {
			maxW = w
		}
	}

	shift := frame / 8
	rampSize := len(anim.BannerSweepRamp)
	for i, line := range bannerLines {
		idx := (i + shift) % rampSize
		style := lipgloss.NewStyle().Foreground(anim.BannerSweepRamp[idx])
		rendered += style.Render(line) + "\n"
	}

	rendered += "\n"
	subtitleText := "Local-First Markdown Context Engine"
	subtitleWidth := lipgloss.Width(subtitleText)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(anim.Breathe(frame, ui.MutedHex, ui.AccentHex)).
		Width(maxW).
		Align(lipgloss.Center)
	rendered += subtitleStyle.Render(subtitleText) + "\n"

	underlineStyle := lipgloss.NewStyle().
		Foreground(ColorBorder).
		Width(maxW).
		Align(lipgloss.Center)
	rendered += underlineStyle.Render(strings.Repeat("─", subtitleWidth)) + "\n"

	return rendered
}

// RenderBanner returns the styled ASCII banner with a vertical cosmic gradient and underline.
func RenderBanner() string {
	rendered := "\n"

	maxW := 0
	for _, line := range bannerLines {
		if w := lipgloss.Width(line); w > maxW {
			maxW = w
		}
	}

	for i, line := range bannerLines {
		color := bannerGradient[i]
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
		rendered += style.Render(line) + "\n"
	}

	rendered += "\n"
	subtitleText := "Local-First Markdown Context Engine"
	subtitleWidth := lipgloss.Width(subtitleText)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Width(maxW).
		Align(lipgloss.Center)
	rendered += subtitleStyle.Render(subtitleText) + "\n"

	underlineStyle := lipgloss.NewStyle().
		Foreground(ColorBorder).
		Width(maxW).
		Align(lipgloss.Center)
	rendered += underlineStyle.Render(strings.Repeat("─", subtitleWidth)) + "\n"

	return rendered
}
