---
title: Settings tab and Drift tab redesign
status: Implemented
version: 1
date: 2026-06-27
related:
  - docs/specs/2026-06-26-2352-interactive-tui.md
  - docs/adr/0030-five-tab-stardust-tui-model.md
  - docs/adr/0032-tui-reads-through-service-layer.md
  - docs/adr/0018-drift-detection-by-commit-distance.md
  - internal/tui/drift_tab.go
  - internal/tui/clean_list.go
  - internal/service/service.go
  - internal/service/governs.go
---

Add a sixth Settings tab that views and edits the per-vault `.stardust` config, lists collections with schema and record counts, and runs index actions through additive service methods, and redesign the Drift tab into single-header, scannable clean lists that kill the duplicate-keycap-header bug and the cut-off drift column.

<details>
<summary><b>Problem</b></summary>
<br>

Two gaps in the interactive TUI (ADR 0030, ADR 0032):

1. There is no surface to view or edit the per-vault configuration. `embed_model`, `ollama_url`, the `ignore` list, and the reranker settings live in `.stardust/config.toml` and are only reachable by hand-editing the file. Collections, their schemas, and record counts are visible on the Status tab but read-only, and the index actions (reindex, rebuild, regenerate registry) are CLI-only. exo-jobs has a clean Settings tab pattern that Stardust copied the TUI framework from (ADR 0029, ADR 0033) but never ported.

2. The Drift tab is cluttered and buggy. `internal/tui/drift_tab.go` `markdownBox` prepends its own `# Drifted Docs` and `# Stale Docs` keycap headers, then appends `service.DriftResult.Markdown` / `service.StaleResult.Markdown`, each of which already begins with the identical header (`renderDriftMarkdown` / `renderStaleMarkdown` in `internal/service/governs.go`). The result renders every header twice. The check report renders as a wall of one row per finding, and the drift and stale tables are glamour-rendered markdown squeezed into a 48-column half-split, so the rightmost column is cut off.

</details>

<details>
<summary><b>Context and background</b></summary>
<br>

Locked decisions this work builds on:

- ADR 0030 fixed the TUI at five tabs (Search, Browse, Graph, Drift, Status). This spec extends that count to six; see ADR 0034.
- ADR 0031 keeps the default path additive: launching the TUI must not change for existing flows.
- ADR 0032 requires every tab to read through `internal/service`, never the index, git, or disk directly. This spec extends that rule to writes: config persistence and index actions go through additive service methods, not `config.Save` or `manifest.WriteRegistry` called from the tui package.
- ADR 0011 fixes the cosmic palette; ADR 0029 / 0033 copied the exo-jobs TUI framework verbatim.

Reusable surface already in the tree:

- `internal/tui/clean_list.go` `renderCleanList(title, label, cols, rows, width, activeRow)` renders one keycap pill header plus fitted columns. `fitCleanListColumns` caps each column at `MaxWidth`, enforces `MinWidth`, and shrinks the `Primary` column first when space is tight. This is the consistent wifey-style list used by Status and Browse.
- `internal/tui/anim.go` `animatedRoundedBox` / `animatedDoubleBox` wrap titled content in the rotating cosmic border.
- `internal/tui/components/charm_adapters.go` `NewExoTextInput(placeholder)` returns a themed `textinput.Model`.
- `internal/service` already exposes every read and write the tabs need except two: live config mutation and one-call registry regeneration. `Index(ctx, "")`, `Rebuild(ctx)`, `ListCollections(ctx)`, `GetCollection(ctx, name)` (carries `Fields`), `Registry(order)`, `RefreshManifest(ctx)` exist. `config.Load` / `config.Save` and `manifest.WriteRegistry` exist but are not exposed through the service.
- exo-jobs `settings_tab.go` is the layout and interaction model: a single cursor over editable rows plus action rows, inline editors for scalar values, and sub-views for list-shaped config (categories) reached by an action row.

</details>

<details>
<summary><b>Goals</b></summary>
<br>

1. A sixth Settings tab renders the live `.stardust` config, the collections with schema and record counts, and the index actions, all in the cosmic clean-list / keycap-header treatment.
2. Config scalar fields (`embed_model`, `ollama_url`, `reranker_url`, `reranker_model`) are inline-editable; the `ignore` list is editable through an add/remove sub-view; every edit persists to `.stardust/config.toml` and updates the live service so later reads use the new model and endpoints.
3. The index actions (reindex, rebuild, regenerate registry) run from the tab through the service layer, report their result, and never block the event loop or fire twice concurrently.
4. The Drift tab renders single keycap headers with no duplicates, a clear summary line, a drifted-docs list with well-fitted Type / Title / Doc / Referenced / Commits columns, a stale-docs list, and the check findings grouped and summarized instead of a raw wall.
5. All new tab code reads and writes only through `internal/service`; the CLI, MCP, and SDK surfaces are untouched.

