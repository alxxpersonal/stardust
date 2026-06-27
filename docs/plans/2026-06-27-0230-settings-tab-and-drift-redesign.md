---
title: Settings tab and Drift tab redesign
status: Draft
date: 2026-06-27
related:
  - docs/specs/2026-06-27-0230-settings-tab-and-drift-redesign.md
  - docs/adr/0034-sixth-settings-tui-tab.md
  - docs/adr/0035-config-and-index-actions-through-service.md
  - docs/adr/0036-drift-renders-typed-slices-single-headers.md
---

Add a sixth Settings tab (config view/edit, collections + schema/counts, index actions) through additive service methods, and redesign the Drift tab into single-header, scannable clean lists that fix the duplicate-header bug and the cut-off column.

# Header

- Goal: ship the Settings tab and the Drift redesign per the spec, reading and writing only through `internal/service`, no CLI/MCP/SDK changes.
- Architecture: Bubble Tea Model-Update-View. Each tab is a `TabModel` (`internal/tui/tabs.go`). Tabs read through `be.svc` (`internal/service`). Lists use `renderCleanList` (`internal/tui/clean_list.go`); boxes use `animatedDoubleBox` / `animatedRoundedBox` (`internal/tui/anim.go`); palette per ADR 0011.
- Tech stack: Go 1.26, charm v2 (bubbletea, lipgloss, bubbles textinput), `github.com/alxxpersonal/stardust` module.
- Global constraints: never panic, wrap errors with `%w`, `// --- Section ---` separators, doc comments on exports in third-person present tense, NO em/en dashes anywhere, messages are types not strings, all API types carry `json` tags. Read/write only through `internal/service`.

# Context

The interactive TUI (ADR 0030, 0032) has five tabs and no config or action surface, and the Drift tab double-renders its keycap headers and cuts off a column. This plan adds Settings as the sixth tab and rewrites the Drift rendering to use the typed result slices.

# Reuse map (read first)

- `internal/tui/tabs.go` - tab constants, `tabNames`, `TabModel` interface.
- `internal/tui/app.go` - App model, fan-out, key routing, `tableWidth`, `centerBlockUniform`, `countViewLines`.
- `internal/tui/status_tab.go` - the closest model for a read-mostly tab and the collections clean list.
- `internal/tui/clean_list.go` - `renderCleanList`, `cleanListColumn`, `cleanListRow`, `cleanListCountLabel`.
- `internal/tui/drift_tab.go` - the file to rewrite (rendering half).
- `internal/tui/anim.go`, `internal/tui/styles.go` - boxes, styles, `clipLines`, `withCommonHints`, `boolWord`.
- `internal/tui/components/charm_adapters.go` - `NewExoTextInput(placeholder)`.
- `internal/service/service.go` - `Service`, `s.Config`, `s.embed`, `s.rerank`, `Status`.
- `internal/service/index.go` - `Index(ctx, "")`, `Rebuild(ctx)`, `IndexStats`.
- `internal/service/registry.go` - `Registry(order)`; `internal/service/manifest.go` - `RefreshManifest`.
- `internal/service/records.go` - `ListCollections`, `GetCollection`, `CollectionInfo` (carries `Fields`).
- `internal/config/config.go` - `Config`, `Load`, `Save`, `Layout.Config()`.
- `internal/manifest/registry.go` - `WriteRegistry`.

Confirm the real signatures in source before wiring; do not trust the plan over the code.

# Task 1: Service methods (SetConfig, RegenerateRegistry)

Files:
- Modify: `internal/service/service.go` (add `SetConfig`)
- Modify: `internal/service/registry.go` (add `RegenerateRegistry`, `defaultRegistryOrder`)
- Test: `internal/service/config_actions_test.go` (new)

Interfaces:
- Produces: `(*Service).SetConfig(cfg config.Config) error`, `(*Service).RegenerateRegistry(ctx context.Context) error`
- Consumes: `config.Save`, `embed.New`, `rerank.New`, `s.Registry`, `manifest.WriteRegistry`, `s.RefreshManifest`

