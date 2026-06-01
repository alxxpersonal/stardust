package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-isatty"

	"github.com/alxxpersonal/stardust/internal/render"
)

// isTTY reports whether stdout is an interactive terminal.
func isTTY() bool {
	fd := os.Stdout.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

// emitMarkdown writes md to w honoring the output mode. In auto mode it renders
// via glamour on a TTY and prints plain markdown when piped (the agent surface).
// md and plain always print raw markdown.
func emitMarkdown(w io.Writer, md, mode string) {
	if mode == "auto" && isTTY() {
		fmt.Fprint(w, render.GlamourRender(md, render.TermWidth()))
		return
	}
	fmt.Fprintln(w, strings.TrimRight(md, "\n"))
}

// emitJSON writes v to w as indented JSON.
func emitJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}
