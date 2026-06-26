package cli

import (
	"errors"
	"fmt"
	"io"

	fang "charm.land/fang/v2"
	"charm.land/lipgloss/v2"

	"github.com/alxxpersonal/stardust/internal/clierr"
	"github.com/alxxpersonal/stardust/internal/ui"
)

// --- Error handler ---

// runLabel styles the muted "Run:" prefix on a hint's suggestion line.
var runLabel = lipgloss.NewStyle().Foreground(ui.Muted)

// suggestionStyle styles the runnable command on a hint's suggestion line in the
// pink cosmic accent so the next action stands out.
var suggestionStyle = lipgloss.NewStyle().Foreground(ui.Accent).Bold(true)

// cosmicErrorHandler renders a clierr.Hint as a clean message plus a highlighted
// Run: line carrying the suggested command. Plain errors fall through to fang's
// default handler unchanged. The writer is routed through colorprofile by fang,
// so styled output auto-disables on a non-tty and piped errors stay plain.
func cosmicErrorHandler(w io.Writer, styles fang.Styles, err error) {
	var h *clierr.Hint
	if errors.As(err, &h) {
		fmt.Fprintln(w, styles.ErrorText.Render(h.Message))
		if h.Suggestion != "" {
			fmt.Fprintln(w)
			fmt.Fprintln(w, runLabel.Render("Run:")+"  "+suggestionStyle.Render(h.Suggestion))
		}
		return
	}
	fang.DefaultErrorHandler(w, styles, err)
}
