---
title: Interactive Stardust TUI - implementation plan
status: Draft
date: 2026-06-26
related:
  - docs/specs/2026-06-26-2352-interactive-tui.md
---

Replace `internal/tui` with a five-tab cosmic Bubble Tea shell copied and adapted from the exo-jobs framework, reading through `internal/service`, leaving the CLI, MCP, and SDK untouched.

> 2026-06-27 amendment: follow ADR 0033, not the looser ADR 0029 language. This is a verbatim exo-jobs visual framework copy with only package/import path, palette, tab data, and pyfiglet `dos_rebel` `STARDUST` banner replacement.

## Header

- **Goal:** Bare `stardust` on a TTY launches a five-tab TUI (Search, Browse, Graph, Drift, Status) with an animated cosmic banner, recolored exo-jobs chrome, and every tab reading through the service. Headless output stays byte-identical.
- **Architecture:** Root `App` (Bubble Tea Model-Update-View) owns `tabs []TabModel` and routes to the active one. Chrome lives in `internal/tui` (`app.go`, `tabs.go`, `banner.go`, `styles.go`, `anim.go`) and `internal/tui/components` (`box.go`, `statusbar.go`, `tablegrid.go`, `sanitize.go`, `charm_adapters.go`). Backend holds `*service.Service`.
- **Tech stack:** Go 1.26.1, `charm.land/...v2` (bubbletea, bubbles, lipgloss, glamour), module `github.com/alxxpersonal/stardust`. Palette from `internal/ui`; markdown from `internal/render.GlamourRender`.
- **Global constraints:** Pure Go, never panic, errors wrapped with `%w` and lowercase operation-prefixed messages, doc comments on exports (third-person present tense), `// --- Section ---` separators, no em/en dashes anywhere, Bubble Tea messages are types not strings. CLI subcommands, `serve --mcp`, and the SDK MUST NOT be edited. Headless `query`/`bundle`/`check`/`status` output MUST stay byte-identical.

## Context

The old TUI is three concrete tabs (Search, Status, Graph) over `index.Store`/`embed.Client`. This plan deletes their bodies and rebuilds on the copied framework, reading through `service`. The launch gate already exists; do not edit it.

### Reuse map (read these first, confirm real signatures in source)

- `internal/cli/root.go:79-89` - launch gate (no-arg TTY -> `tui.Run`). Do not edit.
- `internal/tui/run.go` - backend + `Run(layout, cfg)`. Rewire to `service.Open`.
- `internal/tui/{app,tabs,styles,banner,anim,search_tab,status_tab,graph_tab}.go` - old TUI, replaced.
- `internal/ui/palette.go` - cosmic tokens (`ui.Primary`, `ui.Accent`, `ui.Text`, `ui.Muted`, `ui.Border`, `ui.Secondary`, `ui.CodeBg`, `ui.Bg`).
- `internal/render/glamour.go` - `render.GlamourRender(md, width)`, the only markdown renderer.
- `internal/service/service.go` - `Query`, `GetNote`, `Status`, `Graph`. `records.go` - `ListCollections`, `ListRecords`, `GetRecord`, `CollectionInfo`, `Record`, `RecordList`. `governs.go` - `DriftDocs`, `StaleDocs`, `DriftResult`, `StaleResult`. `check.go` - `Check`, `CheckResult`. `status_report.go` - `GatherStatus`, `VaultStatus`.
- Exo source (copy from, do not import): `~/Desktop/Exo Jobs/10-Code/Worktrees/10-Active/exo-jobs/cli/src/internal/ui/` and its `components/`.

### Execution rules

- Mirror these tasks into the harness todo tool; keep exactly one task in progress; mark complete the moment its validation loop is green; keep the todo tool in sync with the checkboxes below.
- Status markers: `[ ]` idle, `[wip]` in progress, `[x]` complete, `[f]` failed. Flip each box the instant its step is done; never batch.
- Each task ends with a validation loop. Do not exit a task until its command is green. On failure, fix the cause and re-run.
- Do not commit unless the user asks.

## Task 1: Scaffold - copy and recolor the framework

Goal: the chrome compiles in cosmic colors with no tabs wired yet.