Steps:
- [ ] Write failing test `internal/service/config_actions_test.go`: build a temp vault with `.stardust/config.toml` (reuse the existing service test harness/helpers in `service_test`/`index_test`). Assert: after `SetConfig` with a changed `EmbedModel`, `Load(layout.Config())` returns the new value, `svc.Config.EmbedModel` equals it, and `svc.Status(ctx).EmbedModel` reflects it. For `RegenerateRegistry`, assert `docs/INDEX.md` exists under the root after the call and is non-empty.
- [ ] Run it: `go test ./internal/service/ -run 'ConfigActions'` (red).
- [ ] Add to `internal/service/service.go` under a `// --- Config mutation ---` section:

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
```

- [ ] Add to `internal/service/registry.go` (it already imports `os`? confirm; add `os` and `path/filepath` if missing - both likely already present via other service files, but this file needs its own imports):

```go
// defaultRegistryOrder is the fixed collection order for the docs registry,
// mirroring the CLI registry command.
var defaultRegistryOrder = []string{"specs", "plans", "adr", "research"}

// RegenerateRegistry regenerates docs/INDEX.md from the docs collections and
// refreshes the pinned agent manifest, mirroring `stardust registry`. It writes
// to docs/INDEX.md under the vault root.
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

- [ ] Confirm `registry.go` imports include `os`, `path/filepath`, `fmt`, `context`, and `manifest`; add any missing. `goimports` to group stdlib / external / local.
- [ ] Run `go test ./internal/service/ -run 'ConfigActions'` (green). Then `go vet ./internal/service/`.
- [ ] Validation loop: do not exit until the service tests pass and vet is clean.

# Task 2: Drift redesign (single headers, scannable lists)

Files:
- Modify: `internal/tui/drift_tab.go` (rewrite `View` and the render helpers; delete `markdownBox`, replace `issueBox`)
- Test: `internal/tui/drift_tab_test.go` (update + add regression)

Interfaces:
- Consumes: `t.check.Issues`, `t.drift.Docs` (`[]service.DriftDoc`, each with `Type`, `Title`, `DocPath`, `Bindings []DriftBinding{File, ChangedCommits}`), `t.stale.Docs` (`[]service.GovernedDoc` with `Type`, `Title`, `Status`, `DocPath`, `ChangedCommits`, `Matched`), `renderCleanList`, `clipLines`.
- Produces: a vertical-stack `View`; `summaryLine`, `driftList`, `staleList`, `checkSummary` helpers.

Steps:
- [ ] Update `drift_tab_test.go` to the new rendering and add the regression test, then run red:

```go
func TestDriftRendersSingleHeaders(t *testing.T) {
	tab := newDriftTab(nil)
	tab.loaded = true
	tab.drift = service.DriftResult{Docs: []service.DriftDoc{
		{DocPath: "docs/specs/a.md", Title: "Spec A", Type: "spec",
			Bindings: []service.DriftBinding{{File: "internal/a.go", ChangedCommits: 3}}},
	}}
	tab.stale = service.StaleResult{Docs: []service.GovernedDoc{
		{DocPath: "docs/plans/b.md", Title: "Plan B", Type: "plan", Status: "Implemented",
			ChangedCommits: 2, Matched: []string{"internal/b.go"}},
	}}
	out := components.SanitizeText(tab.View(120, 40))
	require.Equal(t, 1, strings.Count(out, "Drifted Docs"))
	require.Equal(t, 1, strings.Count(out, "Stale Docs"))
	require.Contains(t, out, "internal/a.go") // referenced moved file
	require.Contains(t, out, "Spec A")
}

func TestDriftCheckSummaryGroups(t *testing.T) {
	tab := newDriftTab(nil)
	tab.loaded = true
	tab.check = service.CheckResult{Errors: 2, Warnings: 1, Issues: []service.Issue{
		{Severity: "error", Kind: "broken-link", Path: "a.md", Detail: "x"},
		{Severity: "error", Kind: "broken-link", Path: "b.md", Detail: "y"},
		{Severity: "warn", Kind: "orphan", Path: "c.md", Detail: "z"},
	}}
	out := components.SanitizeText(tab.checkSummary(110, 20))
	require.Contains(t, out, "broken-link")
	require.Contains(t, out, "orphan")
	require.NotContains(t, out, "|")
}
```

  Keep `TestDriftLoadedStoresReports`. Delete or rewrite `TestDriftIssuesRenderCleanList` to target `checkSummary`. Add `"strings"` to the test imports.
- [ ] Run `go test ./internal/tui/ -run 'Drift'` (red).
- [ ] In `drift_tab.go`, replace `View` (lines ~108-139) with a vertical stack:

```go
// View renders the coherence report as a vertical stack of clean lists.
func (t DriftTab) View(width, height int) string {
	if width <= 0 {
		width = t.width
	}
	if height <= 0 {
		height = t.height
	}
	if t.err != nil {
		return centerBlockUniform(components.ErrorBox("drift failed", t.err.Error(), tableWidth(width)), width)
	}
	if !t.loaded {
		return centerOverlay(animatedBox(MutedStyle.Render("checking coherence..."), t.frame), width, height)
	}

	cardW := tableWidth(width)
	avail := height - 4
	if avail < 12 {
		avail = 12
	}
	driftH := avail * 35 / 100
	staleH := avail * 35 / 100
	checkH := avail - driftH - staleH
	driftH, staleH, checkH = atLeast(driftH, 6), atLeast(staleH, 6), atLeast(checkH, 6)

	var b strings.Builder
	b.WriteString(t.summaryLine(cardW))
	b.WriteString("\n\n")
	b.WriteString(animatedDoubleBox("DRIFTED DOCS", t.driftList(cardW, driftH), t.frame))
	b.WriteString("\n\n")
	b.WriteString(animatedDoubleBox("STALE DOCS", t.staleList(cardW, staleH), t.frame))
	b.WriteString("\n\n")
	b.WriteString(animatedRoundedBox("CHECK FINDINGS", t.checkSummary(cardW, checkH), t.frame))
	return centerBlockUniform(b.String(), width)
}

// atLeast clamps n up to floor.
func atLeast(n, floor int) int {
	if n < floor {
		return floor
	}
	return n
}
```

- [ ] Add `summaryLine`:

```go
// summaryLine renders the one-line coherence summary, colored by worst severity.
func (t DriftTab) summaryLine(width int) string {
	style := MutedStyle
	if t.check.Errors > 0 {
		style = ErrorStyle
	} else if t.check.Warnings > 0 || len(t.drift.Docs) > 0 || len(t.stale.Docs) > 0 {
		style = WarningStyle
	}
	return style.Render(fmt.Sprintf("%d errors  ·  %d warnings  ·  %d drifted  ·  %d stale",
		t.check.Errors, t.check.Warnings, len(t.drift.Docs), len(t.stale.Docs)))
}
```

- [ ] Replace `markdownBox` with `driftList` and `staleList`:

```go
// driftList renders drifted docs, one row per moved binding, in fitted columns.
func (t DriftTab) driftList(width, height int) string {
	if len(t.drift.Docs) == 0 {
		return SuccessStyle.Render("no drifted docs")
	}
	var rows []cleanListRow
	for _, doc := range t.drift.Docs {
		for _, bind := range doc.Bindings {
			rows = append(rows, cleanListRow{Cells: []string{
				doc.Type,
				components.SanitizeOneLine(doc.Title),
				components.SanitizeOneLine(doc.DocPath),
				components.SanitizeOneLine(bind.File),
				fmt.Sprintf("%d", bind.ChangedCommits),
			}})
		}
	}
	cols := []cleanListColumn{
		{Header: "Type", MinWidth: 4, MaxWidth: 8, Muted: true},
		{Header: "Title", MinWidth: 16, Primary: true},
		{Header: "Doc", MinWidth: 14, MaxWidth: 38, Muted: true, Underline: true},
		{Header: "Referenced", MinWidth: 14, MaxWidth: 38, Muted: true, Underline: true},
		{Header: "Commits", MinWidth: 7, MaxWidth: 8, Align: lipgloss.Right, Count: true},
	}
	label := cleanListCountLabel(len(t.drift.Docs), "doc")
	return clipLines(renderCleanList("Drifted Docs", label, cols, rows, width, -1), height)
}

// staleList renders implemented docs whose governed code changed after the doc.
func (t DriftTab) staleList(width, height int) string {
	if len(t.stale.Docs) == 0 {
		return SuccessStyle.Render("no stale docs")
	}
	rows := make([]cleanListRow, 0, len(t.stale.Docs))
	for _, doc := range t.stale.Docs {
		rows = append(rows, cleanListRow{Cells: []string{
			doc.Type,
			components.SanitizeOneLine(doc.Title),
			components.SanitizeOneLine(doc.Status),
			components.SanitizeOneLine(doc.DocPath),
			fmt.Sprintf("%d", doc.ChangedCommits),
			components.SanitizeOneLine(strings.Join(doc.Matched, ", ")),
		}})
	}
	cols := []cleanListColumn{
		{Header: "Type", MinWidth: 4, MaxWidth: 8, Muted: true},
		{Header: "Title", MinWidth: 16, MaxWidth: 36, Primary: true},
		{Header: "Status", MinWidth: 7, MaxWidth: 12, Muted: true},
		{Header: "Doc", MinWidth: 14, MaxWidth: 34, Muted: true, Underline: true},
		{Header: "Commits", MinWidth: 7, MaxWidth: 8, Align: lipgloss.Right, Count: true},
		{Header: "Matched", MinWidth: 12, Muted: true},
	}
	label := cleanListCountLabel(len(t.stale.Docs), "doc")
	return clipLines(renderCleanList("Stale Docs", label, cols, rows, width, -1), height)
}
```