</details>

<details>
<summary><b>Non-goals</b></summary>
<br>

- No CLI, MCP, or SDK changes. `internal/cli/registry.go` keeps its existing inline registry sequence; the new service method is additive and consumed only by the TUI.
- No change to `internal/service/governs.go` rendering. The service keeps emitting `Markdown`; the Drift tab simply stops consuming it and renders the typed `Docs` slices instead.
- No editing of collection schemas, records, or frontmatter from the Settings tab. Schemas are view-only.
- No new theme, palette, or border styles. Reuse ADR 0011 styles and the existing animated boxes.
- No background polling or live config file watching. The tab loads on open and refreshes on demand.

</details>

<details>
<summary><b>Approach</b></summary>
<br>

Three units: additive service methods, the Settings tab, and the Drift redesign. The app wiring threads the new tab in additively.

### Service methods (additive)

Two methods in `internal/service`, both thin and consistent with ADR 0032:

```go
// SetConfig persists cfg to the vault's config.toml and updates the live
// service, rebuilding the embed and rerank clients so later reads use the new
// model, Ollama URL, and reranker settings without reopening the service.
func (s *Service) SetConfig(cfg config.Config) error {
    if err := config.Save(s.Layout.Config(), cfg); err != nil {
        return err
    }
    s.Config = cfg
    s.embed = embed.New(cfg.OllamaURL, cfg.EmbedModel)
    s.rerank = rerank.New(cfg.RerankerURL, cfg.RerankerModel)
    return nil
}

// RegenerateRegistry regenerates docs/INDEX.md from the docs collections and
// refreshes the pinned agent manifest, mirroring `stardust registry`.
func (s *Service) RegenerateRegistry(ctx context.Context) error {
    groups, err := s.Registry(defaultRegistryOrder)
    if err != nil {
        return err
    }
    out := filepath.Join(s.Layout.Root, "docs", "INDEX.md")
    if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
        return fmt.Errorf("create registry dir: %w", err)
    }
    if err := manifest.WriteRegistry(out, groups); err != nil {
        return err
    }
    return s.RefreshManifest(ctx)
}
```

`SetConfig` closes the stale-client gap: the service caches `embed` and `rerank` clients built from config at `Open`; rebuilding them on save is what makes a model or endpoint change take effect live. `s.Config` is already exported, so reads need no getter. See ADR 0035.

### Settings tab

A new `internal/tui/settings_tab.go` modeled on exo-jobs `settings_tab.go`, satisfying the existing `TabModel` interface. State: the loaded `config.Config`, a cursor over a flat row list, an inline text editor, and two sub-views.

Row surface, in display order, a single cursor walks all of it:

| Key | Label | Kind | Action |
|---|---|---|---|
| `embed_model` | Embed model | text | inline edit |
| `ollama_url` | Ollama URL | text | inline edit |
| `reranker_url` | Reranker URL | text | inline edit (empty = disabled) |
| `reranker_model` | Reranker model | text | inline edit |
| `ignore` | Ignore list | action | open ignore sub-view |
| `reindex` | Reindex | action | `svc.Index(ctx, "")` |
| `rebuild` | Rebuild index | action | `svc.Rebuild(ctx)` |
| `registry` | Regenerate registry | action | `svc.RegenerateRegistry(ctx)` |
| `collections` | Inspect collections | action | open collections sub-view |

View composition (vertical, centered, `tableWidth`):

1. `animatedDoubleBox("CONFIG", rows)` where rows render `cursor + label(padded) + value`. Scalar values use `SuccessStyle`; an empty `reranker_url` renders `(disabled)` in `MutedStyle`. The focused editable row swaps its value for `editInput.View()`.
2. `animatedRoundedBox("COLLECTIONS", cleanList)` read-only, reusing the Status tab clean list: columns Collection / Records / Fields / Path / Description. `Fields` is `len(info.Fields)`. Always visible for at-a-glance counts.
3. `animatedRoundedBox("ACTIONS", rows)` for the three index actions plus inspect, with a running or result note.

Sub-views layered on the list, mirroring exo categories:

- Ignore sub-view: a clean list of the ignore entries, `n` to add (text input), `d` to delete; each change calls `SetConfig`. Esc returns.
- Collections sub-view: a clean list of collections, up/down select, `enter` shows that collection's schema as a clean list (Field / Type / Required / Enum) read through `svc.GetCollection`. Esc returns one level.