- Files:
  - Create `internal/tui/components/sanitize.go`, `box.go`, `statusbar.go`, `tablegrid.go`, `charm_adapters.go` (copied from exo `components/`).
  - Create `internal/tui/components/{sanitize,box,statusbar,tablegrid,charm_adapters}_test.go` (copied, recolored).
  - Modify `internal/tui/styles.go` (recolor), `internal/tui/banner.go` (new wordmark), `internal/tui/anim.go` (keep tick, add shimmer if missing).
- Interfaces:
  - Produces: `components.HintItem{Key, Desc string}`, `components.Hint(key, desc) string`, `components.StatusBarFromItems(hints []HintItem, width int) string`, `components.Box(content string, width int) string`, `components.ActiveBox`, `components.ErrorBox(title, msg string, width int) string`, `components.Table`, `components.TableGrid`, `components.TableGridWithActiveRow`, `components.SanitizeText(string) string`, `components.SanitizeOneLine(string) string`, `components.NewExoTextInput`/`NewExoSpinner`/`NewExoTable`/`NewExoViewport` (rename to cosmic-neutral helpers, keep behavior).
  - Consumes: `internal/ui` tokens.

Steps:

- [ ] Copy `components/sanitize.go` + `sanitize_test.go` verbatim. Run `go test ./internal/tui/components/ -run Sanitize`. Watch it fail to compile (package not wired). Adjust the package name to `components`, re-run until green. Sanitize has no colors, so it is the anchor.
- [ ] Copy `components/box.go`, `statusbar.go`, `tablegrid.go`, `charm_adapters.go`. In each, replace every exo theme reference (`themePrimary`, `themeMuted`, `themeText`, `themeBorder`, `ColorPrimary`, `ColorAccent`, `ColorText`, `ColorMuted`, `ColorBorder`, `ColorSecondary`, `ColorCard`, `ColorBg`) with the cosmic mapping from the spec's recolor table (`ui.Primary`, `ui.Accent`, `ui.Text`, `ui.Muted`, `ui.Border`, `ui.Secondary`, `ui.CodeBg`, `ui.Bg`). Keep semantic status colors (`#34d399`, `#ff5555`, `#f59e0b`) verbatim. Delete any glamour `DarkStyleConfig` remap in copied files; the TUI uses `render.GlamourRender`.
- [ ] Copy the matching `_test.go` files; update any asserted hex literals to the cosmic hex. Run `go test ./internal/tui/components/`. Loop until green.
- [ ] Recolor `internal/tui/styles.go`: keep the `ui`-sourced color vars and the `titleStyle`/`mutedStyle`/`textStyle`/`accentStyle`/`boxStyle`/`pillStyle` set; add `successStyle`/`errorStyle`/`warnStyle` for the Drift tab from the semantic colors.
- [ ] Rewrite `internal/tui/banner.go`: a multi-line stardust ASCII wordmark with a star glyph motif, plus `RenderBannerAnimated(frame int) string` that shifts a `[]color.Color{ui.Primary, ui.Secondary, ui.Accent}` gradient down by `frame/8` and breathes the subtitle, and a static `RenderBanner() string`. No em/en dashes in the art.
- [ ] Run `go build ./internal/tui/...`. Loop until green.

Validation loop: `go test ./internal/tui/components/ && go build ./internal/tui/...` is green. Deliverable: cosmic chrome compiles, components tested.

## Task 2: Five-tab shell - app.go + tabs.go

Goal: the root model routes five empty tabs with banner, tab bar, and status bar.

- Files:
  - Modify `internal/tui/tabs.go` (TabModel interface, tab constants, `renderTabBar`).
  - Modify `internal/tui/app.go` (root Model adapted from exo `app.go`).
  - Modify `internal/tui/app_test.go` (tab-switch + Focused gating tests).
- Interfaces:
  - Produces: `TabModel` interface (`Init() tea.Cmd`, `Update(tea.Msg) (TabModel, tea.Cmd)`, `View(width, height int) string`, `Hints() []components.HintItem`, `Focused() bool`), constants `tabSearch=0, tabBrowse, tabGraph, tabDrift, tabStatus`, `tabNames []string`, `renderTabBar(active int) string`, `App` with `Init/Update/View`.
  - Consumes: `components`, `internal/ui`, `tea`.

Steps:

