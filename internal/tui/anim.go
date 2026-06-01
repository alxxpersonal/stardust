package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// tickMsg drives the banner shimmer at a slow cadence.
type tickMsg time.Time

// animTick schedules the next animation frame.
func animTick() tea.Cmd {
	return tea.Tick(140*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
