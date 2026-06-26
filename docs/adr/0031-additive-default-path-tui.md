---
title: TUI is additive to the no-arg TTY path only
status: Accepted
date: 2026-06-26
---

# TUI is additive to the no-arg TTY path only

## Context

Stardust serves humans and agents from one binary. The hard constraint on the TUI rebuild is that the CLI subcommands, the MCP server (`stardust serve --mcp`), and the SDK keep their exact behavior, and that headless `query`, `bundle`, `check`, and `status` output stays byte-identical. The launch gate already exists in `internal/cli/root.go:79-89`: the root command's `RunE` runs the TUI only when there is no subcommand and stdout is a TTY, and prints help when piped. This is the markdown-safe boundary (SPEC: TTY -> interactive TUI, non-TTY -> clean markdown/JSON).

## Decision

The TUI replacement is confined to `internal/tui` and its new `internal/tui/components` subtree. No CLI command file, no `serve`, and no SDK code changes. The launch wiring is left exactly as it is: bare `stardust` on a TTY calls `tui.Run(vc.Layout, vc.Config)`; piped invocation prints help. `tui.Run` keeps its `(config.Layout, config.Config) error` signature, so `root.go` is not edited.

Headless output is never produced by the TUI. Every subcommand writes to `cmd.OutOrStdout` through its existing renderer; the TUI is a separate code path reachable only on the no-arg TTY branch. A verification step diffs the four headless outputs against the pre-change binary and requires an empty diff.

## Consequences

- The blast radius is one package tree. Subcommands, MCP, and SDK are provably untouched because their files are not edited.
- The TUI cannot leak ANSI or chrome into agent output, because agents hit the non-TTY branch that never constructs the TUI.
- A future change that needs to alter `tui.Run`'s signature must re-touch `root.go`; until then the boundary is stable.

## Alternatives considered

- **Add a `stardust tui` subcommand.** Redundant; bare `stardust` on a TTY already is the entry. A subcommand would also tempt agents to invoke it. Rejected.
- **Gate on a flag instead of TTY detection.** The TTY gate is the existing, correct markdown-safe boundary; a flag duplicates it and invites misuse. Rejected.
- **Let tabs print to stdout for debugging.** Breaks the byte-identical headless guarantee. Rejected.

## References

- Spec: `docs/specs/2026-06-26-2352-interactive-tui.md`.
- `internal/cli/root.go:79-89` (launch gate), `internal/tui/run.go` (Run signature).
- ADR 0010 fang-headless-output-boundary.
