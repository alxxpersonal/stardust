---
title: Interactive Stardust TUI (exo-jobs framework, cosmic recolor)
status: Implemented
version: 1
date: 2026-06-26
related:
  - internal/tui/app.go
  - internal/tui/run.go
  - internal/cli/root.go
  - internal/ui/palette.go
  - internal/render/glamour.go
  - internal/service/service.go
  - docs/adr/0029-copy-adapt-exo-jobs-tui-framework.md
  - docs/adr/0030-five-tab-stardust-tui-model.md
  - docs/adr/0031-additive-default-path-tui.md
  - docs/adr/0032-tui-reads-through-service-layer.md
---

Replace the three-tab Stardust TUI with a five-tab Bubble Tea shell copied and adapted from the exo-jobs TUI framework, recolored to the cosmic violet palette, surfacing Stardust functions (Search, Browse, Graph, Drift, Status) while leaving the CLI subcommands, the MCP server, and the SDK byte-for-byte unchanged.

> 2026-06-27 amendment: ADR 0033 supersedes ADR 0029. The visual framework is a verbatim exo-jobs copy, not a simplified adaptation. Only package names, import paths, palette tokens, tab data, and the pyfiglet `dos_rebel` `STARDUST` banner may differ.

<details>
<summary><b>Problem</b></summary>
<br>

The current TUI (`internal/tui/`) is a thin three-tab app (Search, Status, Graph) built directly over `index.Store` and `embed.Client`. It does not surface the collections browser, the doc-code coherence engine (drift and stale), or the full vault status probe, and its banner is a one-line wordmark with no cosmic motif. The exo-jobs repo already has a polished, domain-neutral Bubble Tea framework (animated banner, bordered boxes, hint status bar, custom table grid, ANSI-safe sanitizer, styled charm adapters, a TabModel interface) that Stardust can adopt. The framework lives in another repo's `internal/` tree, so Go forbids importing it; it must be copied in and adapted.

The replacement MUST stay strictly additive. Bare `stardust` on a TTY already launches the TUI; every subcommand, the MCP server (`stardust serve --mcp`), and the SDK MUST keep their exact behavior and headless output.

</details>

<details>
<summary><b>Context and background</b></summary>
<br>

Launch path today (`internal/cli/root.go:79-89`): the root command's `RunE` runs the TUI only when there is no subcommand and stdout is a TTY; piped invocation prints help. That gate already enforces the markdown-safe boundary, so the TUI is reachable on exactly one path and never pollutes headless output.

Backend today (`internal/tui/run.go`): opens `index.Store` and `embed.Client` directly and hands a `backend` struct to three concrete tab structs delegated by a switch in `App.Update`.

The core library is `internal/service` (ADR 0003: one core, thin surfaces). It exposes every read the new tabs need:

| Tab | Service method(s) | Returns |
|-----|-------------------|---------|
| Search | `Query(ctx, query, limit)` | `QueryResult{Hits []index.Hit, RetrievalMode, RetrievalReason, Reranked}` |
| Browse | `ListCollections(ctx)`, `ListRecords(ctx, name, filter, sort, limit, offset)`, `GetRecord(ctx, path)`, `GetNote(ctx, path)` | `[]CollectionInfo`, `RecordList`, `Record`, `Note` |
| Graph | `Graph(ctx)` | `GraphReport{Notes, Links, Orphans, Broken, PageRank}` |
| Drift | `Check(ctx)`, `DriftDocs(ctx)`, `StaleDocs(ctx)` | `CheckResult`, `DriftResult`, `StaleResult` |
| Status | `GatherStatus(ctx, start)` (falls back to `Status(ctx)`) | `VaultStatus{Root, Initialized, Kind, Collections, Index}` |

