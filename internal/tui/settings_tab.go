package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/internal/tui/components"
)

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
	rowText   settingsRowKind = iota // enter opens an inline text editor
	rowAction                        // enter runs an action or opens a sub-view
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

// --- Settings Tab ---

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

// NewSettingsTab creates the settings tab, seeding its config from the open
// service (falling back to defaults when the backend is absent).
func NewSettingsTab(be *backend) SettingsTab {
	cfg := config.Default()
	if be != nil && be.svc != nil {
		cfg = be.svc.Config
	}
	edit := components.NewExoTextInput("value")
	edit.Blur()
	ignore := components.NewExoTextInput("ignore pattern")
	ignore.Blur()
	return SettingsTab{be: be, cfg: cfg, editInput: edit, ignoreInput: ignore}
}

func newSettingsTab(be *backend) SettingsTab {
	return NewSettingsTab(be)
}

// Resize stores the latest terminal size.
func (t *SettingsTab) Resize(width, height int) {
	t.width = width
	t.height = height
}

// Init loads the collections for the read-only COLLECTIONS box.
func (t SettingsTab) Init() tea.Cmd { return t.loadCollections() }

// Update routes async results and key presses, deferring to the inline editor or
// the active sub-view when one owns the keyboard.
func (t SettingsTab) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	switch msg := msg.(type) {
	case settingsActionMsg:
		t.busy = false
		if msg.err != nil {
			t.note = ErrorStyle.Render(msg.kind + ": " + msg.err.Error())
		} else {
			t.note = SuccessStyle.Render(msg.summary)
		}
		return t, nil
	case settingsCollectionsMsg:
		if msg.err != nil {
			t.note = ErrorStyle.Render("collections: " + msg.err.Error())
		} else {
			t.collections = msg.collections
		}
		return t, nil
	case tea.KeyPressMsg:
		return t.handleKey(msg)
	}
	return t, nil
}

func (t SettingsTab) handleKey(msg tea.KeyPressMsg) (TabModel, tea.Cmd) {
	if t.editing {
		return t.handleEditKey(msg)
	}
	switch t.sub {
	case settingsIgnore:
		return t.handleIgnoreKey(msg)
	case settingsCollections:
		return t.handleCollectionsKey(msg)
	case settingsSchema:
		return t.handleSchemaKey(msg)
	default:
		return t.handleListKey(msg)
	}
}

func (t SettingsTab) handleListKey(msg tea.KeyPressMsg) (TabModel, tea.Cmd) {
	switch msg.String() {
	case "up":
		if t.cursor > 0 {
			t.cursor--
		}
	case "down":
		if t.cursor < len(settingsRows)-1 {
			t.cursor++
		}
	case "enter":
		return t.activateRow()
	case "r":
		return t, t.loadCollections()
	}
	return t, nil
}

func (t SettingsTab) activateRow() (TabModel, tea.Cmd) {
	row := settingsRows[t.cursor]
	switch row.kind {
	case rowText:
		t.editing = true
		t.editKey = row.key
		t.editInput = components.NewExoTextInput(row.label)
		t.editInput.SetValue(settingsValueRaw(row.key, t.cfg))
		return t, t.editInput.Focus()
	case rowAction:
		switch row.key {
		case "ignore":
			t.sub = settingsIgnore
			t.ignoreCursor = 0
			return t, nil
		case "collections":
			t.sub = settingsCollections
			t.colCursor = 0
			return t, t.loadCollections()
		case "reindex":
			if t.busy {
				t.note = MutedStyle.Render("busy")
				return t, nil
			}
			t.busy = true
			t.note = MutedStyle.Render("reindexing...")
			return t, t.reindexCmd()
		case "rebuild":
			if t.busy {
				t.note = MutedStyle.Render("busy")
				return t, nil
			}
			t.busy = true
			t.note = MutedStyle.Render("rebuilding...")
			return t, t.rebuildCmd()
		case "registry":
			if t.busy {
				t.note = MutedStyle.Render("busy")
				return t, nil
			}
			t.busy = true
			t.note = MutedStyle.Render("regenerating registry...")
			return t, t.registryCmd()
		}
	}
	return t, nil
}