- [ ] Replace `issueBox` with grouped `checkSummary`:

```go
// checkSummary groups check findings by kind with a count and a sample, errors
// first, so the report is scannable instead of one row per finding.
func (t DriftTab) checkSummary(width, height int) string {
	if len(t.check.Issues) == 0 {
		return SuccessStyle.Render("clean")
	}
	type group struct {
		severity, kind, sample string
		count                  int
	}
	order := []string{}
	byKind := map[string]*group{}
	for _, is := range t.check.Issues {
		g, ok := byKind[is.kind()]
		if !ok {
			g = &group{severity: is.Severity, kind: is.Kind, sample: is.Detail}
			byKind[is.kind()] = g
			order = append(order, is.kind())
		}
		g.count++
		if is.Severity == "error" {
			g.severity = "error"
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		gi, gj := byKind[order[i]], byKind[order[j]]
		if (gi.severity == "error") != (gj.severity == "error") {
			return gi.severity == "error"
		}
		return order[i] < order[j]
	})
	rows := make([]cleanListRow, 0, len(order))
	for _, k := range order {
		g := byKind[k]
		rows = append(rows, cleanListRow{Cells: []string{
			g.severity, g.kind, fmt.Sprintf("%d", g.count), components.SanitizeOneLine(g.sample),
		}})
	}
	cols := []cleanListColumn{
		{Header: "Severity", MinWidth: 5, MaxWidth: 8, Severity: true},
		{Header: "Kind", MinWidth: 10, MaxWidth: 20, Muted: true},
		{Header: "Count", MinWidth: 5, MaxWidth: 7, Align: lipgloss.Right, Count: true},
		{Header: "Example", MinWidth: 16, Primary: true},
	}
	label := fmt.Sprintf("%d error / %d warn", t.check.Errors, t.check.Warnings)
	return clipLines(renderCleanList("Check Findings", label, cols, rows, width, -1), height)
}
```

  Note: `is.kind()` above is shorthand; group by `is.Kind` directly (use `is.Kind` as the map key and slice element, no helper method). Adjust the code to key on `is.Kind`.
- [ ] Drop the now-unused `render` import and the `markdownBox` method. Keep `fmt`, `strings`, `sort` (add `sort`), `time`, `context`, `lipgloss`, `service`, `components`. Run `goimports`.
- [ ] Keep `StatusLine`, `Hints`, `HeaderLabel`, `Focused`, `Update`, `Init`, `load`, `Resize` as-is.
- [ ] Run `go test ./internal/tui/ -run 'Drift'` (green). Then `go vet ./internal/tui/`.
- [ ] Validation loop: do not exit until the Drift tests pass and there are no duplicate headers in the rendered output.

# Task 3: Settings tab

Files:
- Create: `internal/tui/settings_tab.go`
- Test: `internal/tui/settings_tab_test.go`

Interfaces:
- Produces: `SettingsTab` satisfying `TabModel`; `NewSettingsTab(be *backend) SettingsTab`; `newSettingsTab(be *backend) SettingsTab`; `settingsActionMsg{kind string; summary string; err error}`.
- Consumes: `be.svc.Config`, `be.svc.SetConfig`, `be.svc.Index`, `be.svc.Rebuild`, `be.svc.RegenerateRegistry`, `be.svc.ListCollections`, `be.svc.GetCollection`, `components.NewExoTextInput`.

Steps:
- [ ] Write `internal/tui/settings_tab.go`. Structure (fill in bodies; this is the contract):

