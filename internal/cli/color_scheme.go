package cli

import (
	"image/color"

	fang "charm.land/fang/v2"
	"charm.land/lipgloss/v2"

	"github.com/alxxpersonal/stardust/internal/ui"
)

// --- Cosmic colorscheme ---

// ErrorRed is the cosmic-red used for the fang error header. It is on-brand
// (a pink-leaning red) yet reads clearly as a failure, distinct from the pink
// accent that titles and flags use.
var ErrorRed color.Color = lipgloss.Color("#ff6b8a")

// cosmicColorScheme maps the Stardust cosmic palette onto fang's chrome. The
// theme is dark only, so the lipgloss.LightDarkFunc argument is ignored.
func cosmicColorScheme(lipgloss.LightDarkFunc) fang.ColorScheme {
	return fang.ColorScheme{
		Base:           ui.Text,      // silver-white body text
		Title:          ui.Primary,   // violet title
		Description:    ui.Text,      // flag and command descriptions
		Codeblock:      ui.CodeBg,    // code background
		Program:        ui.Primary,   // program name, violet
		Command:        ui.Accent,    // subcommand names, pink
		DimmedArgument: ui.Muted,     // dimmed args
		Comment:        ui.Muted,     // comments
		Flag:           ui.Accent,    // flag names, pink
		FlagDefault:    ui.Secondary, // flag defaults, light violet
		Argument:       ui.Text,      // positional args
		QuotedString:   ui.Secondary, // quoted strings
		Help:           ui.Secondary, // help hint
		Dash:           ui.Muted,     // flag dashes
		// near-black foreground on the cosmic-red header so an error reads as a
		// distinct on-brand failure, not the pink accent used by titles.
		ErrorHeader:  [2]color.Color{ui.Bg, ErrorRed},
		ErrorDetails: ui.Accent, // error body, pink
	}
}
