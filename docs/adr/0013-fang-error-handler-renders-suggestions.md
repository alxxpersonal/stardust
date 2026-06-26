---
title: "Fang error handler renders a Run: suggestion line"
status: Proposed
version: 1
date: 2026-06-25
related:
  - docs/specs/2026-06-25-2319-informative-cli-errors.md
  - docs/adr/0012-cli-hint-error-type.md
  - docs/adr/0010-fang-headless-output-boundary.md
---

# Fang error handler renders a Run: suggestion line

A `fang.WithErrorHandler` extracts a `clierr.Hint` with `errors.As` and renders the message, then the suggestion on its own highlighted `Run:` line in the cosmic accent; plain errors fall through to fang's default handler. Three buckets get three treatments.

## Context

With `clierr.Hint` carrying message plus suggestion (ADR 0012) and fang adopted (ADR 0009), the suggestion needs to render distinctly. fang exposes `WithErrorHandler(func(io.Writer, fang.Styles, error))`. The handler writer is routed through `colorprofile.NewWriter`, so styled output auto-disables on a non-tty and the markdown-safe boundary (ADR 0010) holds.

## Decision

The handler is:

```go
fang.WithErrorHandler(func(w io.Writer, styles fang.Styles, err error) {
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
})
```

`runLabel` and `suggestionStyle` come from the `internal/ui` cosmic tokens (muted `Run:`, pink-accent command), not hardcoded hex. The exact `fang.Styles` and `DefaultErrorHandler` names are confirmed against the adopted v2 source. The three buckets:

| Bucket | Treatment |
|--------|-----------|
| Command-bearing | `clierr.Hint` with a `Suggestion`; handler renders the `Run:` line |
| Flag-required | plain error, de-jargoned; cobra usage prints the flag |
| Input validation | plain error stating the expected value once; no suggestion |

## Consequences

- Actionable errors surface their fix on a dedicated, styled line.
- Validation and flag errors stay clean and one-line, no false `Run:` suggestion.
- Piped errors stay plain (colorprofile non-tty downgrade); the zero-ANSI data test still holds.
- The handler depends on the fang error-handler API; a signature change is caught at compile time.

## Alternatives considered

- A second fang ColorScheme field for suggestions: the suggestion is rendered by our handler from `internal/ui` tokens; no fang-level field needed. Rejected.
- Add a `Run:` suggestion to flag-required errors too: cobra usage already lists the flag. Default no, revisit if desired.

## References

- docs/specs/2026-06-25-2319-informative-cli-errors.md
- charm-fang skill (`WithErrorHandler`, `fang.Styles`, `fang.DefaultErrorHandler`)
- The exo-jobs error-handler decisions (its ADRs 0012, 0013)
