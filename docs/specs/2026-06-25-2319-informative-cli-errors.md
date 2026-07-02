---
title: Informative CLI errors across stardust
status: Implemented
version: 1
date: 2026-06-25
related:
  - docs/adr/0012-cli-hint-error-type.md
  - docs/adr/0013-fang-error-handler-renders-suggestions.md
  - docs/plans/2026-06-25-2319-fang-cli-cosmic.md
  - docs/specs/2026-06-25-2319-fang-cli-cosmic.md
  - internal/config/config.go
  - internal/cli/context.go
  - internal/cli/hooks.go
  - internal/cli/new.go
  - internal/cli/sync.go
---

# Informative CLI errors across stardust

Every stardust error that has a concrete next action carries a structured `Hint`, the fang error handler renders the suggested command on its own highlighted `Run:` line in the cosmic accent color, and non-actionable errors are reduced to one clean sentence with no internal jargon and no repeated value.

<details>
<summary><b>Problem</b></summary>
<br>

Once fang is adopted (sibling spec `docs/specs/2026-06-25-2319-fang-cli-cosmic.md`), stardust surfaces errors as flat wrapped strings under fang's `ERROR` block. Three defects make them hard to act on:

1. The fix is implied, not stated. `config.ErrNoVault` (`internal/config/config.go:111`) reads `config: no .stardust directory found (run 'stardust init')` - the command is buried mid-string in parentheses and carries an internal `config:` prefix. The reader cannot scan for the action and the prefix is noise.
2. Several obvious next actions are missing entirely. `check --strict` returns `%d vault error(s)` (`check.go:52`) with no hint that `stardust check --fix` or `stardust index` is the next step. `registry stale` returns `%d stale docs` (`registry.go:65`) with no hint to update the docs. `hooks` on a non-git dir returns `hooks: %s is not a git repository` (`hooks.go:28`) with no hint to `git init`.
3. No shared shape. Each call site builds an ad hoc `fmt.Errorf` string, so there is no consistent way to render the actionable part distinctly or to test that every actionable error names its fix.

A grep of `internal/cli` and `internal/config` finds the actionable sites are not uniform: only some have a single runnable command, the rest are flag-required or pure input validation.

</details>

<details>
<summary><b>Context and background</b></summary>
<br>

fang is adopted in the sibling spec (ADRs 0009-0011). fang renders errors and help through `colorprofile.NewWriter`, which auto-disables ANSI on a non-tty and honors `NO_COLOR`; the headless data writers (`emitMarkdown`/`emitJSON` to `cmd.OutOrStdout()`) are never routed through fang, so the markdown-safe boundary (ADR 0010) holds and MUST be preserved by this work.

The actionable sites split into three buckets:

| Bucket | Examples (file:line) | Treatment |
|--------|----------------------|-----------|
| Command-bearing | `config.ErrNoVault` (`config.go:111`, suggest `stardust init`); `check --strict` `%d vault error(s)` (`check.go:52`, suggest `stardust check --fix`); `registry stale` `%d stale docs` (`registry.go:65`, suggest update + `stardust registry`); `sync check failed` (`sync.go`, suggest `stardust sync`); `hooks ... not a git repository` (`hooks.go:28`, suggest `git init`) | `Hint` with a runnable `Suggestion`, rendered on a `Run:` line |
| Flag-required | required flags missing on sync, new, cron run | clean one-line message; cobra usage already prints the flag |
| Input validation | `unsupported sync scope %q` (`sync.go:176`); `unsupported sync tool %q` (`sync.go:193`); `unsupported sync profile %q` (`sync.go:95`); `new: %s already exists and is not empty` (`new.go:80`); `sync config already exists` (`sync.go:109`) | clean one-line message stating the expected value once; no suggestion |

The `config:` and `hooks:` and `new:` and `sync:` prefixes that bury the message are dropped on the command-bearing sites; the value (path, count, enum) appears once.

</details>

<details>
<summary><b>Goals</b></summary>
<br>

1. Every actionable CLI error states its problem once, in plain language, with no internal package prefix and no repeated value or path.
2. Every error whose fix is a single command carries that command in a structured `Suggestion`, and the fang handler renders it on its own highlighted `Run:` line in the cosmic accent color.
3. A leaf `internal/clierr` package owns the error type so any layer (config, cli, hooks, sync, registry, service) can return it without an import cycle.
4. The markdown-safe boundary is preserved: piped `--output json` and `--output md` output stays free of ANSI escape bytes.
5. A test asserts that every command-bearing error names a non-empty suggestion, so the property is enforced rather than hoped for.

