package anim

import (
	"fmt"
	"math"
	"time"

	tea "charm.land/bubbletea/v2"
)

// flameColors is the cosmic forge gradient: dim violet -> bright pink -> back,
// matching the app's breathe range so the "working" indicator stays on-theme.
var flameColors = []struct{ r, g, b uint8 }{
	{76, 76, 109},   // #4c4c6d dim
	{124, 124, 160}, // #7c7ca0
	{167, 139, 250}, // #a78bfa
	{196, 181, 253}, // #c4b5fd
	{240, 171, 252}, // #f0abfc bright
	{196, 181, 253}, // #c4b5fd
	{167, 139, 250}, // #a78bfa
	{124, 124, 160}, // #7c7ca0
}

// FlameColor returns the current flame color as a #rrggbb string for the given
// frame counter, using sine-wave interpolation for a smooth breathing effect.
func FlameColor(frame int) string {
	// Sine wave oscillation: 0->1->0 over ~2 seconds (16 frames at 120ms).
	t := (math.Sin(float64(frame)*0.4) + 1.0) / 2.0 // 0.0 to 1.0

	// Interpolate across the cosmic flame stops.
	idx := t * float64(len(flameColors)-1)
	lo := int(idx)
	hi := lo + 1
	if hi >= len(flameColors) {
		hi = len(flameColors) - 1
	}
	frac := idx - float64(lo)

	r := uint8(float64(flameColors[lo].r) + frac*float64(int(flameColors[hi].r)-int(flameColors[lo].r)))
	g := uint8(float64(flameColors[lo].g) + frac*float64(int(flameColors[hi].g)-int(flameColors[lo].g)))
	b := uint8(float64(flameColors[lo].b) + frac*float64(int(flameColors[hi].b)-int(flameColors[lo].b)))

	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// FlameTickMsg is sent every 80ms to advance a flame animation. An overlay
// listens for it in its Update to drive its frame counter.
type FlameTickMsg struct{}

// FlameTick returns a Cmd that fires a FlameTickMsg after 80ms.
func FlameTick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return FlameTickMsg{}
	})
}

// flameBlockFrames simulates fire flickering with varying width blocks.
var flameBlockFrames = []rune{'▌', '▍', '▎', '▏', '▎', '▍', '▌', '█', '▊', '▋', '▌'}

// FlameBlock returns the current flame block character and its #rrggbb color for
// the given frame.
func FlameBlock(frame int) (string, string) {
	ch := string(flameBlockFrames[frame%len(flameBlockFrames)])
	color := FlameColor(frame)
	return ch, color
}
