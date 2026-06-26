// Package ui holds the Stardust cosmic palette as a single exported source of
// truth. The TUI, the glamour renderer, and the fang CLI colorscheme all import
// these tokens so the violet/silver-white identity never drifts between copies.
//
// It is a leaf package: its only dependency is charm.land/lipgloss/v2. Nothing
// in the Stardust tree is imported here, so any layer can use it without a cycle.
package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// The locked cosmic palette as canonical hex strings. The glamour renderer needs
// hex strings for its ansi.StyleConfig, so the hexes are the single source and
// the color.Color tokens below are derived from them.
const (
	// PrimaryHex is the violet used for titles and the program name.
	PrimaryHex = "#a78bfa"
	// SecondaryHex is the light violet used for flag defaults and the help hint.
	SecondaryHex = "#c4b5fd"
	// AccentHex is the pink used for commands, flags, and error details.
	AccentHex = "#f0abfc"
	// TextHex is the silver-white body, argument, and description color.
	TextHex = "#e9e7ff"
	// MutedHex is the muted gray-violet for comments, dimmed args, and dashes.
	MutedHex = "#7c7ca0"
	// BorderHex is the dark violet border color.
	BorderHex = "#4c4c6d"
	// CodeBgHex is the code-block background.
	CodeBgHex = "#16161e"
	// BgHex is the near-black background, used as the error-header foreground.
	BgHex = "#0a0a12"
)

// The cosmic palette as color.Color tokens derived from the hexes above. Each
// slots directly into both lipgloss styles and fang.ColorScheme fields.
var (
	// Primary is the violet used for titles and the program name.
	Primary color.Color = lipgloss.Color(PrimaryHex)
	// Secondary is the light violet used for flag defaults and the help hint.
	Secondary color.Color = lipgloss.Color(SecondaryHex)
	// Accent is the pink used for commands, flags, and error details.
	Accent color.Color = lipgloss.Color(AccentHex)
	// Text is the silver-white body, argument, and description color.
	Text color.Color = lipgloss.Color(TextHex)
	// Muted is the muted gray-violet for comments, dimmed args, and dashes.
	Muted color.Color = lipgloss.Color(MutedHex)
	// Border is the dark violet border color.
	Border color.Color = lipgloss.Color(BorderHex)
	// CodeBg is the code-block background.
	CodeBg color.Color = lipgloss.Color(CodeBgHex)
	// Bg is the near-black background, used as the error-header foreground.
	Bg color.Color = lipgloss.Color(BgHex)
)