Editing scalars: `enter` opens `editInput` seeded with the current value; `enter` saves via `SetConfig`; `esc` cancels. Trim on save; a `reranker_url` left empty is a valid "disabled" value. No float parsing (unlike exo numbers) since every Stardust config scalar is a string.

Index actions run as a `tea.Cmd` returning `settingsActionMsg{kind, summary, err}`; a `busy` flag rejects a second action while one runs; `StatusLine` reports the outcome (`reindexed 12, skipped 3, vectors on`). The command runs the service call in its goroutine so the event loop never blocks.

`Focused()` returns true while editing or in any sub-view, so the app does not steal keys (number jumps and left/right tab switches are gated behind `!Focused()`).

### App wiring (additive)

- `internal/tui/tabs.go`: add `TabSettings = 5`, append `"Settings"` to `tabNames`.
- `internal/tui/app.go`: add the `settingsTab` field; extend `buildTabs`, `applySize`, `Init`, `syncFrame`, `activeTabModel`, the `View` content switch, and the `Update` message routing (`settingsActionMsg`, and the per-tab fallthrough). Add `"6"` to the number-jump block.
- `internal/tui/app.go`: gate the `left` / `right` tab-switch handling behind `!a.activeTabModel().Focused()`, matching the existing number-jump gate, so arrow keys reach the settings text editor (and the search input) instead of switching tabs. This is the one behavioral change to an existing path; it is strictly additive (a guard) and improves the search input too.

### Drift redesign

Rewrite the rendering half of `internal/tui/drift_tab.go`. Delete `markdownBox` and stop reading `t.drift.Markdown` / `t.stale.Markdown` entirely; render the typed slices `t.drift.Docs`, `t.stale.Docs`, `t.check.Issues` through `renderCleanList`. Because `renderCleanList` emits exactly one pill header per list and the tab no longer prepends markdown headers, the duplicate-header bug is structurally impossible.

New `View` is a vertical stack at `tableWidth`:

1. Summary band: one styled line `N errors  ·  N warnings  ·  N drifted  ·  N stale`, colored by worst severity (error red, else warn amber, else muted).
2. `animatedDoubleBox("DRIFTED DOCS", driftList)`: one row per moved binding. Columns Type (muted) / Title (primary, flexes) / Doc (muted underline) / Referenced (muted underline, the moved file) / Commits (count, right). Header label `cleanListCountLabel(len(drift.Docs), "doc")`. Empty -> `SuccessStyle "no drifted docs"`.
3. `animatedDoubleBox("STALE DOCS", staleList)`: columns Type / Title (primary) / Status / Doc (underline) / Commits (count, right) / Matched (muted). Empty -> `SuccessStyle "no stale docs"`.
4. `animatedRoundedBox("CHECK FINDINGS", checkSummary)`: grouped, not a wall. Aggregate issues by `Kind` into `{severity, count, sampleDetail}`, errors first. Columns Severity / Kind / Count (count, right) / Example (primary, flexes). Clean -> `SuccessStyle "clean"`.

Each box clips to a computed height share (`clipLines`) so three stacked boxes never overflow; full `tableWidth` per box gives the columns room, killing the cut-off. `render.GlamourRender` is dropped from this file.

</details>

<details>
<summary><b>Alternatives considered</b></summary>
<br>

- Keep rendering the service `Markdown` and just strip the tab's own headers. Rejected: the markdown still glamour-renders into a width-constrained table that cuts off columns, and it keeps two parallel renderings (markdown for CLI, the same markdown for TUI) where the clean list is the established TUI treatment. Rendering typed slices is consistent with Status and Browse and fixes both bugs at once.
- Edit `governs.go` to drop the headers from `DriftResult.Markdown`. Rejected: the CLI relies on those headers, and the task scopes out service rendering changes. The bug is the tab double-rendering, so fix it in the tab.
- Make the Settings tab call `config.Save` and `manifest.WriteRegistry` directly from the tui package. Rejected: violates ADR 0032 and leaves the live service holding stale embed and rerank clients after a config change. Additive service methods keep the read-through-service invariant and rebuild the clients.
- Refactor `internal/cli/registry.go` to call the new `RegenerateRegistry`. Rejected for this spec: it is a CLI change, out of scope. The duplication of the three-step sequence is small and intentional.
- A horizontal two-column Drift layout like the current split. Rejected: halving the width is what caused the cut-off column. Vertical full-width stacking is more scannable and fits the five-to-six column tables.

</details>

<details>
<summary><b>Risks</b></summary>
<br>

