---
title: Verbatim copy of the exo-jobs TUI framework
status: Accepted
date: 2026-06-27
supersedes:
  - docs/adr/0029-copy-adapt-exo-jobs-tui-framework.md
---

# Verbatim copy of the exo-jobs TUI framework

## Context

ADR 0029 allowed the exo-jobs TUI framework to be copied and adapted. That left too much room for simplification: the first Stardust TUI attempt kept the data model but reduced the visual system to a thin banner, plain boxes, and lighter layout treatment. That result failed the actual product goal.

The source of truth is now the exo-jobs visual framework itself:

- `~/Desktop/Exo Jobs/10-Code/Worktrees/10-Active/exo-jobs/cli/src/internal/ui`
- `~/Desktop/Exo Jobs/10-Code/Worktrees/10-Active/exo-jobs/cli/src/internal/anim`

## Decision

Stardust copies the exo-jobs TUI visual code verbatim into `internal/tui`, `internal/tui/components`, and `internal/tui/anim`, changing only package names, import paths, the color palette, and the banner content. The root app keeps the exo treatment: centered animated banner, centered tab bar, animated header box, status hint bar, active-row table grid, bordered boxes, breathing and shimmer animations, and the global flame frame tick.

The colors are remapped to the Stardust cosmic palette from `internal/ui`. Semantic status colors remain unchanged: success `#34d399`, error `#ff5555`, and warning `#f59e0b`. Markdown rendering does not copy the exo glamour style block; it routes through Stardust's existing `internal/render` glamour renderer.

The banner is regenerated as `STARDUST` with pyfiglet using the same `dos_rebel` font as exo-jobs. The generated wordmark is stored in `internal/tui/banner.go`, with the subtitle `STARDUST local-first markdown context engine`.

## Consequences

- Visual parity with exo-jobs is the requirement. A simplified Stardust-native approximation is rejected.
- Stardust tab data is domain-specific, but the chrome and motion language stay copied from exo-jobs.
- Future visual fixes should first check whether exo-jobs already solved the same treatment and copy the visual primitive forward.

## Superseded Decision

ADR 0029 is superseded. The old phrase "copy and adapt" is now too permissive. The binding decision is "verbatim copy with palette and banner replacement."