func (t SettingsTab) handleEditKey(msg tea.KeyPressMsg) (TabModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		t.editing = false
		t.editInput.Blur()
		return t, nil
	case "enter":
		next := t.cfg
		setField(&next, t.editKey, strings.TrimSpace(t.editInput.Value()))
		if err := t.saveConfig(next); err != nil {
			t.note = ErrorStyle.Render("save failed: " + err.Error())
		} else {
			t.cfg = next
			t.note = SuccessStyle.Render("saved " + t.editKey)
		}
		t.editing = false
		t.editInput.Blur()
		return t, nil
	}
	var cmd tea.Cmd
	t.editInput, cmd = t.editInput.Update(msg)
	return t, cmd
}

func (t SettingsTab) handleIgnoreKey(msg tea.KeyPressMsg) (TabModel, tea.Cmd) {
	if t.addingIgnore {
		switch msg.String() {
		case "esc":
			t.addingIgnore = false
			t.ignoreInput.Blur()
			return t, nil
		case "enter":
			val := strings.TrimSpace(t.ignoreInput.Value())
			if val != "" {
				next := t.cfg
				next.Ignore = append(append([]string{}, t.cfg.Ignore...), val)
				if err := t.saveConfig(next); err != nil {
					t.note = ErrorStyle.Render("save failed: " + err.Error())
				} else {
					t.cfg = next
					t.note = SuccessStyle.Render("added " + val)
				}
			}
			t.addingIgnore = false
			t.ignoreInput.Blur()
			return t, nil
		}
		var cmd tea.Cmd
		t.ignoreInput, cmd = t.ignoreInput.Update(msg)
		return t, cmd
	}
	switch msg.String() {
	case "esc":
		t.sub = settingsNone
	case "up":
		if t.ignoreCursor > 0 {
			t.ignoreCursor--
		}
	case "down":
		if t.ignoreCursor < len(t.cfg.Ignore)-1 {
			t.ignoreCursor++
		}
	case "n":
		t.addingIgnore = true
		t.ignoreInput = components.NewExoTextInput("ignore pattern")
		t.ignoreInput.SetValue("")
		return t, t.ignoreInput.Focus()
	case "d":
		if len(t.cfg.Ignore) > 0 && t.ignoreCursor >= 0 && t.ignoreCursor < len(t.cfg.Ignore) {
			removed := t.cfg.Ignore[t.ignoreCursor]
			next := t.cfg
			next.Ignore = removeAt(t.cfg.Ignore, t.ignoreCursor)
			if err := t.saveConfig(next); err != nil {
				t.note = ErrorStyle.Render("save failed: " + err.Error())
			} else {
				t.cfg = next
				if t.ignoreCursor >= len(t.cfg.Ignore) && t.ignoreCursor > 0 {
					t.ignoreCursor--
				}
				t.note = SuccessStyle.Render("removed " + removed)
			}
		}
	}
	return t, nil
}

func (t SettingsTab) handleCollectionsKey(msg tea.KeyPressMsg) (TabModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		t.sub = settingsNone
	case "up":
		if t.colCursor > 0 {
			t.colCursor--
		}
	case "down":
		if t.colCursor < len(t.collections)-1 {
			t.colCursor++
		}
	case "enter":
		if t.colCursor >= 0 && t.colCursor < len(t.collections) {
			t.schemaName = t.collections[t.colCursor].Name
			t.sub = settingsSchema
		}
	case "r":
		return t, t.loadCollections()
	}
	return t, nil
}

func (t SettingsTab) handleSchemaKey(msg tea.KeyPressMsg) (TabModel, tea.Cmd) {
	if msg.String() == "esc" {
		t.sub = settingsCollections
	}
	return t, nil
}

// --- Async commands ---

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

