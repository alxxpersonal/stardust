package ui

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
)

// TestPaletteTokens asserts each cosmic token equals its locked hex so a stray
// edit to the palette is caught immediately.
func TestPaletteTokens(t *testing.T) {
	cases := []struct {
		name string
		got  color.Color
		want color.Color
	}{
		{"Primary", Primary, lipgloss.Color("#a78bfa")},
		{"Secondary", Secondary, lipgloss.Color("#c4b5fd")},
		{"Accent", Accent, lipgloss.Color("#f0abfc")},
		{"Text", Text, lipgloss.Color("#e9e7ff")},
		{"Muted", Muted, lipgloss.Color("#7c7ca0")},
		{"Border", Border, lipgloss.Color("#4c4c6d")},
		{"CodeBg", CodeBg, lipgloss.Color("#16161e")},
		{"Bg", Bg, lipgloss.Color("#0a0a12")},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}