</details>

<details>
<summary><b>Non-goals</b></summary>
<br>

- Rewriting cobra usage output. Flag-required errors keep relying on cobra's usage block for the flag list.
- Localizing or internationalizing messages.
- Adding suggestions to errors with no single concrete next action (pure validation states only the expected value).
- Changing the headless renderers or the per-command `--output` format.
- The `config:` package-level sentinel `ErrNoVault` keeps its sentinel identity for `errors.Is` callers; the spec wraps it in a `Hint` at the CLI boundary rather than changing its package string (see Approach).

</details>

<details>
<summary><b>Approach</b></summary>
<br>

New leaf package `internal/clierr` with one carrier type:

```go
// Package clierr carries actionable CLI errors: a clean problem statement plus
// an optional runnable suggestion the fang error handler renders distinctly.
package clierr

// Hint is an actionable error: a clean problem statement plus an optional
// runnable suggestion (a command or concrete next step). Suggestion is empty
// for validation errors whose message already states the fix.
type Hint struct {
    Message    string
    Suggestion string
    cause      error
}

// New returns a Hint with no wrapped cause.
func New(message, suggestion string) *Hint { return &Hint{Message: message, Suggestion: suggestion} }

// Wrap returns a Hint that wraps cause so errors.Is/As still reach it.
func Wrap(cause error, message, suggestion string) *Hint {
    return &Hint{Message: message, Suggestion: suggestion, cause: cause}
}

// Error returns a flat single line so non-fang contexts stay readable.
func (h *Hint) Error() string {
    if h.Suggestion != "" {
        return h.Message + " (try: " + h.Suggestion + ")"
    }
    return h.Message
}

// Unwrap exposes the wrapped cause for errors.Is/As.
func (h *Hint) Unwrap() error { return h.cause }
```

`Error()` returns a flat single line (suggestion inline in parentheses) so non-fang contexts (logs, `%w` chains, tests, the old `version` subcommand path) stay readable and the path or value appears once.

