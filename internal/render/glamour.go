// Package render turns markdown into terminal output: a glamour renderer with
// the Stardust cosmic palette (cached per width) plus terminal-width detection.
// Shared by the CLI's auto output mode and the TUI so both look identical.
package render

import (
	"os"
	"sync"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
	"golang.org/x/term"

	"github.com/alxxpersonal/stardust/internal/ui"
)

// Stardust cosmic/violet palette, sourced from the shared internal/ui leaf package.
const (
	clrPrimary   = ui.PrimaryHex
	clrSecondary = ui.SecondaryHex
	clrAccent    = ui.AccentHex
	clrText      = ui.TextHex
	clrMuted     = ui.MutedHex
	clrBorder    = ui.BorderHex
	clrCodeBg    = ui.CodeBgHex
)

// stardustStyle returns the glamour ansi.StyleConfig for the cosmic palette so
// rendered markdown blends with the rest of the TUI. Every block markdown
// element (headings, fenced code with syntax highlighting, inline code, lists,
// links, blockquotes, tables, images) is themed, mirroring the polish of the
// exo-jobs markdown style but recolored to the violet/pink cosmic identity.
func stardustStyle() ansi.StyleConfig {
	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Color: ptr(clrText)},
			Margin:         ptr(uint(0)),
		},
		Paragraph: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Color: ptr(clrText)},
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Color: ptr(clrSecondary), Bold: ptr(true), BlockSuffix: "\n"},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           ptr(ui.BgHex),
				BackgroundColor: ptr(clrPrimary),
				Bold:            ptr(true),
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Color: ptr(clrPrimary), Bold: ptr(true)},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Color: ptr(clrAccent), Bold: ptr(true)},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Color: ptr(clrSecondary)},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Color: ptr(clrMuted)},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Color: ptr(clrMuted), Faint: ptr(true)},
		},
		Text:          ansi.StylePrimitive{Color: ptr(clrText)},
		Strong:        ansi.StylePrimitive{Color: ptr(clrPrimary), Bold: ptr(true)},
		Emph:          ansi.StylePrimitive{Color: ptr(clrSecondary), Italic: ptr(true)},
		Strikethrough: ansi.StylePrimitive{CrossedOut: ptr(true), Color: ptr(clrMuted)},
		HorizontalRule: ansi.StylePrimitive{
			Color:  ptr(clrBorder),
			Format: "\n──────────\n",
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Color: ptr(clrMuted), Italic: ptr(true)},
			Indent:         ptr(uint(1)),
			IndentToken:    ptr("│ "),
		},
		List: ansi.StyleList{
			LevelIndent: 2,
			StyleBlock:  ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Color: ptr(clrText)}},
		},
		Item:        ansi.StylePrimitive{BlockPrefix: "✧ ", Color: ptr(clrAccent)},
		Enumeration: ansi.StylePrimitive{BlockPrefix: ". ", Color: ptr(clrPrimary)},
		Task: ansi.StyleTask{
			StylePrimitive: ansi.StylePrimitive{Color: ptr(clrText)},
			Ticked:         "[x] ",
			Unticked:       "[ ] ",
		},
		Link:      ansi.StylePrimitive{Color: ptr(clrAccent), Underline: ptr(true)},
		LinkText:  ansi.StylePrimitive{Color: ptr(clrSecondary), Bold: ptr(true)},
		Image:     ansi.StylePrimitive{Color: ptr(clrAccent), Underline: ptr(true)},
		ImageText: ansi.StylePrimitive{Color: ptr(clrMuted), Format: "Image: {{.text}} →"},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           ptr(clrSecondary),
				BackgroundColor: ptr(clrCodeBg),
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{Color: ptr(clrText), BackgroundColor: ptr(clrCodeBg)},
				Margin:         ptr(uint(2)),
			},
			Chroma: stardustChroma(),
		},
		Table: ansi.StyleTable{
			StyleBlock:      ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Color: ptr(clrText)}},
			CenterSeparator: ptr("┼"),
			ColumnSeparator: ptr("│"),
			RowSeparator:    ptr("─"),
		},
		DefinitionList: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Color: ptr(clrText)},
		},
		DefinitionTerm:        ansi.StylePrimitive{Color: ptr(clrPrimary), Bold: ptr(true)},
		DefinitionDescription: ansi.StylePrimitive{Color: ptr(clrText), BlockPrefix: "\n✧ "},
	}
}