```go
package tui

// --- Settings Messages ---

// settingsActionMsg carries the result of an async index action.
type settingsActionMsg struct {
	kind    string
	summary string
	err     error
}

// settingsCollectionsMsg carries the loaded collections for the settings view.
type settingsCollectionsMsg struct {
	collections []service.CollectionInfo
	err         error
}

// --- Settings rows ---

type settingsRowKind int

const (
	rowText settingsRowKind = iota // enter opens an inline text editor
	rowAction                      // enter runs an action or opens a sub-view
)

type settingsRow struct {
	key   string
	label string
	kind  settingsRowKind
}

var settingsRows = []settingsRow{
	{"embed_model", "Embed model", rowText},
	{"ollama_url", "Ollama URL", rowText},
	{"reranker_url", "Reranker URL", rowText},
	{"reranker_model", "Reranker model", rowText},
	{"ignore", "Ignore list", rowAction},
	{"reindex", "Reindex", rowAction},
	{"rebuild", "Rebuild index", rowAction},
	{"registry", "Regenerate registry", rowAction},
	{"collections", "Inspect collections", rowAction},
}

type settingsSub int

const (
	settingsNone settingsSub = iota
	settingsIgnore
	settingsCollections
	settingsSchema
)

// SettingsTab views and edits the per-vault config, lists collections, and runs
// index actions through the service layer.
type SettingsTab struct {
	be     *backend
	cfg    config.Config
	cursor int
	note   string
	busy   bool

	editing   bool
	editKey   string
	editInput textinput.Model

	sub         settingsSub
	collections []service.CollectionInfo
	colCursor   int
	schemaName  string

	ignoreCursor int
	addingIgnore bool
	ignoreInput  textinput.Model

	width  int
	height int
	frame  int
}

type settingsTab = SettingsTab

func NewSettingsTab(be *backend) SettingsTab { /* seed cfg from be.svc.Config (guard nil), build inputs blurred */ }
func newSettingsTab(be *backend) SettingsTab { return NewSettingsTab(be) }

func (t *SettingsTab) Resize(width, height int) { t.width, t.height = width, height }
func (t SettingsTab) Init() tea.Cmd            { return t.loadCollections() } // for the read-only COLLECTIONS box
func (t SettingsTab) Update(msg tea.Msg) (TabModel, tea.Cmd) { /* see below */ }
func (t SettingsTab) View(width, height int) string          { /* CONFIG + COLLECTIONS + ACTIONS, or sub-view */ }
func (t SettingsTab) Hints() []components.HintItem            { /* mode-aware */ }
func (t SettingsTab) Focused() bool { return t.editing || t.sub != settingsNone }
func (t SettingsTab) StatusLine() string { return t.note }
func (t SettingsTab) HeaderLabel() string { return "settings · config + index" }
```

- [ ] `Update`: handle `settingsActionMsg` (clear `busy`, set `note` from summary or err), `settingsCollectionsMsg` (store collections), `tea.KeyPressMsg` -> `handleKey`. When `editing`, route non-handled keys to `editInput.Update`. When `addingIgnore`, route to `ignoreInput.Update`.
- [ ] `handleKey` main list: `up`/`down` move cursor (clamp to `len(settingsRows)`), `enter` -> `activateRow`, `r` -> reload collections. Sub-views get their own handlers (`handleIgnoreKey`, `handleCollectionsKey`, `handleSchemaKey`, `handleEditKey`) mirroring exo `handleCatsKey` / `handleEditKey`.
- [ ] `activateRow`:
  - `rowText` (`embed_model`, `ollama_url`, `reranker_url`, `reranker_model`): set `editing`, `editKey`, seed `editInput` from `settingsValueRaw(key, cfg)`, focus the input, return `textinput.Blink`.
  - `ignore`: `sub = settingsIgnore`, `ignoreCursor = 0`.
  - `reindex` / `rebuild` / `registry`: if `busy` return with note "busy"; else set `busy`, `note = "running..."`, return the matching action command.
  - `collections`: `sub = settingsCollections`, `colCursor = 0`.
- [ ] `handleEditKey`: `esc` cancels (blur, clear editing); `enter` saves: `trimmed := strings.TrimSpace(editInput.Value())`, set the field on `t.cfg`, call `t.be.svc.SetConfig(t.cfg)` (guard `be`/`svc` nil), on error set `note` to the error and keep `cfg` unchanged, on success `note = "saved"`; clear editing. Other keys -> `editInput.Update`.
- [ ] Action commands (each returns `tea.Cmd` that runs the service call in its goroutine and returns a `settingsActionMsg`):

