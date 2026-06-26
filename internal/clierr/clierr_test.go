package clierr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/alxxpersonal/stardust/internal/clierr"
	"github.com/alxxpersonal/stardust/internal/config"
)

// TestErrorWithSuggestion asserts a hint with a suggestion renders one inline line.
func TestErrorWithSuggestion(t *testing.T) {
	got := clierr.New("x", "y").Error()
	if got != "x (try: y)" {
		t.Errorf("Error() = %q, want %q", got, "x (try: y)")
	}
}

// TestErrorWithoutSuggestion asserts a hint with no suggestion is just the message.
func TestErrorWithoutSuggestion(t *testing.T) {
	got := clierr.New("x", "").Error()
	if got != "x" {
		t.Errorf("Error() = %q, want %q", got, "x")
	}
}

// TestErrorsAsThroughDoubleWrap asserts a hint is recoverable through two layers
// of %w wrapping so the fang handler's errors.As reaches it.
func TestErrorsAsThroughDoubleWrap(t *testing.T) {
	base := clierr.New("no vault", "stardust init")
	wrapped := fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", base))

	var h *clierr.Hint
	if !errors.As(wrapped, &h) {
		t.Fatalf("errors.As did not extract *clierr.Hint from %v", wrapped)
	}
	if h.Suggestion != "stardust init" {
		t.Errorf("Suggestion = %q, want %q", h.Suggestion, "stardust init")
	}
	if h.Message != "no vault" {
		t.Errorf("Message = %q, want %q", h.Message, "no vault")
	}
}

// TestWrapPreservesSentinel asserts Wrap keeps the wrapped cause reachable by
// errors.Is so config.ErrNoVault stays an errors.Is-checkable sentinel.
func TestWrapPreservesSentinel(t *testing.T) {
	wrapped := clierr.Wrap(config.ErrNoVault, "no stardust vault found here", "stardust init")

	if !errors.Is(wrapped, config.ErrNoVault) {
		t.Errorf("errors.Is(wrapped, config.ErrNoVault) = false, want true")
	}
	var h *clierr.Hint
	if !errors.As(wrapped, &h) {
		t.Fatalf("errors.As did not extract *clierr.Hint")
	}
	if h.Suggestion != "stardust init" {
		t.Errorf("Suggestion = %q, want %q", h.Suggestion, "stardust init")
	}
}

// TestUnwrapNilWhenNoCause asserts a hint built with New has no wrapped cause.
func TestUnwrapNilWhenNoCause(t *testing.T) {
	if cause := clierr.New("x", "y").Unwrap(); cause != nil {
		t.Errorf("Unwrap() = %v, want nil", cause)
	}
}
