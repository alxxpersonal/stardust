// Package anim holds the pure terminal-animation, color, border, and bar
// helpers shared by the TUI. It lives below internal/tui so the root model and
// tabs can share the same render primitives without an import cycle.
//
// Everything here is pure: deterministic in its inputs, free of any dependency
// back on internal/ui, and byte-for-byte identical to the helpers that
// previously lived in internal/ui. Callers that need ui-specific composition
// (titled boxes, section headers) keep that logic in internal/ui and call into
// these primitives.
package anim

import (
	"image/color"
	"math"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/alxxpersonal/stardust/internal/ui"
)

// --- Shared Colors ---

// ColorMuted is the muted cosmic tone used across the TUI.
var ColorMuted = ui.Muted

// --- Animation Ramps ---

// Precomputed at init so per-frame rendering is pure index math (no RGB
// interpolation in the hot path).
var (
	// MonoShimmerRamp is a symmetric dim-bright-dim cosmic ramp for
	// per-rune shimmer sweeps.
	MonoShimmerRamp []color.Color

	// BannerSweepRamp is a 16-stop cosmic ramp the banner lines roll through.
	BannerSweepRamp []color.Color

	// GreenShimmerRamp is a symmetric emerald ramp a bright mint glint sweeps
	// across - the "shoowsh" used on high scores and live statuses.
	GreenShimmerRamp []color.Color

	// BorderStops are the cosmic colors the animated table border rotates through.
	BorderStops = []color.Color{
		ui.Border,
		ui.Muted,
		ui.Primary,
		ui.Secondary,
	}
)

func init() {
	MonoShimmerRamp = lipgloss.Blend1D(60,
		ui.Border,
		ui.Muted,
		ui.Primary,
		ui.Text,
		ui.Primary,
		ui.Muted,
		ui.Border,
	)
	BannerSweepRamp = lipgloss.Blend1D(16,
		lipgloss.Color("#f0abfc"),
		lipgloss.Color("#e9b8ff"),
		lipgloss.Color("#d8a0fb"),
		lipgloss.Color("#c4b5fd"),
		lipgloss.Color("#a78bfa"),
		lipgloss.Color("#9478e0"),
		lipgloss.Color("#7c7ca0"),
		lipgloss.Color("#6a6a90"),
	)
	GreenShimmerRamp = lipgloss.Blend1D(60,
		lipgloss.Color("#0f5132"),
		lipgloss.Color("#198754"),
		lipgloss.Color("#34d399"),
		lipgloss.Color("#6ee7b7"),
		lipgloss.Color("#d1fae5"),
		lipgloss.Color("#6ee7b7"),
		lipgloss.Color("#34d399"),
		lipgloss.Color("#198754"),
		lipgloss.Color("#0f5132"),
	)
}

// --- Animation Helpers ---

// Breathe returns a color that slowly oscillates between dim and bright hex
// tones on a sine wave (full cycle roughly every 45 frames at 80ms ticks).
func Breathe(frame int, dim, bright string) color.Color {
	t := (math.Sin(float64(frame)*0.14) + 1.0) / 2.0
	dr, dg, db := HexToRGB(dim)
	br, bg, bb := HexToRGB(bright)
	r := uint8(float64(dr) + t*float64(int(br)-int(dr)))
	g := uint8(float64(dg) + t*float64(int(bg)-int(dg)))
	b := uint8(float64(db) + t*float64(int(bb)-int(db)))
	return lipgloss.Color(RGBToHex(r, g, b))
}

// Shimmer renders text with a per-rune grayscale wave that sweeps across it,
// using the shared MonoShimmerRamp.
func Shimmer(text string, frame int) string {
	return ShimmerColor(text, frame, MonoShimmerRamp)
}

