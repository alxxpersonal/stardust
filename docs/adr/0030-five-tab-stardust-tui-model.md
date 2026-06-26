---
title: Five-tab TabModel rebranded to Stardust functions
status: Accepted
date: 2026-06-26
---

# Five-tab TabModel rebranded to Stardust functions

## Context

The exo-jobs framework manages 14 jobs-domain tabs through a `TabModel` interface and an `int` active-tab index, with number keys jumping to tabs and arrows cycling. Stardust's current TUI has three concrete tab structs (Search, Status, Graph) delegated by a switch in `App.Update`, with no shared interface. The new TUI must surface five Stardust functions, not jobs: Search, Browse, Graph, Drift, Status. The framework's tab abstraction is being copied (ADR 0029), so the tab model is a locked decision.

A real concern from the current code (`app.go:68`): digit keys must switch tabs only when the active tab does not own text input, otherwise the Search box swallows them as query text.

## Decision

Adopt the copied `TabModel` interface, trimmed to what the root App calls:

```go
type TabModel interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (TabModel, tea.Cmd)
	View(width, height int) string
	Hints() []components.HintItem
	Focused() bool
}
```

Five tabs, indexed 0-4 as named constants `tabSearch, tabBrowse, tabGraph, tabDrift, tabStatus`. Number keys `1`-`5` jump directly, `tab`/`shift+tab` cycle, but digit jumps fire only when `!tabs[active].Focused()`. The root App holds `tabs []TabModel` and an `active int`; `Update` delegates to `tabs[active]` after handling global keys, frame ticks, and size, and routes async result messages (search done, spinner tick) to their owning tab regardless of the active index so a mid-load switch never strands them.

The exo interface's `StatusLine()` and `HeaderLabel()` are dropped; the App composes the banner and status bar itself from `Hints()`.

## Consequences

- One interface, five implementations, uniform routing. Adding a tab is implementing `TabModel` and appending to the slice.
- `Focused()` cleanly gates digit-key tab switching against text input, fixing the concern the old switch handled ad hoc.
- The tab order encodes the workflow narrative: find (Search), wander (Browse), see structure (Graph), see decay (Drift), see health (Status).

## Alternatives considered

- **Keep three concrete structs and a switch.** Does not scale to five and shares no chrome. Rejected.
- **Keep the full exo `TabModel` with `StatusLine`/`HeaderLabel`.** Unused surface; trimmed. Rejected.
- **Combine Drift into Status or Graph.** Drift is the coherence engine and deserves its own surface; folding it hides it. Rejected.

## References

- Spec: `docs/specs/2026-06-26-2352-interactive-tui.md`.
- ADR 0029 copy-adapt-exo-jobs-tui-framework.
- `internal/tui/app.go`, `internal/tui/tabs.go`.
