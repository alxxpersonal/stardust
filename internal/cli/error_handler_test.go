package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	fang "charm.land/fang/v2"

	"github.com/alxxpersonal/stardust/internal/clierr"
)

// TestErrorHandlerRendersRunLineForHint asserts a hint with a suggestion renders
// the message and a distinct Run: line carrying the command.
func TestErrorHandlerRendersRunLineForHint(t *testing.T) {
	var buf bytes.Buffer
	hint := clierr.New("no stardust vault found here", "stardust init")

	cosmicErrorHandler(&buf, fang.Styles{}, hint)

	out := buf.String()
	if !strings.Contains(out, "no stardust vault found here") {
		t.Errorf("output missing message, got %q", out)
	}
	if !strings.Contains(out, "Run:") {
		t.Errorf("output missing Run: label, got %q", out)
	}
	if !strings.Contains(out, "stardust init") {
		t.Errorf("output missing suggestion command, got %q", out)
	}
}

// TestErrorHandlerNoRunLineForEmptySuggestion asserts a hint with no suggestion
// renders the message but no Run: line.
func TestErrorHandlerNoRunLineForEmptySuggestion(t *testing.T) {
	var buf bytes.Buffer
	hint := clierr.New("unsupported sync scope \"bogus\"", "")

	cosmicErrorHandler(&buf, fang.Styles{}, hint)

	out := buf.String()
	if !strings.Contains(out, "unsupported sync scope") {
		t.Errorf("output missing message, got %q", out)
	}
	if strings.Contains(out, "Run:") {
		t.Errorf("output should not carry a Run: line, got %q", out)
	}
}

// TestErrorHandlerPlainErrorMatchesDefault asserts a plain error falls through to
// fang's default handler byte-for-byte.
func TestErrorHandlerPlainErrorMatchesDefault(t *testing.T) {
	err := errors.New("plain boom")

	var got, want bytes.Buffer
	cosmicErrorHandler(&got, fang.Styles{}, err)
	fang.DefaultErrorHandler(&want, fang.Styles{}, err)

	if got.String() != want.String() {
		t.Errorf("plain error handling diverged from default:\n got %q\nwant %q", got.String(), want.String())
	}
}
