---
title: A leaf clierr.Hint carries actionable errors
status: Accepted
version: 1
date: 2026-06-25
related:
  - docs/specs/2026-06-25-2319-informative-cli-errors.md
  - docs/adr/0013-fang-error-handler-renders-suggestions.md
---

# A leaf clierr.Hint carries actionable errors

A single `internal/clierr.Hint{Message, Suggestion, cause}` type carries every actionable CLI error, so any layer can return a clean problem plus a runnable suggestion without an import cycle.

## Context

Actionable errors are scattered as ad hoc `fmt.Errorf` strings across `internal/cli` and `internal/config`, with internal prefixes (`config:`, `hooks:`, `new:`, `sync:`) that bury the message and with the fix implied mid-sentence. There is no shared shape to render the actionable part distinctly or to test that every actionable error names its fix.

## Decision

A leaf package `internal/clierr` exports one carrier type:

```go
type Hint struct {
    Message    string
    Suggestion string
    cause      error
}
func New(message, suggestion string) *Hint
func Wrap(cause error, message, suggestion string) *Hint
func (h *Hint) Error() string   // "Message (try: Suggestion)" or "Message"
func (h *Hint) Unwrap() error   // exposes cause for errors.Is/As
```

`Error()` returns a flat single line so logs, `%w` chains, and tests stay readable and the value appears once. `Suggestion` is empty for validation errors whose message already states the fix. `internal/clierr` is a leaf with no stardust imports, so config, cli, hooks, sync, registry, and service can all return it without a cycle. `config.ErrNoVault` keeps its sentinel string for `errors.Is` callers; the CLI wraps it via `clierr.Wrap(err, ...)` with `%w` so both `errors.Is` and `errors.As` succeed.

## Consequences

- One carrier type covers every actionable site; the handler's `errors.As` switch has one branch.
- Validation and flag-required errors stay plain (no suggestion).
- A test can enumerate command-bearing sites and assert each carries a non-empty `Suggestion`.

## Alternatives considered

- Per-error custom types (`NoVaultError`, `StaleDocsError`, ...): more types than the handler needs. Rejected.
- Put the type in `internal/config`: pulls config upstream of hooks/sync/registry only for an error type. Rejected for a leaf package.
- Encode the suggestion in the message and parse it in the handler: fragile, loses the structured field a test asserts on. Rejected.

## References

- docs/specs/2026-06-25-2319-informative-cli-errors.md
- internal/config/config.go (`ErrNoVault` sentinel)
- The exo-jobs clierr decision (its ADR 0011)
