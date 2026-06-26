---
title: Copy and adapt the exo-jobs TUI framework
status: Superseded
date: 2026-06-26
superseded_by: docs/adr/0033-verbatim-copy-exo-jobs-tui-framework.md
---

# Copy and adapt the exo-jobs TUI framework

Superseded by ADR 0033. The binding decision is now a verbatim copy of the
exo-jobs TUI framework with only package/import path, palette, and banner
replacement.

## Context

The exo-jobs repo has a polished, domain-neutral Bubble Tea framework: an animated banner, bordered and error boxes, a hint status bar with overflow clamping, a custom active-row table grid, an ANSI/control sanitizer, styled charm-v2 adapters (textinput, spinner, table, viewport), and a `TabModel` interface with a frame-synced root model. Stardust's current TUI has none of this chrome. The framework files (`app.go`, `tabs.go`, `banner.go`, `styles.go`, `components/{box,statusbar,tablegrid,sanitize,charm_adapters}.go`) are domain-neutral and map cleanly onto a vault-backed multi-tab UI. They live in `exo-jobs/cli/src/internal/ui/`, another module's `internal/` tree.

Go forbids importing another module's `internal/` packages. Vendoring a sibling product's internal package would also couple two unrelated products and their dependency pins.

## Decision

Copy the domain-neutral framework files into `internal/tui` (chrome) and `internal/tui/components` (the box, statusbar, tablegrid, sanitize, charm_adapters set), then adapt them to Stardust's pinned `charm.land/...v2` versions and recolor them to the cosmic palette. Do not import, vendor, or git-submodule the exo-jobs package. The jobs-domain files (`agent_tab.go`, `jobs_tab.go`, `crons_tab.go`, the other `*_tab.go`, and `internal/{agent,store,propose,scorer}`) are not copied; Stardust's own tabs are written against `internal/service`.

The copied component tests (`sanitize_test.go`, `box_test.go`, `tablegrid_test.go`, `statusbar_test.go`, `charm_adapters_test.go`) come along and are adapted to the cosmic palette, so the copied code keeps its coverage.

## Consequences

- Stardust gains a battle-tested TUI framework without re-solving multi-tab routing, animated banners, ANSI safety, and active-row tables.
- The copied code is now Stardust's to maintain; upstream exo-jobs fixes do not flow in automatically. Accepted, because the framework is stable and small.
- The copy is recolored at copy time, so no exo gray leaks into the cosmic identity.
- A clean module boundary: Stardust depends on no exo-jobs code at build time.

## Alternatives considered

- **Import or vendor the exo `internal/ui` package.** Impossible (Go internal rule) or coupling (vendoring a sibling product). Rejected.
- **Extend the existing three concrete tabs in place.** No shared chrome to extend; would rebuild the framework anyway. Rejected.
- **Build a fresh framework from scratch.** Re-invites the bugs the exo framework already fixed. Rejected.

## References

- Spec: `docs/specs/2026-06-26-2352-interactive-tui.md`.
- Source: `/Users/alxx/Desktop/Exo Jobs/10-Code/Worktrees/10-Active/exo-jobs/cli/src/internal/ui/`.
- ADR 0011 stardust-cosmic-colorscheme (palette the copy is recolored to).