- [ ] Write `tabs.go`: the `TabModel` interface, the five constants, `tabNames = []string{"Search","Browse","Graph","Drift","Status"}`, and `renderTabBar` from exo (JoinHorizontal of active/inactive segments) recolored.
- [ ] Write a failing `app_test.go` case: construct `App` with five stub `TabModel`s (one stub reporting `Focused()==true`), send `tea.KeyPressMsg` "2" -> assert `active==tabBrowse`; set active to the focused stub, send "3" -> assert active unchanged (digit swallowed); send `tea.KeyPressMsg` "tab" -> assert wraps. Run `go test ./internal/tui/ -run TestApp`. Watch it fail.
- [ ] Implement `app.go`: `App{be, tabs []TabModel, active, width, height, frame}`. `newApp(be)` builds the five tabs. `Init` batches each tab's `Init` plus `animTick`. `Update` handles `tickMsg` (frame++ , re-arm), `WindowSizeMsg` (store + broadcast via a size message to each tab), global keys (`ctrl+c`, `tab`, `shift+tab`, digits `1`-`5` only when `!tabs[active].Focused()`), routes `searchDoneMsg`/`spinner.TickMsg` to their owning tab, else delegates to `tabs[active]`. `View` composes `RenderBannerAnimated(frame)`, `renderTabBar(active)`, `tabs[active].View(width, bodyHeight)`, and `components.StatusBarFromItems(tabs[active].Hints(), width)`, in `AltScreen`.
- [ ] Re-run `go test ./internal/tui/ -run TestApp`. Loop until green.

Validation loop: `go test ./internal/tui/ -run TestApp && go build ./internal/tui/...`. Deliverable: five-tab shell routes and switches.

## Task 3: Search tab

Goal: enter runs `svc.Query`, list shows hits, right pane glamour-renders the selection.

- Files: Modify `internal/tui/search_tab.go`; add `internal/tui/search_tab_test.go`.
- Interfaces:
  - Consumes: `backend.svc.Query(ctx, q, 12) (service.QueryResult, error)`, `render.GlamourRender`, `components.SanitizeOneLine`.
  - Produces: `searchTab` implementing `TabModel`; `searchDoneMsg{query string; result service.QueryResult}`.

Steps:

