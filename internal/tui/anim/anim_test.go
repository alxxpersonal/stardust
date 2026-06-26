package anim

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"

	"github.com/alxxpersonal/stardust/internal/tui/components"
)

// fillStub and emptyStub are plain styles the bar tests render through so the
// assertions key on the cell glyphs, not on a specific color.
var (
	fillStub  = lipgloss.NewStyle().Foreground(lipgloss.Color("#34d399"))
	emptyStub = lipgloss.NewStyle().Foreground(ColorMuted)
)

// --- Ramp Tests ---

func TestAnimRampsPrecomputed(t *testing.T) {
	assert.Len(t, MonoShimmerRamp, 60, "shimmer ramp should hold 60 blended stops")
	assert.Len(t, BannerSweepRamp, 16, "banner sweep ramp should hold 16 blended stops")
	assert.Len(t, BorderStops, 4, "border rotation should hold 4 stops")
}

// --- Breathe Tests ---

func TestBreatheReturnsBlendedColor(t *testing.T) {
	c := Breathe(0, "#202020", "#e0e0e0")
	assert.NotNil(t, c)
	// Different phases of the sine must produce different colors.
	other := Breathe(11, "#202020", "#e0e0e0")
	assert.NotEqual(t, c, other, "breathe should oscillate across frames")
}

func TestBreatheDeterministicPerFrame(t *testing.T) {
	assert.Equal(t, Breathe(7, "#202020", "#e0e0e0"), Breathe(7, "#202020", "#e0e0e0"))
}

// --- Shimmer Tests ---

func TestShimmerPreservesRuneContent(t *testing.T) {
	out := components.SanitizeText(Shimmer("Scanning sources", 3))
	assert.Equal(t, "Scanning sources", out, "shimmer must restyle, never rewrite, the text")
}

func TestShimmerEmptyInput(t *testing.T) {
	assert.Empty(t, Shimmer("", 5))
}

func TestShimmerShiftsAcrossFrames(t *testing.T) {
	assert.NotEqual(t, Shimmer("pulse", 0), Shimmer("pulse", 9),
		"the wave offset should move across frames")
}

func TestShimmerOutputContainsANSI(t *testing.T) {
	out := Shimmer("x", 0)
	assert.True(t, strings.Contains(out, "\x1b["), "shimmer should emit styled output")
}

// --- Scramble Tests ---

func TestCharScrambleRevealsFullyAtTotal(t *testing.T) {
	final := "Research Engineer"
	assert.Equal(t, final, CharScramble(final, 25, 25))
	assert.Equal(t, final, CharScramble(final, 30, 25))
}

func TestCharScramblePartialMidway(t *testing.T) {
	final := "Research Engineer"
	mid := CharScramble(final, 12, 25)
	assert.Len(t, []rune(mid), len([]rune(final)), "scramble must keep the rune length")
	assert.NotEqual(t, final, mid, "midway frames should still hold scrambled runes")
}

func TestCharScramblePreservesSpaces(t *testing.T) {
	out := CharScramble("a b", 0, 25)
	assert.Equal(t, ' ', []rune(out)[1], "spaces are never scrambled")
}

func TestCharScrambleDeterministic(t *testing.T) {
	assert.Equal(t, CharScramble("Globex", 5, 25), CharScramble("Globex", 5, 25))
}

// --- Border Stop Tests ---

func TestAnimatedBorderStopsRotate(t *testing.T) {
	a0, b0, c0 := AnimatedBorderStops(0)
	a8, _, _ := AnimatedBorderStops(8)
	assert.NotNil(t, a0)
	assert.NotNil(t, b0)
	assert.NotNil(t, c0)
	assert.NotEqual(t, a0, a8, "stops should rotate every eight frames")
}

// --- Hex Utility Tests ---

func TestHexRoundTrip(t *testing.T) {
	r, g, b := HexToRGB("#d77757")
	assert.Equal(t, "#d77757", RGBToHex(r, g, b))
}

func TestHexToRGBMalformed(t *testing.T) {
	r, g, b := HexToRGB("nope")
	assert.True(t, r == 0 && g == 0 && b == 0, "malformed hex decodes to zeros")
}

// --- Bar Tests ---

func TestHBarFillsAndClamps(t *testing.T) {
	out := components.SanitizeText(HBar(10, 4, fillStub, emptyStub))
	assert.Equal(t, "████░░░░░░", out, "bar should render filled then empty cells to width")

	over := components.SanitizeText(HBar(10, 99, fillStub, emptyStub))
	assert.Equal(t, "██████████", over, "filled clamps to width")

	under := components.SanitizeText(HBar(10, -3, fillStub, emptyStub))
	assert.Equal(t, "░░░░░░░░░░", under, "negative filled clamps to zero")

	assert.Empty(t, HBar(0, 5, fillStub, emptyStub), "non-positive width yields empty")
}