func (t SettingsTab) rebuildCmd() tea.Cmd {
	be := t.be
	return func() tea.Msg {
		if be == nil || be.svc == nil {
			return settingsActionMsg{kind: "rebuild", err: fmt.Errorf("settings: service is not open")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		stats, err := be.svc.Rebuild(ctx)
		if err != nil {
			return settingsActionMsg{kind: "rebuild", err: err}
		}
		return settingsActionMsg{kind: "rebuild", summary: fmt.Sprintf("rebuilt: indexed %d, deleted %d, vectors %s",
			stats.Indexed, stats.Deleted, boolWord(stats.Vectors))}
	}
}

func (t SettingsTab) registryCmd() tea.Cmd {
	be := t.be
	return func() tea.Msg {
		if be == nil || be.svc == nil {
			return settingsActionMsg{kind: "registry", err: fmt.Errorf("settings: service is not open")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := be.svc.RegenerateRegistry(ctx); err != nil {
			return settingsActionMsg{kind: "registry", err: err}
		}
		return settingsActionMsg{kind: "registry", summary: "regenerated docs/INDEX.md"}
	}
}

func (t SettingsTab) loadCollections() tea.Cmd {
	be := t.be
	return func() tea.Msg {
		if be == nil || be.svc == nil {
			return settingsCollectionsMsg{err: fmt.Errorf("settings: service is not open")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cols, err := be.svc.ListCollections(ctx)
		return settingsCollectionsMsg{collections: cols, err: err}
	}
}

// saveConfig persists cfg through the service so the live embed and rerank
// clients are rebuilt; a nil backend yields a service-not-open error.
func (t SettingsTab) saveConfig(cfg config.Config) error {
	if t.be == nil || t.be.svc == nil {
		return fmt.Errorf("settings: service is not open")
	}
	return t.be.svc.SetConfig(cfg)
}

// --- View ---

// View renders the config and collections boxes as a tidy matched-width full-
// width stack, or the active sub-view. CONFIG is the only box with a wrapper
// title because its content is plain rows; every clean-list box (COLLECTIONS and
// the sub-views) carries its own keycap header, so those wrappers pass an empty
// title and exactly one header shows per section.
func (t SettingsTab) View(width, height int) string {
	if width <= 0 {
		width = t.width
	}
	if height <= 0 {
		height = t.height
	}
	cardW := tableWidth(width)
	switch t.sub {
	case settingsIgnore:
		return centerBlockUniform(animatedDoubleBox("", t.ignoreView(cardW, height-4), t.frame), width)
	case settingsCollections:
		return centerBlockUniform(animatedDoubleBox("", t.collectionsView(cardW, height-4), t.frame), width)
	case settingsSchema:
		return centerBlockUniform(animatedDoubleBox("", t.schemaView(cardW, height-4), t.frame), width)
	}

	configBox := animatedDoubleBox("CONFIG", t.configRows(cardW), t.frame)
	avail := height - 6
	if avail < 12 {
		avail = 12
	}
	colH := avail - countViewLines(configBox)
	if colH < 6 {
		colH = 6
	}
	var b strings.Builder
	b.WriteString(configBox)
	b.WriteString("\n\n")
	b.WriteString(animatedRoundedBox("", t.collectionsList(cardW, colH), t.frame))
	return centerBlockUniform(b.String(), width)
}

// configRows renders the editable config rows, padding each line to width so the
// CONFIG box matches the COLLECTIONS box width for a tidy aligned stack.
func (t SettingsTab) configRows(width int) string {
	var b strings.Builder
	for i, row := range settingsRows {
		cursor := MutedStyle.Render("  ")
		label := MutedStyle.Render(padRightCell(row.label, 20))
		if i == t.cursor {
			cursor = AccentStyle.Bold(true).Render("> ")
			label = AccentStyle.Bold(true).Render(padRightCell(row.label, 20))
		}
		value := t.rowValue(row)
		if t.editing && t.editKey == row.key {
			value = t.editInput.View()
		}
		b.WriteString(padRightCell(cursor+label+value, width))
		if i < len(settingsRows)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (t SettingsTab) rowValue(row settingsRow) string {
	switch row.kind {
	case rowText:
		raw := strings.TrimSpace(settingsValueRaw(row.key, t.cfg))
		if raw == "" {
			if row.key == "reranker_url" {
				return MutedStyle.Render("(disabled)")
			}
			return MutedStyle.Render("(unset)")
		}
		return SuccessStyle.Render(components.SanitizeOneLine(raw))
	case rowAction:
		switch row.key {
		case "ignore":
			return MutedStyle.Render(fmt.Sprintf("manage (%d)", len(t.cfg.Ignore)))
		case "collections":
			return MutedStyle.Render("inspect")
		default:
			if t.busy {
				return MutedStyle.Render("run (busy)")
			}
			return MutedStyle.Render("run")
		}
	}
	return ""
}

func (t SettingsTab) collectionsList(width, height int) string {
	if len(t.collections) == 0 {
		return clipLines(renderCleanListHeader("Collections", "", width)+"\n\n"+MutedStyle.Render("no collections configured"), height)
	}
	return clipLines(t.renderCollections(width, -1), height)
}

func (t SettingsTab) collectionsView(width, height int) string {
	if len(t.collections) == 0 {
		return clipLines(renderCleanListHeader("Collections", "", width)+"\n\n"+MutedStyle.Render("no collections configured"), height)
	}
	return clipLines(t.renderCollections(width, t.colCursor), height)
}

func (t SettingsTab) renderCollections(width, active int) string {
	rows := make([]cleanListRow, 0, len(t.collections))
	for _, c := range t.collections {
		rows = append(rows, cleanListRow{Cells: []string{
			components.SanitizeOneLine(c.Name),
			fmt.Sprintf("%d", c.Records),
			fmt.Sprintf("%d", len(c.Fields)),
			components.SanitizeOneLine(c.Path),
			components.SanitizeOneLine(c.Description),
		}})
	}
	cols := []cleanListColumn{
		{Header: "Collection", MinWidth: 12, MaxWidth: 24, Primary: true},
		{Header: "Records", MinWidth: 7, MaxWidth: 8, Align: lipgloss.Right, Count: true},
		{Header: "Fields", MinWidth: 6, MaxWidth: 7, Align: lipgloss.Right, Count: true},
		{Header: "Path", MinWidth: 14, MaxWidth: 36, Muted: true, Underline: true},
		{Header: "Description", MinWidth: 16, MaxWidth: 60, Muted: true},
	}
	return renderCleanList("Collections", cleanListCountLabel(len(t.collections), "collection"), cols, rows, width, active)
}

func (t SettingsTab) ignoreView(width, height int) string {
	var b strings.Builder
	if t.addingIgnore {
		b.WriteString(t.ignoreInput.View())
		b.WriteString("\n\n")
	}
	if len(t.cfg.Ignore) == 0 {
		b.WriteString(renderCleanListHeader("Ignore", "", width))
		b.WriteString("\n\n")
		b.WriteString(MutedStyle.Render("no ignore patterns"))
		return clipLines(b.String(), height)
	}
	rows := make([]cleanListRow, 0, len(t.cfg.Ignore))
	for _, pat := range t.cfg.Ignore {
		rows = append(rows, cleanListRow{Cells: []string{components.SanitizeOneLine(pat)}})
	}
	cols := []cleanListColumn{{Header: "Pattern", MinWidth: 12, Primary: true}}
	active := t.ignoreCursor
	if t.addingIgnore {
		active = -1
	}
	b.WriteString(renderCleanList("Ignore", cleanListCountLabel(len(t.cfg.Ignore), "pattern"), cols, rows, width, active))
	return clipLines(b.String(), height)
}

func (t SettingsTab) schemaView(width, height int) string {
	idx := -1
	for i, c := range t.collections {
		if c.Name == t.schemaName {
			idx = i
			break
		}
	}
	if idx < 0 || len(t.collections[idx].Fields) == 0 {
		return renderCleanListHeader(t.schemaName+" schema", "", width) + "\n\n" +
			MutedStyle.Render("no fields in "+components.SanitizeOneLine(t.schemaName))
	}
	fields := t.collections[idx].Fields
	rows := make([]cleanListRow, 0, len(fields))
	for _, f := range fields {
		rows = append(rows, cleanListRow{Cells: []string{
			components.SanitizeOneLine(f.Name),
			components.SanitizeOneLine(f.Type),
			boolWord(f.Required),
			components.SanitizeOneLine(strings.Join(f.Enum, ", ")),
		}})
	}
	cols := []cleanListColumn{
		{Header: "Field", MinWidth: 10, MaxWidth: 24, Primary: true},
		{Header: "Type", MinWidth: 6, MaxWidth: 12, Muted: true},
		{Header: "Required", MinWidth: 8, MaxWidth: 9},
		{Header: "Enum", MinWidth: 10, Muted: true},
	}
	label := cleanListCountLabel(len(fields), "field")
	return clipLines(renderCleanList(t.schemaName+" schema", label, cols, rows, width, -1), height)
}

// Hints returns mode-aware key hints for the settings tab.
func (t SettingsTab) Hints() []components.HintItem {
	if t.editing {
		return []components.HintItem{
			{Key: "enter", Desc: "save"},
			{Key: "esc", Desc: "cancel"},
		}
	}
	switch t.sub {
	case settingsIgnore:
		if t.addingIgnore {
			return []components.HintItem{
				{Key: "enter", Desc: "add"},
				{Key: "esc", Desc: "cancel"},
			}
		}
		return []components.HintItem{
			{Key: "up/down", Desc: "select"},
			{Key: "n", Desc: "add"},
			{Key: "d", Desc: "delete"},
			{Key: "esc", Desc: "back"},
		}
	case settingsCollections:
		return []components.HintItem{
			{Key: "up/down", Desc: "select"},
			{Key: "enter", Desc: "schema"},
			{Key: "esc", Desc: "back"},
		}
	case settingsSchema:
		return []components.HintItem{{Key: "esc", Desc: "back"}}
	}
	return withCommonHints(
		components.HintItem{Key: "up/down", Desc: "select"},
		components.HintItem{Key: "enter", Desc: "edit/run"},
		components.HintItem{Key: "r", Desc: "refresh"},
	)
}

// Focused reports whether the settings tab owns keyboard text, true while the
// inline editor or any sub-view is active, so the app stops switching tabs.
func (t SettingsTab) Focused() bool { return t.editing || t.sub != settingsNone }

// StatusLine returns the settings tab status text.
func (t SettingsTab) StatusLine() string {
	if t.note != "" {
		return t.note
	}
	return MutedStyle.Render("config + index actions")
}

// HeaderLabel returns the shared animated header label.
func (t SettingsTab) HeaderLabel() string { return "settings · config + index" }

// --- Helpers ---

// settingsValueRaw returns the plain config string backing an editable row.
func settingsValueRaw(key string, cfg config.Config) string {
	switch key {
	case "embed_model":
		return cfg.EmbedModel
	case "ollama_url":
		return cfg.OllamaURL
	case "reranker_url":
		return cfg.RerankerURL
	case "reranker_model":
		return cfg.RerankerModel
	}
	return ""
}

// setField writes the trimmed value onto the matching config field.
func setField(cfg *config.Config, key, val string) {
	switch key {
	case "embed_model":
		cfg.EmbedModel = val
	case "ollama_url":
		cfg.OllamaURL = val
	case "reranker_url":
		cfg.RerankerURL = val
	case "reranker_model":
		cfg.RerankerModel = val
	}
}

// removeAt returns a copy of s with the element at i removed; an out-of-range
// index returns a copy unchanged.
func removeAt(s []string, i int) []string {
	if i < 0 || i >= len(s) {
		return append([]string{}, s...)
	}
	out := make([]string, 0, len(s)-1)
	out = append(out, s[:i]...)
	out = append(out, s[i+1:]...)
	return out
}

// padRightCell pads s with trailing spaces to width display columns, leaving it
// unchanged when already at least that wide.
func padRightCell(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}