Palette source of truth is `internal/ui/palette.go` (ADR 0011): `Primary #a78bfa`, `Secondary #c4b5fd`, `Accent #f0abfc`, `Text #e9e7ff`, `Muted #7c7ca0`, `Border #4c4c6d`, `CodeBg #16161e`, `Bg #0a0a12`. Markdown rendering is already cosmic via `internal/render.GlamourRender`, shared by the CLI and the TUI, so the new TUI reuses it and drops the exo-jobs glamour style block entirely.

Exo-jobs framework source (reference only, copied not imported): `~/Desktop/Exo Jobs/10-Code/Worktrees/10-Active/exo-jobs/cli/src/internal/ui/`. Its domain-neutral files are `app.go`, `tabs.go`, `banner.go`, `styles.go`, and `components/{box,statusbar,tablegrid,sanitize,charm_adapters}.go`. Its jobs-domain files (`agent_tab.go`, `jobs_tab.go`, `crons_tab.go`, the other `*_tab.go`, and `internal/{agent,store,propose,scorer}`) are left behind.

</details>

<details>
<summary><b>Goals</b></summary>
<br>

1. Bare `stardust` on a TTY launches a five-tab cosmic TUI: Search, Browse, Graph, Drift, Status.
2. The framework chrome (banner, boxes, status bar, table grid, sanitizer, charm adapters, TabModel) is copied from exo-jobs into `internal/tui` and recolored to the cosmic palette in `internal/ui`, importing the existing tokens rather than redefining hexes.
3. A stardust ASCII wordmark banner with a cosmic motif animates a violet-to-pink gradient sweep synced to the global frame.
4. Every tab reads through `internal/service`, giving the TUI the same capability surface as the CLI and MCP server.
5. The CLI subcommands, the MCP server, and the SDK keep their exact behavior; headless `query`, `bundle`, `check`, and `status` output is byte-identical to before.

</details>

<details>
<summary><b>Non-goals</b></summary>
<br>

- No agent chat, cron execution, permission dialogs, or any jobs-domain tab. The exo-jobs `agent_tab.go`, `jobs_tab.go`, and `crons_tab.go` are not copied.
- No write paths from the TUI in v1 (no record create, no archive, no remember). Tabs are read-only views over the service.
- No vault switching, no quick-capture modal. Single vault per session, resolved at launch.
- No change to the glamour style; the existing `internal/render.GlamourRender` is the only markdown renderer.
- No new CLI flags, no change to `serve`, no change to the SDK.

</details>

<details>
<summary><b>Approach</b></summary>
<br>

### Layout

```
internal/tui/
  app.go            # root Model: 5-tab routing, frame ticker, size broadcast (adapted from exo app.go)
  tabs.go           # TabModel interface + tab constants + renderTabBar (adapted from exo tabs.go)
  banner.go         # stardust ASCII wordmark + animated cosmic gradient (adapted from exo banner.go)
  styles.go         # lipgloss styles, palette refs -> internal/ui tokens (recolored exo styles.go)
  anim.go           # frame tick + shimmer helpers (kept/extended)
  run.go            # opens *service.Service, launches the program (rewired)
  search_tab.go     # Search: svc.Query, list + glamour preview (TabModel)
  browse_tab.go     # Browse: collections/records tree, GetRecord rendered (TabModel)
  graph_tab.go      # Graph: svc.Graph notes/links/orphans/broken/pagerank (TabModel)
  drift_tab.go      # Drift: svc.Check + svc.DriftDocs + svc.StaleDocs (TabModel)
  status_tab.go     # Status: svc.GatherStatus root/kind/collections/index (TabModel)
  components/
    box.go          # bordered + active + error boxes, key-value table, width clamp
    statusbar.go    # HintItem, keycap hint, overflow-clamped status bar
    tablegrid.go    # custom grid table with active-row highlight
    sanitize.go     # ANSI escape + control + bidi stripping (markdown-safe)
    charm_adapters.go  # styled textinput, spinner, table, viewport, info table
```

### TabModel interface (copied from exo `tabs.go`, trimmed)