```go
func (t SettingsTab) reindexCmd() tea.Cmd {
	be := t.be
	return func() tea.Msg {
		if be == nil || be.svc == nil {
			return settingsActionMsg{kind: "reindex", err: fmt.Errorf("settings: service is not open")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		stats, err := be.svc.Index(ctx, "")
		if err != nil {
			return settingsActionMsg{kind: "reindex", err: err}
		}
		return settingsActionMsg{kind: "reindex", summary: fmt.Sprintf("reindexed %d, skipped %d, deleted %d, vectors %s",
			stats.Indexed, stats.Skipped, stats.Deleted, boolWord(stats.Vectors))}
	}
}
```

  `rebuildCmd` mirrors with `be.svc.Rebuild(ctx)`. `registryCmd` calls `be.svc.RegenerateRegistry(ctx)` and returns `summary: "regenerated docs/INDEX.md"`.
- [ ] `loadCollections` command returns `settingsCollectionsMsg` from `be.svc.ListCollections(ctx)` (guard nil), mirroring `browse_tab.loadCollections`.
- [ ] `View`: when `sub == settingsIgnore` render the ignore sub-view (clean list of `t.cfg.Ignore`, `n` add, `d` delete, mirror exo `renderCats`); when `settingsCollections` render a collections clean list (Collection / Records / Fields / Path / Description); when `settingsSchema` render the selected collection's fields clean list (Field / Type / Required / Enum). Otherwise the three-box main view:
  - CONFIG: `animatedDoubleBox("CONFIG", t.renderRows(cardW))` where each row is `cursor + label(padRightCell 18) + value`. Value = `SuccessStyle` for non-empty scalars, `MutedStyle "(disabled)"` for empty `reranker_url`, action rows show a hint (`"edit"`, `"manage"`, `"run"`). Focused editing row swaps value for `editInput.View()`.
  - COLLECTIONS: `animatedRoundedBox("COLLECTIONS", t.collectionsList(cardW, height/3))` reusing the Status tab clean list with an added `Fields` count column.
  - ACTIONS row text lives inside CONFIG rows already (reindex/rebuild/registry/collections are rows); the COLLECTIONS box is read-only. Keep one cursor over `settingsRows`.
- [ ] `Hints`: editing -> enter save / esc cancel; ignore sub-view -> nav / n add / d delete / esc back; collections sub-view -> nav / enter schema / esc back; schema -> esc back; default -> nav / enter / r refresh. Wrap with `withCommonHints` for the default mode only.
- [ ] `settingsValueRaw(key, cfg)` returns the plain string for the editor seed (`cfg.EmbedModel`, `cfg.OllamaURL`, `cfg.RerankerURL`, `cfg.RerankerModel`). `setField(key, val)` writes the trimmed value onto a `*config.Config`.
- [ ] Add a `padRightCell` helper if not already present in the tui package (exo has one); if absent, add a small local one or reuse `components`-level padding.
- [ ] Write `internal/tui/settings_tab_test.go` (nil backend, pure view/cursor logic; service calls are guarded so nil `be` is safe):

```go
func TestSettingsRendersConfigValues(t *testing.T) {
	tab := newSettingsTab(nil)
	tab.cfg = config.Config{EmbedModel: "bge-m3", OllamaURL: "http://localhost:11434",
		Ignore: []string{".git"}, RerankerURL: ""}
	out := components.SanitizeText(tab.View(120, 40))
	require.Contains(t, out, "Embed model")
	require.Contains(t, out, "bge-m3")
	require.Contains(t, out, "(disabled)") // empty reranker url
}

func TestSettingsCursorMoves(t *testing.T) {
	tab := newSettingsTab(nil)
	updated, _ := tab.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	tab = updated.(settingsTab)
	require.Equal(t, 1, tab.cursor)
}

func TestSettingsEditFocuses(t *testing.T) {
	tab := newSettingsTab(nil)
	updated, _ := tab.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // enter on first text row
	tab = updated.(settingsTab)
	require.True(t, tab.Focused())
}
```

  Confirm the real `tea.KeyPressMsg` construction against `internal/tui/browse_tab_test.go` / `search_tab_test.go` and match their pattern exactly (key code vs string).
- [ ] Run `go test ./internal/tui/ -run 'Settings'` (green), `go vet ./internal/tui/`.
- [ ] Validation loop: do not exit until Settings tests pass and the view renders all three boxes plus sub-views without panic on a nil backend.

