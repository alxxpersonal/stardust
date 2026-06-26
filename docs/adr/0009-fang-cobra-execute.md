---
title: Wrap the cobra tree with fang.Execute
status: Proposed
version: 1
date: 2026-06-25
related:
  - docs/specs/2026-06-25-2319-fang-cli-cosmic.md
  - docs/adr/0010-fang-headless-output-boundary.md
  - docs/adr/0011-stardust-cosmic-colorscheme.md
---

# Wrap the cobra tree with fang.Execute

The stardust CLI shell runs through `fang.Execute(ctx, root, opts...)` instead of `cobra.Command.Execute()`, gaining styled help, a styled error block, auto `--version`, completions, a manpage, and signal handling without touching any command.

## Context

`internal/cli/root.go:21-26` calls `newRootCmd().Execute()` and prints a flat `error: %v` line on failure. The CLI has no `--version` flag (only a manual `version` subcommand), no shell completions, and no manpage. The TUI is already cosmic-themed but the command shell is bare cobra default. fang wraps cobra and adds all of this in one line, the same proven path adopted in exo-jobs (its ADR 0008).

## Decision

`cli.Execute()` builds the root, creates a signal-aware context, and calls `fang.Execute(ctx, root, opts...)` from `charm.land/fang/v2` (vanity domain, matching the four `charm.land/...v2` modules already in `go.mod`; NOT `github.com/charmbracelet/fang`). Options passed: `WithColorSchemeFunc(cosmicColorScheme)` (ADR 0011), a conditional `WithVersion`/`WithCommit`, and `WithNotifySignal(os.Interrupt, syscall.SIGTERM)`. Every `*cobra.Command` stays exactly as written. The flat error line is removed; fang owns error rendering. The redundant `SilenceUsage`/`SilenceErrors` settings in `newRootCmd()` stay (fang sets them anyway, harmless).

## Consequences

- One-line shell upgrade: styled help, styled `ERROR` block, `--version`, `completion`, hidden `man`, signal cancellation into every `RunE` via `cmd.Context()` (which `serve` can select on).
- A new direct dependency `charm.land/fang/v2`.
- The manual `version` subcommand becomes redundant but stays for now (harmless).
- Build ldflags gain `-X` for `version` and `commit`; unset builds fall back to `debug.ReadBuildInfo` via `versionOpt()`.

## Alternatives considered

- Hand-roll lipgloss help and error templates on plain cobra: reimplements fang, drifts from the palette, still needs manual version/completion/manpage wiring. Rejected.
- Keep the manual `version` subcommand and skip fang `--version` with `WithoutVersion()`: loses the build-info fallback for no gain. Rejected.

## References

- docs/specs/2026-06-25-2319-fang-cli-cosmic.md
- charm-fang skill (`charm.land/fang/v2` Execute, options)
- The exo-jobs fang adoption (its ADR 0008)
