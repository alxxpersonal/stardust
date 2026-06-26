package tui

import (
	"charm.land/lipgloss/v2"

	"github.com/alxxpersonal/stardust/internal/tui/anim"
)

// The pure animation, color, border, bar, and flame helpers live in
// internal/tui/anim. What stays here is the tui-specific box composition that
// needs local styles (sectionHeaderStyle); it delegates the rotating border
// gradient to anim.AnimatedBorderStops.

// --- Animated Boxes ---

// animatedBox wraps content in a rounded box whose border gradient rotates
// with the frame. Used for empty states and standalone notices.
func animatedBox(content string, frame int) string {
	a, b, c := anim.AnimatedBorderStops(frame)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForegroundBlend(a, b, c).
		Padding(1, 3).
		Render(content)
}

// animatedTitledBox wraps titled content in a box with the given border whose
// gradient animates with the frame. It sizes to its content; center it at the
// call site (PlaceHorizontal) so it lines up with the cards + xp bar.
func animatedTitledBox(title, content string, frame int, border lipgloss.Border) string {
	a, b, c := anim.AnimatedBorderStops(frame)
	inner := content
	if title != "" {
		inner = sectionHeaderStyle.Render(title) + "\n\n" + content
	}
	return lipgloss.NewStyle().
		Border(border).
		BorderForegroundBlend(a, b, c).
		Padding(1, 2).
		Render(inner)
}

// animatedDoubleBox is a double-bordered titled box.
func animatedDoubleBox(title, content string, frame int) string {
	return animatedTitledBox(title, content, frame, lipgloss.DoubleBorder())
}

// animatedRoundedBox is a single (rounded) bordered titled box.
func animatedRoundedBox(title, content string, frame int) string {
	return animatedTitledBox(title, content, frame, lipgloss.RoundedBorder())
}
