---
title: Settings is the sixth Stardust TUI tab
status: Accepted
date: 2026-06-27
supersedes: 0030
related:
  - docs/adr/0030-five-tab-stardust-tui-model.md
  - docs/adr/0031-additive-default-path-tui.md
  - docs/specs/2026-06-27-0230-settings-tab-and-drift-redesign.md
---

# Settings is the sixth Stardust TUI tab

## Context

ADR 0030 fixed the interactive TUI at five tabs: Search, Browse, Graph, Drift, Status. None of them edits the per-vault `.stardust` config or runs index actions; config is hand-edited in `config.toml` and reindex / rebuild / registry are CLI-only. exo-jobs, the framework Stardust copied its TUI from (ADR 0029, ADR 0033), ships a Settings tab as its config and action surface. Stardust needs the same: one place to view and edit `embed_model`, `ollama_url`, the `ignore` list, and the reranker settings, to see collections with schema and record counts, and to run the index actions.

## Decision

Add Settings as a sixth tab at index `5`, after Status. `tabNames` becomes six entries; the number-jump keys extend to `"6"`. The tab satisfies the existing `TabModel` interface and is wired additively into the App fan-out (`buildTabs`, `applySize`, `Init`, `syncFrame`, `activeTabModel`, the `View` switch, and `Update` routing), preserving ADR 0031: launching the TUI is unchanged and no existing tab moves.

This supersedes the five-tab count in ADR 0030. Every other decision in ADR 0030 (per-tab `TabModel`, animated cosmic chrome, the tab interface) stands.

## Consequences

- The TUI gains a config and action surface without leaving the terminal.
- The tab count is now six; any code or test asserting five tabs updates to six.
- Left/right tab switching is gated behind the active tab's `Focused()` so the Settings text editor can receive arrow keys; this also fixes arrow handling in the Search input.
- Future tabs append at the end behind the same additive pattern.

## Alternatives considered

- Fold config into the Status tab. Rejected: Status is a read-only health probe; mixing editable config and actions into it muddies both and breaks the one-concern-per-tab model.
- A separate `stardust config` TUI invoked outside the main app. Rejected: duplicates the chrome and the service wiring, and splits the operator's mental model across two binaries' worth of UI.

## References

- ADR 0030 five-tab model (count superseded here), ADR 0031 additive default path, ADR 0029 / 0033 exo-jobs framework copy.
- Spec: docs/specs/2026-06-27-0230-settings-tab-and-drift-redesign.md
- exo-jobs `cli/src/internal/ui/settings_tab.go`.
</content>