// ShimmerColor renders text with a per-rune wave sweeping the given ramp, so a
// bright glint travels across the runes as the frame advances. Returns "" for
// empty text or an empty ramp.
func ShimmerColor(text string, frame int, ramp []color.Color) string {
	runes := []rune(text)
	if len(runes) == 0 || len(ramp) == 0 {
		return ""
	}
	rampSize := len(ramp)
	offset := (frame * 2) % rampSize
	var b strings.Builder
	for i, r := range runes {
		idx := (offset + i) % rampSize
		b.WriteString(lipgloss.NewStyle().
			Foreground(ramp[idx]).
			Render(string(r)))
	}
	return b.String()
}

// ScrambleChars is the glyph pool CharScramble draws unrevealed runes from.
const ScrambleChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*"

// CharScramble returns final with characters progressively revealed left to
// right from scrambled noise. frame counts frames since the reveal started;
// at or past total the final text is returned verbatim. The noise is
// deterministic in (frame, position) so renders are stable per frame.
func CharScramble(final string, frame, total int) string {
	if total <= 0 || frame >= total {
		return final
	}
	runes := []rune(final)
	out := make([]rune, len(runes))
	progress := float64(frame) / float64(total)
	for i, r := range runes {
		if r == ' ' {
			out[i] = ' '
			continue
		}
		if len(runes) > 0 && progress > float64(i)/float64(len(runes)) {
			out[i] = r
			continue
		}
		out[i] = rune(ScrambleChars[(frame*7+i*13)%len(ScrambleChars)])
	}
	return string(out)
}

// AnimatedBorderStops returns three border-gradient stops that rotate through
// the gray palette every eight frames. Feed the triple to
// lipgloss Style.BorderForegroundBlend to render a rotating box border.
func AnimatedBorderStops(frame int) (color.Color, color.Color, color.Color) {
	n := len(BorderStops)
	i := (frame / 8) % n
	return BorderStops[i], BorderStops[(i+1)%n], BorderStops[(i+2)%n]
}

// --- Bar Renderer ---

// HBar renders a single horizontal block bar: a run of filled cells ("█") in
// fillStyle followed by empty cells ("░") in emptyStyle, padding the bar to
// width. filled is clamped to the [0, width] range. width <= 0 yields "". This
// is the shared bar primitive an overlay (HP, charge) and the ledger funnel
// both build on.
func HBar(width, filled int, fillStyle, emptyStyle lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return fillStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("░", width-filled))
}

// --- Color Utilities ---

// ColorToHex renders any color.Color as a #rrggbb string (drops alpha).
func ColorToHex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return RGBToHex(uint8(r>>8&0xff), uint8(g>>8&0xff), uint8(b>>8&0xff))
}

// DimHex scales a #rrggbb color toward black by factor (0..1), e.g. 0.4 yields
// a 40%-brightness floor for a breathe.
func DimHex(hex string, factor float64) string {
	r, g, b := HexToRGB(hex)
	return RGBToHex(uint8(float64(r)*factor), uint8(float64(g)*factor), uint8(float64(b)*factor))
}

// --- Hex Utilities ---

// HexToRGB parses a #rrggbb string into components, returning zeros on
// malformed input.
func HexToRGB(hex string) (uint8, uint8, uint8) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 0, 0, 0
	}
	var out [3]uint8
	for i := range 3 {
		hi, okHi := hexNibble(hex[i*2])
		lo, okLo := hexNibble(hex[i*2+1])
		if !okHi || !okLo {
			return 0, 0, 0
		}
		out[i] = hi<<4 | lo
	}
	return out[0], out[1], out[2]
}

// hexNibble decodes one hex digit, reporting false when invalid.
func hexNibble(c byte) (uint8, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}

// RGBToHex formats components as a #rrggbb string.
func RGBToHex(r, g, b uint8) string {
	const digits = "0123456789abcdef"
	out := []byte{'#', 0, 0, 0, 0, 0, 0}
	for i, v := range []uint8{r, g, b} {
		out[1+i*2] = digits[v>>4]
		out[2+i*2] = digits[v&0x0f]
	}
	return string(out)
}
