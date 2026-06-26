// Package clierr carries actionable CLI errors: a clean problem statement plus
// an optional runnable suggestion the fang error handler renders distinctly.
//
// It is a leaf package with no Stardust imports, so config, cli, hooks, sync,
// registry, and service can all return a Hint without creating an import cycle.
package clierr

// --- Hint ---

// Hint is an actionable error: a clean problem statement plus an optional
// runnable suggestion (a command or concrete next step). Suggestion is empty for
// validation errors whose message already states the fix.
type Hint struct {
	Message    string
	Suggestion string
	cause      error
}

// New returns a Hint with no wrapped cause.
func New(message, suggestion string) *Hint {
	return &Hint{Message: message, Suggestion: suggestion}
}

// Wrap returns a Hint that wraps cause so errors.Is and errors.As still reach it.
func Wrap(cause error, message, suggestion string) *Hint {
	return &Hint{Message: message, Suggestion: suggestion, cause: cause}
}

// Error returns a flat single line so non-fang contexts (logs, %w chains, tests)
// stay readable and the value appears once.
func (h *Hint) Error() string {
	if h.Suggestion != "" {
		return h.Message + " (try: " + h.Suggestion + ")"
	}
	return h.Message
}

// Unwrap exposes the wrapped cause for errors.Is and errors.As.
func (h *Hint) Unwrap() error { return h.cause }