- [ ] Write a failing test: build `searchTab`, feed a `searchDoneMsg` whose `query` matches the input value, assert `t.result.Hits` is stored and `cursor==0`; feed one whose query does not match, assert it is ignored. Run `go test ./internal/tui/ -run TestSearch`. Watch it fail.
- [ ] Implement `searchTab`: focused `textinput`, `Dot` spinner styled `ui.Accent`. `Update` handles `spinner.TickMsg`, `searchDoneMsg` (drop stale by query mismatch), enter (`runSearch` async + spinner tick), up/down cursor. `Focused()` returns `t.input.Focused()`. `runSearch` calls `svc.Query` with a 30s timeout, returns `searchDoneMsg`.
- [ ] `View`: title + input; loading spinner; left hit list (active row marked, titles via `SanitizeOneLine`) and right preview box (`render.GlamourRender` of the selected note's file, falling back to the hit snippet); a pill showing `result.RetrievalMode`. `Hints()` returns enter/up-down plus the globals.
- [ ] Re-run `go test ./internal/tui/ -run TestSearch`. Loop until green.

Validation loop: `go test ./internal/tui/ -run TestSearch && go build ./internal/tui/...`. Deliverable: Search returns and previews hits.

## Task 4: Browse tab

Goal: collections list -> records list -> rendered record, with esc stepping back.

- Files: Create `internal/tui/browse_tab.go`, `browse_tab_test.go`.
- Interfaces:
  - Consumes: `svc.ListCollections(ctx) ([]service.CollectionInfo, error)`, `svc.ListRecords(ctx, name, nil, "-updated_at", 200, 0) (service.RecordList, error)`, `svc.GetRecord(ctx, path) (service.Record, error)`, `render.GlamourRender`, `components.TableGridWithActiveRow`, `components.SanitizeOneLine`.
  - Produces: `browseTab` implementing `TabModel`; `collectionsLoadedMsg`, `recordsLoadedMsg`, `recordLoadedMsg`.

Steps:

- [ ] Write a failing test: build `browseTab`, feed `collectionsLoadedMsg` with two collections, assert level 0 holds them; simulate enter -> assert it fires the `loadRecords` path (state moves to level 1 on `recordsLoadedMsg`); feed `recordLoadedMsg` -> assert level 2 holds the rendered body. Run `go test ./internal/tui/ -run TestBrowse`. Watch it fail.
- [ ] Implement `browseTab`: a `level` enum (`levelCollections, levelRecords, levelRecord`), a cursor per level, and the three load commands. `Init` loads collections. `Update`: enter descends (collection -> records -> record), esc ascends, up/down moves the cursor, `r` reloads the current level. `Focused()` returns false (no text input in v1).
- [ ] `View`: render the active level as a `TableGridWithActiveRow` (collections: name, records, description; records: title, path, updated). The record level renders `GetRecord` body via `render.GlamourRender` in a box. Sanitize every cell. `Hints()` reflects the level (enter open, esc back, up-down, r refresh).
- [ ] Re-run `go test ./internal/tui/ -run TestBrowse`. Loop until green.

Validation loop: `go test ./internal/tui/ -run TestBrowse && go build ./internal/tui/...`. Deliverable: Browse navigates collections to a rendered record.

## Task 5: Graph tab

Goal: notes/links counts, PageRank, orphans, broken links, refresh on `r`.

- Files: Modify `internal/tui/graph_tab.go`; add `internal/tui/graph_tab_test.go`.
- Interfaces:
  - Consumes: `svc.Graph(ctx) (service.GraphReport, error)`.
  - Produces: `graphTab` implementing `TabModel`; `graphLoadedMsg{report service.GraphReport; err error}`.

Steps:

- [ ] Write a failing test: build `graphTab`, feed `graphLoadedMsg` with a report (notes, links, two orphans, one broken, pagerank), assert fields stored and `loaded==true`; feed one with `err`, assert error surfaces. Run `go test ./internal/tui/ -run TestGraph`. Watch it fail.
- [ ] Implement `graphTab`: `Init` calls `svc.Graph`, `r` reloads. Store `report`/`err`/`loaded`.
- [ ] `View`: title, `N notes, M links`, a PageRank box (top entries), an Orphans box, a Broken-links box (`from -> [[target]]`), each overflow-clamped to height. Sanitize paths. `Hints()`: r refresh plus globals.
- [ ] Re-run `go test ./internal/tui/ -run TestGraph`. Loop until green.

Validation loop: `go test ./internal/tui/ -run TestGraph && go build ./internal/tui/...`. Deliverable: Graph loads and refreshes.

## Task 6: Drift tab

Goal: the coherence engine made visible - check report plus drift and stale warnings.

- Files: Create `internal/tui/drift_tab.go`, `drift_tab_test.go`.
- Interfaces:
  - Consumes: `svc.Check(ctx) (service.CheckResult, error)`, `svc.DriftDocs(ctx) (service.DriftResult, error)`, `svc.StaleDocs(ctx) (service.StaleResult, error)`, `render.GlamourRender`.
  - Produces: `driftTab` implementing `TabModel`; `driftLoadedMsg{check service.CheckResult; drift service.DriftResult; stale service.StaleResult; err error}`.

Steps:

- [ ] Write a failing test: build `driftTab`, feed `driftLoadedMsg` with a check (1 error, 2 warnings), a drift result (1 doc with bindings), a stale result (1 doc), assert all stored and `loaded==true`. Run `go test ./internal/tui/ -run TestDrift`. Watch it fail.
- [ ] Implement `driftTab`: `Init` runs the three reads concurrently-batched into one `driftLoadedMsg` (call them sequentially inside one `tea.Cmd`, or batch three messages and assemble; pick the single-message form for simpler state). `r` reloads.
- [ ] `View`: section 1 the check report (errors then warnings, each `severity kind path detail`, colored `errorStyle`/`warnStyle`); section 2 Drift (docs referencing moved code: `docPath type changedCommits`); section 3 Stale (governed docs: `docPath type changedCommits`). Prefer rendering the service-provided `Markdown` fields through `render.GlamourRender` so the tab matches `stardust check`/`stale`. Sanitize any raw path. `Hints()`: r refresh plus globals.
- [ ] Re-run `go test ./internal/tui/ -run TestDrift`. Loop until green.

Validation loop: `go test ./internal/tui/ -run TestDrift && go build ./internal/tui/...`. Deliverable: Drift shows check + drift + stale.

## Task 7: Status tab

Goal: root/kind/collections/index health from the full probe.

- Files: Modify `internal/tui/status_tab.go`; add `internal/tui/status_tab_test.go`.
- Interfaces:
  - Consumes: `service.GatherStatus(ctx, root) (service.VaultStatus, error)` (read `backend.svc.Layout.Root`); `Status` for index-health fallback.
  - Produces: `statusTab` implementing `TabModel`; `statusLoadedMsg{status service.VaultStatus; err error}`.

Steps:

- [ ] Write a failing test: build `statusTab`, feed `statusLoadedMsg` with a `VaultStatus` (initialized, kind, two collections, index health notes/vectors/commits-behind), assert stored and `loaded==true`. Run `go test ./internal/tui/ -run TestStatus`. Watch it fail.
- [ ] Implement `statusTab`: `Init` calls `GatherStatus(ctx, be.svc.Layout.Root)`, `r` reloads.
- [ ] `View`: a key-value `components.Table` for root, initialized, kind; a collections `TableGrid` (name, records); an index-health block (notes, vectors on/off with reason, commits-behind, last indexed SHA truncated to 12, embed model). Sanitize the root path. `Hints()`: r refresh plus globals.
- [ ] Re-run `go test ./internal/tui/ -run TestStatus`. Loop until green.

Validation loop: `go test ./internal/tui/ -run TestStatus && go build ./internal/tui/...`. Deliverable: Status shows the full probe.

## Task 8: Rewire run.go and confirm the launch path

Goal: the backend opens the service; bare `stardust` launches the five-tab TUI; `root.go` untouched.

- Files: Modify `internal/tui/run.go`. Do not edit `internal/cli/root.go`.
- Interfaces:
  - Consumes: `service.Open(ctx, layout.Root) (*service.Service, error)`.
  - Produces: `backend{svc *service.Service; hasVec bool}`, `Run(config.Layout, config.Config) error` (signature unchanged).

Steps:

- [ ] Rewrite `backend` to `{svc *service.Service; hasVec bool}`. Update every tab constructor to read `be.svc`. Delete the old `index`/`embed`/`config` fields and the direct `store`/`embed` usage.
- [ ] Rewrite `Run`: `service.Open(ctx, layout.Root)` wrapped `"open service: %w"`, `defer svc.Close()`, set `hasVec` from `svc.Status(ctx).Vectors`, run `tea.NewProgram(newApp(be))`. Keep the `(layout, cfg)` signature.
- [ ] Run `go build ./...` and `go vet ./...`. Loop until green.
- [ ] Confirm `internal/cli/root.go:79-89` is unchanged (git diff shows no edit to that file).

Validation loop: `go build ./... && go vet ./...` green; `git diff --stat internal/cli/` shows no TUI-driven change to command files. Deliverable: the binary launches the new TUI on a TTY.

## Task 9: Verification

Goal: prove the build, the tests, the byte-identical headless output, the TTY shell, and the unchanged MCP.

- [ ] `go build ./... && go vet ./... && go test ./...` all green. Fix any failure at its cause before proceeding.
- [ ] Headless byte-diff. Build the new binary to `/tmp/stardust-new`. Against a known vault, capture the pre-change binary and the new binary outputs for `query "test" --json`, `check`, `status`, `bundle "test"` with stdout piped (non-TTY), and `diff` each pair. Every diff MUST be empty. If any differs, the change leaked outside `internal/tui`; revert that leak.
- [ ] TTY smoke (manual, real terminal): run bare `stardust`. Confirm the cosmic banner animates; all five tab labels render; `1`-`5` and `tab`/`shift+tab` switch; Search returns hits with a glamour preview and a retrieval-mode pill; Browse opens a collection then a record; Graph, Drift, Status load and refresh on `r`; `ctrl+c` quits to a clean prompt with no leftover alt-screen.
- [ ] Adversarial sanitize: open (in Browse) a note containing a raw `\x1b[31m` escape; confirm the surrounding chrome is not recolored or corrupted.
- [ ] MCP unchanged: `stardust serve --mcp` starts and answers a `tools/list` request as before.
- [ ] Self-review gate: re-read the diff; confirm no em/en dashes, doc comments on every export, `%w` wrapping, no panic, no edit to any CLI command file, `serve`, or SDK.

Deliverable: a five-tab cosmic TUI on the no-arg TTY path, with provably unchanged headless and MCP surfaces.

## Verification summary

`go build ./...`, `go vet ./...`, `go test ./...` green; headless `query`/`bundle`/`check`/`status` byte-identical to the pre-change binary; bare `stardust` TTY smoke passes all five tabs; `serve --mcp` unchanged. Then run `stardust registry` to refresh `docs/INDEX.md`.
