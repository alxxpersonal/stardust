package cli

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/alxxpersonal/stardust/internal/ui"
)

// TestCosmicColorScheme asserts the key fang.ColorScheme fields map onto the
// cosmic palette tokens so the CLI chrome reads violet, not the fang default.
func TestCosmicColorScheme(t *testing.T) {
	cs := cosmicColorScheme(lipgloss.LightDark(true))

	cases := []struct {
		name string
		got  color.Color
		want color.Color
	}{
		{"Title", cs.Title, ui.Primary},
		{"Program", cs.Program, ui.Primary},
		{"Command", cs.Command, ui.Accent},
		{"Flag", cs.Flag, ui.Accent},
		{"Base", cs.Base, ui.Text},
		{"FlagDefault", cs.FlagDefault, ui.Secondary},
		{"Dash", cs.Dash, ui.Muted},
		{"Codeblock", cs.Codeblock, ui.CodeBg},
		{"ErrorDetails", cs.ErrorDetails, ui.Accent},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}

	// ErrorHeader is [fg, bg]: near-black background-color foreground on the
	// cosmic-red header per the decision (reads as a failure, on-brand).
	if cs.ErrorHeader[0] != ui.Bg {
		t.Errorf("ErrorHeader fg = %v, want %v", cs.ErrorHeader[0], ui.Bg)
	}
	if cs.ErrorHeader[1] != ErrorRed {
		t.Errorf("ErrorHeader bg = %v, want %v", cs.ErrorHeader[1], ErrorRed)
	}
}

// TestErrorRedConstant pins the cosmic-red error color so the on-brand failure
// hue cannot drift to the pink accent.
func TestErrorRedConstant(t *testing.T) {
	if ErrorRed != lipgloss.Color("#ff6b8a") {
		t.Errorf("ErrorRed = %v, want %v", ErrorRed, lipgloss.Color("#ff6b8a"))
	}
}