# Task 4: App wiring (sixth tab + Focused gate)

Files:
- Modify: `internal/tui/tabs.go`
- Modify: `internal/tui/app.go`
- Test: `internal/tui/app_test.go`

Interfaces:
- Consumes: `SettingsTab`, `settingsActionMsg`, `settingsCollectionsMsg`.

Steps:
- [ ] `tabs.go`: add `TabSettings = 5`; append `"Settings"` so `tabNames = []string{"Search", "Browse", "Graph", "Drift", "Status", "Settings"}`.
- [ ] `app.go`: add `settingsTab SettingsTab` to `App`. Extend `buildTabs` (`a.settingsTab = NewSettingsTab(be)`), `applySize`, `Init` (`a.settingsTab.Init()`), `syncFrame` (`a.settingsTab.frame = a.frame`), `activeTabModel` (`case TabSettings: return a.settingsTab`), the `View` content switch (`case TabSettings: content = a.settingsTab.View(a.width, contentHeight)`).
- [ ] `app.go` `Update`: in the number-jump block add `case "6": a.activeTab = TabSettings`. In the typed-message routing add `case settingsActionMsg, settingsCollectionsMsg:` routing to `a.settingsTab.Update` (and add a `case TabSettings:` to the per-tab fallthrough switch).
- [ ] `app.go` `Update`: gate the `left` / `right` handling behind `!a.activeTabModel().Focused()`. Move the `switch msg.String() { case "left": ...; case "right": ... }` inside the existing `if !a.activeTabModel().Focused() {` block (alongside the number jumps), so a focused tab keeps arrow keys.
- [ ] Update `app_test.go`: assert `len(tabNames) == 6`; pressing `"6"` sets `activeTab == TabSettings`; with the active tab `Focused()` (drive Settings into editing), `left`/`right` do not change `activeTab`. Match the existing `app_test.go` construction of the App and key messages.
- [ ] Run `go test ./internal/tui/` (all green), `go vet ./internal/tui/`.
- [ ] Validation loop: do not exit until the full tui package builds and tests pass.

# Verification

- [ ] `go build ./...` clean.
- [ ] `go vet ./...` clean.
- [ ] `go test ./...` green (race if the repo default uses it: `go test -race ./internal/service/ ./internal/tui/`).
- [ ] `grep -rnP "\x{2014}|\x{2013}" internal/tui/settings_tab.go internal/tui/drift_tab.go internal/service/service.go internal/service/registry.go docs/specs/2026-06-27-0230-settings-tab-and-drift-redesign.md docs/plans/2026-06-27-0230-settings-tab-and-drift-redesign.md` returns nothing (no em/en dashes).
- [ ] Manual TUI: `go run ./cmd/stardust` in this vault. Tab to Settings (key `6`): edit `embed_model`, confirm `.stardust/config.toml` changed; run Reindex and Regenerate registry, confirm the result note and that `docs/INDEX.md` updated; open Ignore and Inspect collections sub-views and back out. Tab to Drift (key `4`): confirm one `Drifted Docs` header, one `Stale Docs` header, the referenced moved file and its commit count are visible, columns are not cut off, and the check findings are grouped. Resize narrow and wide; confirm columns shrink and boxes clip without overflow.
- [ ] Adversarial: empty vault (clean check, no collections), many findings (grouping holds), empty `reranker_url` saved (`(disabled)`, search still works), double-trigger an action fast (second rejected by `busy`).

# Self-review gate

- [ ] No placeholders or TBDs left in the code; every method body is implemented.
- [ ] Names and types consistent: `settingsActionMsg`, `SettingsTab`, `TabSettings`, `SetConfig`, `RegenerateRegistry` used identically across tasks.
- [ ] Every spec requirement maps to a task: config view/edit (T3), collections + schema/counts (T3), index actions (T1+T3), Drift single headers + summary + fitted columns + grouped findings (T2), service-only writes (T1), sixth tab + Focused gate (T4).
- [ ] No CLI/MCP/SDK files touched; `internal/cli/registry.go` unchanged.
- [ ] No em/en dashes anywhere; `// --- Section ---` separators and export doc comments present.

# Validation

- [ ] Mirror these tasks into the harness todo tool, one in progress at a time, ticking each box the moment it lands.
- [ ] Each task ends green before the next begins.
</content>