// stardustChroma returns the syntax-highlighting theme for fenced code blocks,
// recoloring chroma's token classes onto the cosmic palette so highlighted code
// stays legible on the dark code background instead of glamour's default blues.
func stardustChroma() *ansi.Chroma {
	return &ansi.Chroma{
		Text:                ansi.StylePrimitive{Color: ptr(clrText)},
		Error:               ansi.StylePrimitive{Color: ptr(ui.BgHex), BackgroundColor: ptr(clrAccent)},
		Comment:             ansi.StylePrimitive{Color: ptr(clrMuted), Italic: ptr(true)},
		CommentPreproc:      ansi.StylePrimitive{Color: ptr(clrAccent)},
		Keyword:             ansi.StylePrimitive{Color: ptr(clrPrimary), Bold: ptr(true)},
		KeywordReserved:     ansi.StylePrimitive{Color: ptr(clrPrimary), Bold: ptr(true)},
		KeywordNamespace:    ansi.StylePrimitive{Color: ptr(clrAccent)},
		KeywordType:         ansi.StylePrimitive{Color: ptr(clrSecondary)},
		Operator:            ansi.StylePrimitive{Color: ptr(clrSecondary)},
		Punctuation:         ansi.StylePrimitive{Color: ptr(clrMuted)},
		Name:                ansi.StylePrimitive{Color: ptr(clrText)},
		NameBuiltin:         ansi.StylePrimitive{Color: ptr(clrAccent)},
		NameTag:             ansi.StylePrimitive{Color: ptr(clrPrimary)},
		NameAttribute:       ansi.StylePrimitive{Color: ptr(clrSecondary)},
		NameClass:           ansi.StylePrimitive{Color: ptr(clrPrimary), Bold: ptr(true), Underline: ptr(true)},
		NameConstant:        ansi.StylePrimitive{Color: ptr(clrAccent)},
		NameDecorator:       ansi.StylePrimitive{Color: ptr(clrAccent)},
		NameFunction:        ansi.StylePrimitive{Color: ptr(clrSecondary)},
		LiteralNumber:       ansi.StylePrimitive{Color: ptr(clrAccent)},
		LiteralString:       ansi.StylePrimitive{Color: ptr(clrSecondary)},
		LiteralStringEscape: ansi.StylePrimitive{Color: ptr(clrAccent)},
		GenericDeleted:      ansi.StylePrimitive{Color: ptr(clrAccent)},
		GenericEmph:         ansi.StylePrimitive{Italic: ptr(true)},
		GenericInserted:     ansi.StylePrimitive{Color: ptr(clrSecondary)},
		GenericStrong:       ansi.StylePrimitive{Bold: ptr(true)},
		GenericSubheading:   ansi.StylePrimitive{Color: ptr(clrMuted)},
		Background:          ansi.StylePrimitive{BackgroundColor: ptr(clrCodeBg)},
	}
}

// --- Renderer cache ---

var (
	rendererMu   sync.Mutex
	cachedWidth  int
	cachedRender *glamour.TermRenderer
)

// GlamourRender renders markdown with the Stardust style at the given word-wrap
// width. The renderer is cached per width since rebuilding it every call is waste.
func GlamourRender(md string, width int) string {
	if width < 20 {
		width = 20
	}
	rendererMu.Lock()
	if cachedRender == nil || cachedWidth != width {
		r, err := glamour.NewTermRenderer(
			glamour.WithStyles(stardustStyle()),
			glamour.WithWordWrap(width),
		)
		if err != nil {
			rendererMu.Unlock()
			return md
		}
		cachedRender = r
		cachedWidth = width
	}
	r := cachedRender
	rendererMu.Unlock()

	out, err := r.Render(md)
	if err != nil {
		return md
	}
	return out
}

// TermWidth returns stdout's width capped to a readable maximum, or 100 when
// stdout is not a terminal.
func TermWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 100
	}
	if w > 120 {
		return 120
	}
	return w
}

func ptr[T any](v T) *T { return &v }