```go
// TabModel is one tab in the Stardust TUI. The root App owns the tabs and
// delegates Update and View to the active one.
type TabModel interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (TabModel, tea.Cmd)
	View(width, height int) string
	Hints() []components.HintItem
	Focused() bool // true when the tab owns text input, so digits are not tab switches
}
```

`Focused()` resolves the exact concern in today's `app.go:68`: digit keys switch tabs only when the active tab is not focused (the Search input owns them while typing).

### Root model (adapted from exo `app.go`)

```go
type App struct {
	be     *backend
	tabs   []TabModel
	active int
	width  int
	height int
	frame  int
}
```

`Update`: `tickMsg` advances `frame` and re-arms `animTick`; `WindowSizeMsg` stores size; global keys (`ctrl+c` quit, `tab`/`shift+tab` cycle, digits `1`-`5` jump when `!tabs[active].Focused()`); async result messages (search/spinner) route to their owning tab regardless of active tab so a mid-load tab switch does not strand them; everything else delegates to `tabs[active]`. `View` composes `RenderBannerAnimated(frame)`, `renderTabBar(active)`, the active tab body, and `StatusBarFromItems(tabs[active].Hints(), width)`, in `AltScreen`.

### Backend rewire (`run.go`)

```go
type backend struct {
	svc    *service.Service
	hasVec bool
}

func Run(layout config.Layout, cfg config.Config) error {
	ctx := context.Background()
	svc, err := service.Open(ctx, layout.Root)
	if err != nil {
		return fmt.Errorf("open service: %w", err)
	}
	defer func() { _ = svc.Close() }()
	be := &backend{svc: svc, hasVec: false}
	st, _ := svc.Status(ctx)
	be.hasVec = st.Vectors
	if _, err := tea.NewProgram(newApp(be)).Run(); err != nil {
		return fmt.Errorf("run tui: %w", err)
	}
	return nil
}
```

`Run`'s signature is unchanged, so `root.go:88` is untouched. `service.Open` re-resolves the root and reloads config, giving every tab the full method surface (ADR 0032).

### Recolor map (exo theme -> cosmic)

| exo-jobs token | exo hex | cosmic token | cosmic hex |
|----------------|---------|--------------|-----------|
| ColorPrimary / themePrimary | #a0a0a0 | `ui.Primary` | #a78bfa |
| ColorAccent | #909090 | `ui.Accent` | #f0abfc |
| ColorText / themeText | #e0e0e0 | `ui.Text` | #e9e7ff |
| ColorMuted / themeMuted | #404040 | `ui.Muted` | #7c7ca0 |
| ColorBorder / themeBorder | #2a2a2a | `ui.Border` | #4c4c6d |
| ColorSecondary | #686868 | `ui.Secondary` | #c4b5fd |
| ColorCard / CodeBg | #17171a | `ui.CodeBg` | #16161e |
| ColorBg | #0a0a0a | `ui.Bg` | #0a0a12 |

