---
title: Drift tab renders typed slices as single-header clean lists
status: Accepted
date: 2026-06-27
related:
  - docs/adr/0018-drift-detection-by-commit-distance.md
  - docs/adr/0032-tui-reads-through-service-layer.md
  - docs/specs/2026-06-27-0230-settings-tab-and-drift-redesign.md
  - internal/tui/drift_tab.go
  - internal/service/governs.go
---

# Drift tab renders typed slices as single-header clean lists

## Context

`internal/tui/drift_tab.go` `markdownBox` prepends its own `# Drifted Docs` and `# Stale Docs` keycap headers, then appends `service.DriftResult.Markdown` and `service.StaleResult.Markdown`. Each of those service strings, built by `renderDriftMarkdown` and `renderStaleMarkdown` in `internal/service/governs.go`, already begins with the identical header. Every header renders twice. The two tables are glamour-rendered markdown squeezed into a 48-column half-split, so the rightmost column is cut off, and the check report is a raw wall of one row per finding. The service `Markdown` field exists for the CLI and is correct there; the bug is the tab consuming a pre-headed string and adding its own header.

## Decision

The Drift tab renders the typed result slices, not the service `Markdown`. It reads `DriftResult.Docs`, `StaleResult.Docs`, and `CheckResult.Issues` and renders them through `renderCleanList`, which emits exactly one keycap pill header per list. The tab no longer prepends any markdown header and no longer reads `.Markdown`, so duplicate headers are structurally impossible.

The layout becomes a vertical full-width stack: a colored summary line (`N errors / N warnings / N drifted / N stale`), a DRIFTED DOCS list (Type / Title / Doc / Referenced / Commits), a STALE DOCS list (Type / Title / Status / Doc / Commits / Matched), and a CHECK FINDINGS list grouped by `Kind` with a count and a sample, errors first, instead of one row per finding. `render.GlamourRender` leaves this file.

`internal/service/governs.go` is unchanged: it keeps emitting `Markdown` for the CLI. This is a presentation-only change in the tab, consistent with ADR 0032 (the tab reads the service result, just the typed fields).

## Consequences

- Single keycap headers, no duplicates; the bug cannot recur because the header source is now the single pill per clean list.
- Full `tableWidth` per stacked box gives columns room, removing the cut-off; `fitCleanListColumns` shrinks the flexible primary column instead of clipping the rightmost one.
- The check report is scannable: kinds grouped with counts, not a wall.
- The Drift tab matches the Status and Browse clean-list treatment, so the TUI is visually consistent.
- The service still computes `Markdown` that the tab ignores; harmless, and kept so the CLI rendering is untouched.

## Alternatives considered

- Strip only the tab's own headers and keep rendering the service `Markdown`. Rejected: still glamour-renders a width-constrained table that cuts off columns and keeps two parallel renderings.
- Drop the headers from `governs.go`. Rejected: the CLI depends on them and service rendering is out of scope; the bug lives in the tab.
- Keep the horizontal split. Rejected: halving width caused the cut-off; vertical full-width stacking fixes it.

## References

- ADR 0018 drift detection by commit distance, ADR 0032 TUI reads through service.
- `internal/tui/drift_tab.go`, `internal/tui/clean_list.go`, `internal/service/governs.go`.
- Spec: docs/specs/2026-06-27-0230-settings-tab-and-drift-redesign.md
</content>
