---
title: Stardust cosmic colorscheme for fang chrome
status: Proposed
version: 1
date: 2026-06-25
related:
  - docs/specs/2026-06-25-2319-fang-cli-cosmic.md
  - docs/adr/0009-fang-cobra-execute.md
---

# Stardust cosmic colorscheme for fang chrome

The fang `ColorScheme` is mapped from the existing Stardust cosmic palette - violet primary with silver-white text - not a monochrome scheme. This is the deliberate point of divergence from the exo-jobs gray template.

## Context

stardust already owns a cosmic violet identity in its TUI (`internal/tui/styles.go:13-21`) and glamour renderer (`internal/render/glamour.go:17-23`), with identical hexes duplicated in both. The CLI shell currently renders no color. The exo-jobs fang adoption used a monochrome gray scheme; stardust is the vault/stars engine and its shell SHOULD match its own cosmic brand instead.

## Decision

A new exported leaf package `internal/ui` holds the cosmic palette as `lipgloss.Color` tokens, and `internal/tui` and `internal/render` are pointed at it to collapse the duplication. `internal/cli/color_scheme.go` defines `cosmicColorScheme(lipgloss.LightDarkFunc) fang.ColorScheme` mapping the tokens onto every fang field. The theme is dark only, so the `LightDarkFunc` argument is ignored.

Locked cosmic identity (exact hexes):

| Token | Hex | Role | Fang fields |
|-------|-----|------|-------------|
| Primary | `#a78bfa` | violet | Title, Program |
| Secondary | `#c4b5fd` | light violet | FlagDefault, QuotedString, Help |
| Accent | `#f0abfc` | pink | Command, Flag, ErrorDetails |
| Text | `#e9e7ff` | silver-white | Base, Description, Argument |
| Muted | `#7c7ca0` | muted violet | Comment, DimmedArgument, Dash |
| Border | `#4c4c6d` | dark violet | (reserved) |
| CodeBg | `#16161e` | code bg | Codeblock |
| Bg | `#0a0a12` | near-black | ErrorHeader fg |

The error header is near-black text on a pink-accent background (`ErrorHeader: [2]color.Color{Bg, Accent}`), keeping the cosmic identity in the alert block. Whether to use pink-accent or a dedicated cosmic error-red is flagged in the spec for confirmation.

## Consequences

- The CLI shell, TUI, and glamour renderer share one palette source; no drift.
- A new `internal/ui` leaf dependency (only `charm.land/lipgloss/v2`).
- stardust's CLI reads visibly distinct from exo-jobs (violet, not gray).
- A version bump that renames a `fang.ColorScheme` field fails to compile and is caught immediately.

## Alternatives considered

- Reuse a monochrome scheme like exo-jobs: rejected; stardust has its own cosmic brand the shell should match.
- `WithTheme(fang.ColorScheme{...})` instead of `WithColorSchemeFunc`: works, noted as fallback; `WithColorSchemeFunc` is the documented light/dark entry point. Rejected as primary.
- Keep the palette duplicated and add a third copy in `cli`: three copies drift. Rejected in favor of one exported `internal/ui`.

## References

- docs/specs/2026-06-25-2319-fang-cli-cosmic.md
- internal/tui/styles.go, internal/render/glamour.go (the duplicated palette)
- charm-fang skill (`fang.ColorScheme` fields, `WithColorSchemeFunc`)