The exo `styles.go` glamour `DarkStyleConfig` remap is deleted; the TUI calls `render.GlamourRender` instead. Success/error/warning accents (#34d399, #ff5555, #f59e0b) are kept verbatim; they are semantic status colors, not chrome, matching how the CLI treats them.

### Banner

A multi-line stardust ASCII wordmark with a cosmic motif (a star glyph plus the STARDUST letterform). `RenderBannerAnimated(frame)` shifts a violet-to-pink gradient (`Primary -> Secondary -> Accent`) down by `frame/8` and breathes the subtitle, the same animation shape as exo `banner.go` with the gradient array swapped to cosmic tokens.

### Tab designs

- **Search**: focused `textinput`, enter runs `svc.Query(ctx, q, 12)` async, returns `searchDoneMsg{query, result}`; left pane is the hit list (active row via `tablegrid` or the existing cursor list), right pane is `render.GlamourRender` of the selected note's file (full file when readable, else the hit snippet). The retrieval mode (hybrid-semantic vs fts-only) is shown as a pill. `Focused()` returns true while the input has focus.
- **Browse**: two-level navigation. Level 0 lists collections from `ListCollections` (name, record count, description). Enter on a collection lists its records via `ListRecords(name, nil, "-updated_at", 200, 0)`. Enter on a record renders it via `GetRecord(path)` + `render.GlamourRender`. A free-path mode reads any note via `GetNote`. Esc steps back up a level.
- **Graph**: `svc.Graph(ctx)` on Init and on `r`; renders `Notes`/`Links` counts, top-N PageRank, orphan list, broken-link list, each in a bordered box, with overflow clamped.
- **Drift**: the coherence engine made visible. `svc.Check`, `svc.DriftDocs`, `svc.StaleDocs` on Init and on `r`. Renders the check report (errors then warnings) and below it the drift warnings (docs referencing moved code) and stale governed docs, each row showing doc path, type, and changed-commit count. Re-renders the service-provided `Markdown` fields through glamour so the TUI matches the CLI.
- **Status**: `svc.GatherStatus(ctx, root)` on Init and on `r`; renders root, initialized state, detected kind, the collections table with counts, and index health (notes, vectors on/off with reason, commits-behind, last indexed SHA, embed model).

### Markdown-safe discipline

`components/sanitize.go` strips the six ANSI escape families, bidi controls, and unicode control chars from any vault-derived string before it enters a bordered widget or a list row, so a hostile note cannot inject escape sequences into the TUI chrome. Glamour output is trusted (it is the renderer's own ANSI). The TUI runs only on a TTY; headless paths never construct the TUI, so raw markdown and JSON stay raw (ADR 0031).

</details>

<details>
<summary><b>Alternatives considered</b></summary>
<br>

- **Import the exo-jobs `internal/ui` package.** Rejected: Go forbids importing another module's `internal/` tree, and vendoring a sibling repo's internal package couples two unrelated products. Copy-and-adapt is the only viable reuse (ADR 0029).
- **Extend the existing three tabs in place.** Rejected: the existing tabs are concrete structs delegated by a switch with no shared chrome, no banner system, no sanitizer, and no active-row table. Bolting Browse and Drift onto that means rebuilding the framework anyway; copying the proven one is less work and yields the polish the ask requires.
- **Build a fresh framework from scratch.** Rejected: the exo-jobs framework is battle-tested for exactly this shape (multi-tab, animated banner, glamour detail, ANSI-safe). Rebuilding it invites the same bugs it already solved.
- **Route tabs directly at `index.Store`/`embed.Client` like today.** Rejected: that gives the TUI a different capability surface than the CLI and MCP server and re-implements query fusion, drift math, and status probing that already live in the service. Reading through the service keeps parity (ADR 0032).
- **Keep the exo glamour style block.** Rejected: Stardust already has one cosmic glamour renderer shared with the CLI; a second style would drift. Reuse `render.GlamourRender`.

</details>

<details>
<summary><b>Risks</b></summary>
<br>

- **Headless output regression.** The hard constraint is byte-identical headless output. Mitigation: the change is confined to `internal/tui` plus its new `components/` subtree; no CLI command file, no `serve`, no SDK is touched. A verification step diffs `query`/`bundle`/`check`/`status` non-TTY output against the current binary.
- **Bubble Tea v2 API drift between repos.** Exo-jobs may pin different `charm.land/...v2` versions. Mitigation: copy source, not deps; adapt each file to Stardust's pinned versions and let the compiler find the gaps. Build green is the gate.
- **TabModel interface churn.** The exo interface has `StatusLine()`/`HeaderLabel()` that Stardust may not need. Mitigation: trim the interface to what the root App calls (`Init`, `Update`, `View`, `Hints`, `Focused`); drop the rest.
- **Sanitize over-stripping.** Aggressive control stripping could mangle legitimate unicode in notes. Mitigation: sanitize only list rows and box chrome strings, never the glamour-rendered body, and cover with the copied `sanitize_test.go` cases.
- **Browse on a vault with many records.** `ListRecords` with no limit could be large. Mitigation: cap at 200 with a "more" indicator, paginate on demand.

</details>

<details>
<summary><b>Open questions</b></summary>
<br>

- Should Drift fold `StaleDocs` (governs+Implemented) and `DriftDocs` (reference-bound) into one list or show them as two sections? Default: two labeled sections, drift first.
- Should Browse expose the free-path `GetNote` reader as a key (`o` to open a path) or defer it? Default: defer to a follow-up; v1 navigates collections only.
- Does the active-row `tablegrid` replace the cursor list in Search, or only in Browse/Status? Default: use `tablegrid` for Browse and Status tables; keep the lighter cursor list for Search hits.

</details>

<details>
<summary><b>Verification</b></summary>
<br>

- `go build ./...` and `go vet ./...` are green.
- `go test ./internal/tui/... ./internal/service/...` passes, including the copied component tests (`sanitize_test.go`, `box_test.go`, `tablegrid_test.go`, `statusbar_test.go`, `charm_adapters_test.go`) adapted to the cosmic palette.
- Headless regression: build the new binary, run `stardust query "x" --json`, `stardust check`, `stardust status`, `stardust bundle "x"` piped (non-TTY) and diff against the pre-change binary's output. Diff MUST be empty.
- TTY smoke: run bare `stardust` in a real terminal; confirm the banner animates, all five tabs render, digit keys and tab/shift+tab switch, Search returns hits with a glamour preview, Browse opens a collection then a record, Graph/Drift/Status load and refresh on `r`, `ctrl+c` quits cleanly.
- `stardust serve --mcp` still starts and answers a `tools/list` request unchanged.
- Adversarial: open a note containing a raw ANSI escape sequence in Browse; confirm the chrome is not corrupted (sanitizer holds).

</details>

<details>
<summary><b>Out of scope</b></summary>
<br>

- The MCP server, the HTTP/JSON API, and the SDK surfaces.
- Any write/mutation tab (create record, archive, remember).
- Vault switching and quick-capture.
- New glamour styling.

</details>

<details>
<summary><b>Work breakdown</b></summary>
<br>

1. Scaffold: copy and recolor the framework (`components/{sanitize,box,statusbar,tablegrid,charm_adapters}.go`), recolor `styles.go` to `internal/ui` tokens, copy and recolor `banner.go` with the stardust wordmark, adapt `app.go` + `tabs.go` to a five-tab `TabModel` shell, rewire `run.go` to `service.Open`.
2. Search tab over `svc.Query`.
3. Browse tab over `ListCollections` / `ListRecords` / `GetRecord`.
4. Graph tab over `svc.Graph`.
5. Drift tab over `svc.Check` / `svc.DriftDocs` / `svc.StaleDocs`.
6. Status tab over `svc.GatherStatus`.
7. Wire and confirm the bare-`stardust` launch path; smoke test the shell.
8. Verification: build, vet, tests, headless byte-diff, TTY smoke, MCP unchanged.

</details>

<details>
<summary><b>References</b></summary>
<br>

- ADR 0029 copy-adapt-exo-jobs-tui-framework, ADR 0030 five-tab-stardust-tui-model, ADR 0031 additive-default-path-tui, ADR 0032 tui-reads-through-service-layer.
- ADR 0003 one-method-registry-multi-transport (the service is the one core), ADR 0011 stardust-cosmic-colorscheme (palette source of truth).
- `internal/cli/root.go:79-89` (launch gate), `internal/tui/run.go` (backend), `internal/service/service.go` and `internal/service/{records,governs,status_report,check}.go` (tab data), `internal/render/glamour.go` (shared renderer), `internal/ui/palette.go` (tokens).
- Exo-jobs framework source: `~/Desktop/Exo Jobs/10-Code/Worktrees/10-Active/exo-jobs/cli/src/internal/ui/`.

</details>
