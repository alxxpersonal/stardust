---
title: Fang styles only chrome, data writers stay raw
status: Proposed
version: 1
date: 2026-06-25
related:
  - docs/specs/2026-06-25-2319-fang-cli-cosmic.md
  - docs/adr/0009-fang-cobra-execute.md
---

# Fang styles only chrome, data writers stay raw

Fang governs help, usage, and error rendering only. Every headless data path keeps writing raw bytes to `cmd.OutOrStdout()`, so piped `--output json`, `--output md`, and `--output plain` stay ANSI-free, and a test enforces it.

## Context

stardust is agent-facing: its primary output is markdown and JSON data through `emitMarkdown`/`emitJSON` (`internal/cli/output.go`). The auto mode already glamour-renders on a TTY and emits plain markdown when piped. Adding fang's styled writers must not leak ANSI escape sequences into that data, which would corrupt the agent contract and break downstream parsers.

## Decision

Fang's styled writers wrap `c.OutOrStdout()`/`c.ErrOrStderr()` through `colorprofile.NewWriter` and are used ONLY for help and error rendering. The data path (`emitMarkdown`/`emitJSON` to `cmd.OutOrStdout()`) is written by the command and is never routed through fang. lipgloss and colorprofile both downgrade to no-color on a non-tty and honor `NO_COLOR`, so even chrome is plain when piped. A hard test runs a data command (`query` or `registry`) with `--output json` and stdout redirected to an `os.Pipe` (non-tty), then asserts the captured bytes contain no `\x1b[` and that the JSON parses. The boundary is asserted, not assumed.

## Consequences

- The markdown-safe boundary is structural: fang touches chrome, never data.
- A regression that colors piped data fails the build via the zero-ANSI test.
- No custom output stream is needed; the existing `cmd.OutOrStdout()` path is already isolated from fang's writers.

## Alternatives considered

- Route fang's writers to a separate stream to guarantee isolation: unnecessary, since fang only writes chrome and the data path is already separate; adds complexity and a new failure mode. Rejected.

## References

- docs/specs/2026-06-25-2319-fang-cli-cosmic.md
- internal/cli/output.go (`emitMarkdown`, `emitJSON`, `isTTY`)
- The exo-jobs headless-boundary decision (its ADR 0009)