- `Rebuild` closes and reopens `s.store` in place. The service is shared across tabs through `be.svc`; a rebuild firing while another tab's load command reads concurrently could race on the store handle. Mitigation: the tabs load once on open and on explicit `r`, not on a poll, and the Settings `busy` flag serializes its own actions, so a deliberate rebuild while idle is safe. A service-level store lock is a noted follow-up, not in this spec.
- After a successful reindex, rebuild, or config change, other tabs hold stale cached data (status counts, collections, drift). Mitigation: each tab already refreshes on `r`; the Settings `StatusLine` reports the result so the change is visible. Broadcasting a refresh message is a future enhancement.
- Gating left/right behind `Focused()` changes an existing key path. Mitigation: it matches the existing number-jump gate and only blocks tab-switching while a tab owns text; covered by an app test.
- A bad `ollama_url` saved through the tab degrades search to FTS-only on the next read. This is the existing loud-degradation behavior (ADR 0016), surfaced rather than hidden; not a regression.

</details>

<details>
<summary><b>Open questions</b></summary>
<br>

- Should the Check Findings group expand to its individual rows on `enter`? v1 ships the grouped summary only; expand-on-enter is a candidate follow-up.
- Should a successful index action broadcast a refresh to the Status, Browse, and Graph tabs? v1 relies on per-tab `r`.
- Should `ollama_url` and `reranker_url` get light URL validation on save, or save verbatim and let the loud FTS-only degradation report a bad endpoint? v1 saves verbatim.

</details>

<details>
<summary><b>Verification</b></summary>
<br>

- `go build ./...` and `go vet ./...` clean.
- `go test ./internal/service/...`: `SetConfig` persists to `.stardust/config.toml`, updates `s.Config`, and changes the embed model reported by `Status`; `RegenerateRegistry` writes `docs/INDEX.md` and refreshes the manifest, against a temp vault.
- `go test ./internal/tui/...`:
  - Settings: config values render; cursor navigation moves; a scalar edit updates the in-model `cfg`; sub-view open/close transitions hold; `Focused()` is true while editing.
  - Drift: `strings.Count(out, "Drifted Docs") == 1` and `== 1` for `Stale Docs` (the duplicate-header regression test); the drift list contains the referenced moved file and its commit count; the check summary shows grouped kinds with counts, not one row per finding; output contains no `|` or box-drawing rules from a markdown table.
  - App: `len(tabNames) == 6`; `"6"` jumps to `TabSettings`; left/right do not switch tabs while the active tab is `Focused()`.
- Manual: `stardust` TUI, tab to Settings, edit `embed_model`, confirm `.stardust/config.toml` changed; run Reindex and Regenerate registry, confirm the result note and that `docs/INDEX.md` updated; tab to Drift, confirm single headers, fitted columns, and a grouped check summary at narrow and wide terminal widths.
- Adversarial: empty vault (no collections, clean check), a vault with many findings (grouping holds), a very narrow terminal (columns shrink without cut-off, boxes clip without overflow), saving an empty `reranker_url` (renders `(disabled)`, search still works), triggering an action twice fast (second is rejected by `busy`).

</details>

<details>
<summary><b>Out of scope</b></summary>
<br>

- Editing collection schemas or records from the tab.
- Service-level locking around `Rebuild`.
- Cross-tab refresh broadcasting.
- CLI `registry` refactor to consume `RegenerateRegistry`.
- URL validation of config endpoints.

</details>

<details>
<summary><b>Work breakdown</b></summary>
<br>

1. Service: add `SetConfig` and `RegenerateRegistry` (+ `defaultRegistryOrder`); tests.
2. Drift: rewrite `View`, replace `issueBox` with grouped `checkSummary`, replace `markdownBox` with `driftList` + `staleList` + `summaryLine`; drop the glamour import; update tests including the duplicate-header regression.
3. Settings tab: new `settings_tab.go` (state, Update, View, sub-views, inline editor, action commands); tests.
4. App wiring: `tabs.go` sixth tab; `app.go` field, fan-out, routing, `"6"` jump, left/right `Focused()` gate; app test.
5. Verify: build, vet, full test, manual TUI pass.

</details>

<details>
<summary><b>References</b></summary>
<br>

- ADR 0034 (sixth Settings tab), ADR 0035 (config and index actions through additive service methods), ADR 0036 (Drift renders typed slices, single headers).
- ADR 0030 five-tab model, ADR 0031 additive default path, ADR 0032 TUI reads through service, ADR 0011 cosmic palette, ADR 0016 vectors loud degradation, ADR 0018 drift by commit distance.
- `internal/tui/drift_tab.go`, `internal/tui/clean_list.go`, `internal/tui/status_tab.go`, `internal/service/governs.go`, `internal/service/index.go`, `internal/service/registry.go`, `internal/config/config.go`.
- exo-jobs `cli/src/internal/ui/settings_tab.go` (layout and interaction reference).

</details>
</content>
</invoke>