The fang handler in `internal/cli` extracts the hint with `errors.As` and renders structure:

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
    fang.DefaultErrorHandler(w, styles, err) // unchanged for plain errors
})
```

The exact `WithErrorHandler` signature and the `fang.Styles` / default-handler names MUST be confirmed against the adopted `charm.land/fang/v2` version in source; the handler styles (`runLabel`, `suggestionStyle`) come from the `internal/ui` cosmic tokens (accent pink for the command, muted for the `Run:` label), not hardcoded hex. Because fang routes the handler writer through `colorprofile.NewWriter`, the styled output auto-disables on a non-tty, so piped errors stay plain and the data paths are untouched.

Call-site conversion, by bucket:

- Command-bearing: at the CLI boundary, convert to `clierr.New(message, command)` or `clierr.Wrap(cause, message, command)`. For `config.ErrNoVault`, the CLI catches it in `resolveVault`/`openService` (`context.go`) and wraps it: `clierr.Wrap(err, "no stardust vault found here", "stardust init")`. `check --strict` returns `clierr.New(fmt.Sprintf("%d vault error(s)", n), "stardust check --fix")`. `registry stale` returns `clierr.New(fmt.Sprintf("%d stale docs", n), "stardust registry")`. `hooks` non-git returns `clierr.New(root+" is not a git repository", "git init")`.
- Flag-required and validation: leave as plain errors but de-jargon the message (state the value once, drop the `sync:`/`new:` prefix that buries it). These render under fang's default handler.

`config.ErrNoVault` keeps its sentinel string for any `errors.Is(err, config.ErrNoVault)` caller; the CLI wraps it with `%w` via `clierr.Wrap`, so both `errors.Is(err, config.ErrNoVault)` and `errors.As(err, &hint)` succeed.

</details>

<details>
<summary><b>Alternatives considered</b></summary>
<br>

- Hint type in `internal/config`. Rejected: hooks, sync, registry, and service would import config only for the error type, and config does not belong upstream of them. A leaf `internal/clierr` has no such pull.
- Per-error custom types (a `NoVaultError`, a `StaleDocsError`, and so on). Rejected: more types than the handler needs; the handler only needs message plus suggestion, so one carrier type covers every site and keeps the `errors.As` switch to one branch.
- Change `config.ErrNoVault`'s string to drop the `config:` prefix and add the suggestion there. Rejected: it is a sentinel checked by `errors.Is` and used outside the CLI; the CLI boundary is the right place to dress it for humans, leaving the sentinel pure.
- Encode the suggestion in the message and parse it in the handler. Rejected: fragile string parsing, and it loses the structured field a test can assert on.

</details>

<details>
<summary><b>Risks</b></summary>
<br>

- `errors.As` reach. A hint wrapped without `%w` becomes invisible to the handler. Mitigation: use `clierr.Wrap` or `%w`; a test wraps a hint twice and asserts the handler still renders the suggestion.
- ANSI leak into data. A mistaken render path could color piped output. Mitigation: the zero-ANSI test on `--output json` from the sibling spec stays in the suite and is part of verification.
- Double-dressing `ErrNoVault`. If both config and the CLI add a suggestion, the message could read it twice. Mitigation: config keeps only the bare sentence (or keeps its current string but the CLI overrides the Message), and only the CLI wrap adds the `Suggestion`.

</details>

<details>
<summary><b>Open questions</b></summary>
<br>

1. Suggestion verb label. The handler prefixes the command with `Run:`; alternatives are `Try:` or `Fix:`. Default `Run:` for commands, no label for non-command next steps.
2. Whether flag-required errors also gain a suggestion (for example `Run: stardust sync init`). Default no; cobra usage covers the flag list.
3. Whether `config.ErrNoVault` keeps its `config:` prefix for non-CLI callers. Default yes; the CLI overrides the human-facing Message.

</details>

<details>
<summary><b>Verification</b></summary>
<br>

- `internal/clierr`: `Hint.Error()` returns one line with the value once and the suggestion inline; `errors.As` extracts a hint through a double `%w` wrap; `errors.Is(wrapped, config.ErrNoVault)` still succeeds.
- A table test enumerates the command-bearing sites and asserts each returns a `*clierr.Hint` with a non-empty `Suggestion`.
- The fang handler rendered to a buffer contains the message and, when present, a `Run:` line with the command; for a plain error it matches the default handler output.
- The zero-ANSI guarantee holds: piping a data command `--output json` as a non-tty yields no `\x1b[` bytes.
- Gates: `go build ./...`, `go test ./...`, `go vet ./...`, `gofmt -l .` empty, `make lint` clean, no em or en dashes.

</details>

<details>
<summary><b>Out of scope</b></summary>
<br>

The fang adoption and the cosmic colorscheme (sibling spec `docs/specs/2026-06-25-2319-fang-cli-cosmic.md`). The same pattern extended to exo-jobs and other repos (already shipped in exo-jobs; this is the stardust mirror).

</details>

<details>
<summary><b>Work breakdown</b></summary>
<br>

1. `internal/clierr` package and `Hint` type with `New`, `Wrap`, `Error`, `Unwrap`, plus unit tests.
2. fang `WithErrorHandler` in `internal/cli` rendering message plus suggestion from `internal/ui` tokens (added alongside the fang adoption from the sibling plan).
3. Convert the command-bearing sites (`ErrNoVault` at the CLI boundary, `check --strict`, `registry stale`, `sync check failed`, `hooks` non-git) to `clierr` hints with their exact commands.
4. De-jargon the flag-required and validation messages (value once, no `sync:`/`new:` prefix).
5. Tests: clierr unit, the command-bearing suggestion table, the handler buffer render, the retained zero-ANSI check.

</details>

<details>
<summary><b>References</b></summary>
<br>

- `docs/specs/2026-06-25-2319-fang-cli-cosmic.md`, ADRs 0009-0011 (fang adoption, headless boundary, cosmic scheme).
- charm-fang skill (`charm.land/fang/v2`): `WithErrorHandler`, `WithColorSchemeFunc`, `fang.ColorScheme`, `fang.Styles`, `fang.DefaultErrorHandler`.
- Error sites: `internal/config/config.go:111` (`ErrNoVault`), `internal/cli/context.go` (resolveVault/openService), `internal/cli/check.go:52`, `internal/cli/registry.go:65`, `internal/cli/hooks.go:28`, `internal/cli/sync.go` (95, 109, 176, 193).
- The exo-jobs informative-errors spec `docs/specs/2026-06-25-2305-informative-cli-errors.md` (the proven template this adapts).

</details>
