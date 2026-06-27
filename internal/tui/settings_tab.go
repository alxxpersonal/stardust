package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/alxxpersonal/stardust/internal/collections"
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

// settingsCollectionMutationMsg carries a saved or deleted collection plus a
// fresh collection list.
type settingsCollectionMutationMsg struct {
	summary     string
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

type collectionFormMode int

const (
	collectionFormNone collectionFormMode = iota
	collectionFormAdd
	collectionFormEdit
)

type collectionFormStep int

const (
	collectionStepName collectionFormStep = iota
	collectionStepPath
	collectionStepDescription
)

type collectionFormState struct {
	mode  collectionFormMode
	step  collectionFormStep
	info  service.CollectionInfo
	input textinput.Model
}

type fieldFormMode int

const (
	fieldFormNone fieldFormMode = iota
	fieldFormAdd
	fieldFormEdit
)

type fieldFormStep int

const (
	fieldStepName fieldFormStep = iota
	fieldStepType
	fieldStepRequired
	fieldStepEnum
)

type fieldFormState struct {
	mode      fieldFormMode
	step      fieldFormStep
	editIndex int
	field     collections.Field
	input     textinput.Model
}

var schemaFieldTypes = []string{
	collections.TypeString,
	collections.TypeEnum,
	collections.TypeDate,
	collections.TypeNumber,
	collections.TypeTags,
	collections.TypeRef,
	collections.TypeBool,
}

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

	sub          settingsSub
	collections  []service.CollectionInfo
	colCursor    int
	schemaName   string
	schemaCursor int

	collectionForm             collectionFormState
	confirmingCollectionDelete bool
	fieldForm                  fieldFormState
	confirmingFieldDelete      bool

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
	collectionInput := components.NewExoTextInput("collection")
	collectionInput.Blur()
	fieldInput := components.NewExoTextInput("field")
	fieldInput.Blur()
	return SettingsTab{
		be:          be,
		cfg:         cfg,
		editInput:   edit,
		ignoreInput: ignore,
		collectionForm: collectionFormState{
			input: collectionInput,
		},
		fieldForm: fieldFormState{
			editIndex: -1,
			input:     fieldInput,
		},
	}
}

func newSettingsTab(be *backend) SettingsTab {
	return NewSettingsTab(be)
}

// Resize stores the latest terminal size.
func (t *SettingsTab) Resize(width, height int) {
	t.width = width
	t.height = height
}

// Init loads the collections for the settings overview.
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
			t.clampCollectionCursors()
		}
		return t, nil
	case settingsCollectionMutationMsg:
		t.busy = false
		if msg.err != nil {
			t.note = ErrorStyle.Render("collections: " + msg.err.Error())
		} else {
			t.collections = msg.collections
			t.clampCollectionCursors()
			t.note = SuccessStyle.Render(msg.summary)
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
	if t.collectionForm.mode != collectionFormNone {
		return t.handleCollectionFormKey(msg)
	}
	if t.confirmingCollectionDelete && msg.String() != "d" {
		t.confirmingCollectionDelete = false
	}
	switch msg.String() {
	case "esc":
		t.sub = settingsNone
		t.confirmingCollectionDelete = false
	case "up":
		if t.colCursor > 0 {
			t.colCursor--
		}
	case "down":
		if t.colCursor < len(t.collections)-1 {
			t.colCursor++
		}
	case "enter":
		return t.startCollectionEdit()
	case "n":
		return t.startCollectionAdd()
	case "s":
		return t.openSelectedSchema()
	case "d":
		return t.deleteSelectedCollection()
	case "r":
		return t, t.loadCollections()
	}
	return t, nil
}

func (t SettingsTab) handleSchemaKey(msg tea.KeyPressMsg) (TabModel, tea.Cmd) {
	if t.fieldForm.mode != fieldFormNone {
		return t.handleFieldFormKey(msg)
	}
	if t.confirmingFieldDelete && msg.String() != "d" {
		t.confirmingFieldDelete = false
	}
	switch msg.String() {
	case "esc":
		t.sub = settingsCollections
		t.confirmingFieldDelete = false
	case "up":
		if t.schemaCursor > 0 {
			t.schemaCursor--
		}
	case "down":
		if c, ok := t.schemaCollection(); ok && t.schemaCursor < len(c.Fields)-1 {
			t.schemaCursor++
		}
	case "n":
		return t.startFieldAdd()
	case "enter":
		return t.startFieldEdit()
	case "d":
		return t.deleteSelectedField()
	case "r":
		return t, t.loadCollections()
	}
	return t, nil
}

func (t SettingsTab) startCollectionAdd() (TabModel, tea.Cmd) {
	if t.busy {
		t.note = MutedStyle.Render("busy")
		return t, nil
	}
	t.confirmingCollectionDelete = false
	t.collectionForm = collectionFormState{
		mode:  collectionFormAdd,
		step:  collectionStepName,
		info:  service.CollectionInfo{},
		input: components.NewExoTextInput("collection name"),
	}
	return t, t.collectionForm.input.Focus()
}

func (t SettingsTab) startCollectionEdit() (TabModel, tea.Cmd) {
	if t.busy {
		t.note = MutedStyle.Render("busy")
		return t, nil
	}
	c, ok := t.selectedCollection()
	if !ok {
		t.note = MutedStyle.Render("no collection selected")
		return t, nil
	}
	if !collectionIsEditable(c) {
		t.note = MutedStyle.Render("reference collection is not editable")
		return t, nil
	}
	t.confirmingCollectionDelete = false
	t.collectionForm = collectionFormState{
		mode:  collectionFormEdit,
		step:  collectionStepPath,
		info:  c,
		input: components.NewExoTextInput("collection path"),
	}
	t.collectionForm.input.SetValue(c.Path)
	return t, t.collectionForm.input.Focus()
}

func (t SettingsTab) handleCollectionFormKey(msg tea.KeyPressMsg) (TabModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		t.collectionForm = collectionFormState{}
		return t, nil
	case "enter":
		return t.advanceCollectionForm()
	}
	var cmd tea.Cmd
	t.collectionForm.input, cmd = t.collectionForm.input.Update(msg)
	return t, cmd
}

func (t SettingsTab) advanceCollectionForm() (TabModel, tea.Cmd) {
	val := strings.TrimSpace(t.collectionForm.input.Value())
	switch t.collectionForm.step {
	case collectionStepName:
		if val == "" {
			t.note = ErrorStyle.Render("collection name is required")
			return t, nil
		}
		t.collectionForm.info.Name = val
		t.collectionForm.step = collectionStepPath
		t.collectionForm.input = components.NewExoTextInput("collection path")
		t.collectionForm.input.SetValue(t.collectionForm.info.Path)
		return t, t.collectionForm.input.Focus()
	case collectionStepPath:
		if val == "" {
			t.note = ErrorStyle.Render("collection path is required")
			return t, nil
		}
		t.collectionForm.info.Path = val
		t.collectionForm.step = collectionStepDescription
		t.collectionForm.input = components.NewExoTextInput("description")
		t.collectionForm.input.SetValue(t.collectionForm.info.Description)
		return t, t.collectionForm.input.Focus()
	case collectionStepDescription:
		t.collectionForm.info.Description = val
		info := t.collectionForm.info
		summary := "saved collection " + info.Name
		if t.collectionForm.mode == collectionFormAdd {
			summary = "added collection " + info.Name
		}
		t.collectionForm = collectionFormState{}
		t.busy = true
		t.note = MutedStyle.Render("saving collection...")
		return t, t.saveCollectionCmd(info, summary)
	}
	return t, nil
}

func (t SettingsTab) openSelectedSchema() (TabModel, tea.Cmd) {
	c, ok := t.selectedCollection()
	if !ok {
		t.note = MutedStyle.Render("no collection selected")
		return t, nil
	}
	if !collectionIsEditable(c) {
		t.note = MutedStyle.Render("reference collection is not editable")
		return t, nil
	}
	t.schemaName = c.Name
	t.schemaCursor = 0
	t.sub = settingsSchema
	t.confirmingCollectionDelete = false
	return t, nil
}

func (t SettingsTab) deleteSelectedCollection() (TabModel, tea.Cmd) {
	if t.busy {
		t.note = MutedStyle.Render("busy")
		return t, nil
	}
	c, ok := t.selectedCollection()
	if !ok {
		t.note = MutedStyle.Render("no collection selected")
		return t, nil
	}
	if !collectionIsEditable(c) {
		t.note = MutedStyle.Render("reference collection is not editable")
		return t, nil
	}
	if !t.confirmingCollectionDelete {
		t.confirmingCollectionDelete = true
		t.note = MutedStyle.Render("press d again to unregister " + c.Name + "; docs stay on disk")
		return t, nil
	}
	t.confirmingCollectionDelete = false
	t.busy = true
	t.note = MutedStyle.Render("deleting collection...")
	return t, t.deleteCollectionCmd(c.Name, "deleted collection "+c.Name+"; docs left on disk")
}

func (t SettingsTab) startFieldAdd() (TabModel, tea.Cmd) {
	if t.busy {
		t.note = MutedStyle.Render("busy")
		return t, nil
	}
	if _, ok := t.schemaCollection(); !ok {
		t.note = MutedStyle.Render("collection not found")
		return t, nil
	}
	t.confirmingFieldDelete = false
	t.fieldForm = fieldFormState{
		mode:      fieldFormAdd,
		step:      fieldStepName,
		editIndex: -1,
		field:     collections.Field{Type: collections.TypeString},
		input:     components.NewExoTextInput("field name"),
	}
	return t, t.fieldForm.input.Focus()
}

func (t SettingsTab) startFieldEdit() (TabModel, tea.Cmd) {
	if t.busy {
		t.note = MutedStyle.Render("busy")
		return t, nil
	}
	c, ok := t.schemaCollection()
	if !ok || len(c.Fields) == 0 {
		t.note = MutedStyle.Render("no field selected")
		return t, nil
	}
	if t.schemaCursor < 0 || t.schemaCursor >= len(c.Fields) {
		t.note = MutedStyle.Render("no field selected")
		return t, nil
	}
	field := c.Fields[t.schemaCursor]
	t.confirmingFieldDelete = false
	t.fieldForm = fieldFormState{
		mode:      fieldFormEdit,
		step:      fieldStepName,
		editIndex: t.schemaCursor,
		field:     field,
		input:     components.NewExoTextInput("field name"),
	}
	t.fieldForm.input.SetValue(field.Name)
	return t, t.fieldForm.input.Focus()
}

func (t SettingsTab) handleFieldFormKey(msg tea.KeyPressMsg) (TabModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		t.fieldForm = fieldFormState{editIndex: -1}
		return t, nil
	case "enter":
		return t.advanceFieldForm()
	}
	var cmd tea.Cmd
	t.fieldForm.input, cmd = t.fieldForm.input.Update(msg)
	return t, cmd
}

func (t SettingsTab) advanceFieldForm() (TabModel, tea.Cmd) {
	val := strings.TrimSpace(t.fieldForm.input.Value())
	switch t.fieldForm.step {
	case fieldStepName:
		if val == "" {
			t.note = ErrorStyle.Render("field name is required")
			return t, nil
		}
		t.fieldForm.field.Name = val
		t.fieldForm.step = fieldStepType
		t.fieldForm.input = components.NewExoTextInput("field type")
		t.fieldForm.input.SetValue(t.fieldForm.field.Type)
		return t, t.fieldForm.input.Focus()
	case fieldStepType:
		typ := strings.ToLower(val)
		if !validSchemaFieldType(typ) {
			t.note = ErrorStyle.Render("field type must be " + strings.Join(schemaFieldTypes, ", "))
			return t, nil
		}
		t.fieldForm.field.Type = typ
		t.fieldForm.step = fieldStepRequired
		t.fieldForm.input = components.NewExoTextInput("required true/false")
		t.fieldForm.input.SetValue(boolWord(t.fieldForm.field.Required))
		return t, t.fieldForm.input.Focus()
	case fieldStepRequired:
		required, ok := parseBoolInput(val)
		if !ok {
			t.note = ErrorStyle.Render("required must be true or false")
			return t, nil
		}
		t.fieldForm.field.Required = required
		if t.fieldForm.field.Type != collections.TypeEnum {
			t.fieldForm.field.Enum = nil
			return t.saveFieldForm()
		}
		t.fieldForm.step = fieldStepEnum
		t.fieldForm.input = components.NewExoTextInput("enum values")
		t.fieldForm.input.SetValue(strings.Join(t.fieldForm.field.Enum, ", "))
		return t, t.fieldForm.input.Focus()
	case fieldStepEnum:
		t.fieldForm.field.Enum = splitCSV(val)
		return t.saveFieldForm()
	}
	return t, nil
}

func (t SettingsTab) saveFieldForm() (TabModel, tea.Cmd) {
	c, ok := t.schemaCollection()
	if !ok {
		t.note = ErrorStyle.Render("collection not found")
		return t, nil
	}
	info := c
	fields := append([]collections.Field{}, c.Fields...)
	switch t.fieldForm.mode {
	case fieldFormAdd:
		fields = append(fields, t.fieldForm.field)
	case fieldFormEdit:
		if t.fieldForm.editIndex < 0 || t.fieldForm.editIndex >= len(fields) {
			t.note = ErrorStyle.Render("field not found")
			return t, nil
		}
		fields[t.fieldForm.editIndex] = t.fieldForm.field
	}
	info.Fields = fields
	name := t.fieldForm.field.Name
	summary := "saved field " + name
	if t.fieldForm.mode == fieldFormAdd {
		summary = "added field " + name
	}
	t.fieldForm = fieldFormState{editIndex: -1}
	t.busy = true
	t.note = MutedStyle.Render("saving schema...")
	return t, t.saveCollectionCmd(info, summary)
}

func (t SettingsTab) deleteSelectedField() (TabModel, tea.Cmd) {
	if t.busy {
		t.note = MutedStyle.Render("busy")
		return t, nil
	}
	c, ok := t.schemaCollection()
	if !ok || len(c.Fields) == 0 {
		t.note = MutedStyle.Render("no field selected")
		return t, nil
	}
	if t.schemaCursor < 0 || t.schemaCursor >= len(c.Fields) {
		t.note = MutedStyle.Render("no field selected")
		return t, nil
	}
	field := c.Fields[t.schemaCursor]
	if !t.confirmingFieldDelete {
		t.confirmingFieldDelete = true
		t.note = MutedStyle.Render("press d again to delete field " + field.Name)
		return t, nil
	}
	info := c
	info.Fields = removeFieldAt(c.Fields, t.schemaCursor)
	t.confirmingFieldDelete = false
	t.busy = true
	t.note = MutedStyle.Render("saving schema...")
	return t, t.saveCollectionCmd(info, "deleted field "+field.Name)
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

func (t SettingsTab) saveCollectionCmd(info service.CollectionInfo, summary string) tea.Cmd {
	be := t.be
	return func() tea.Msg {
		if be == nil || be.svc == nil {
			return settingsCollectionMutationMsg{err: fmt.Errorf("settings: service is not open")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := be.svc.SaveCollection(ctx, info); err != nil {
			return settingsCollectionMutationMsg{err: err}
		}
		cols, err := be.svc.ListCollections(ctx)
		return settingsCollectionMutationMsg{summary: summary, collections: cols, err: err}
	}
}

func (t SettingsTab) deleteCollectionCmd(name, summary string) tea.Cmd {
	be := t.be
	return func() tea.Msg {
		if be == nil || be.svc == nil {
			return settingsCollectionMutationMsg{err: fmt.Errorf("settings: service is not open")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := be.svc.DeleteCollection(ctx, name); err != nil {
			return settingsCollectionMutationMsg{err: err}
		}
		cols, err := be.svc.ListCollections(ctx)
		return settingsCollectionMutationMsg{summary: summary, collections: cols, err: err}
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
	var b strings.Builder
	if t.collectionForm.mode != collectionFormNone {
		b.WriteString(t.collectionFormLine())
		b.WriteString("\n\n")
	}
	if len(t.collections) == 0 {
		b.WriteString(renderCleanListHeader("Collections", "", width))
		b.WriteString("\n\n")
		b.WriteString(MutedStyle.Render("no collections configured"))
		return clipLines(b.String(), height)
	}
	active := t.colCursor
	if t.collectionForm.mode != collectionFormNone {
		active = -1
	}
	b.WriteString(t.renderCollections(width, active))
	return clipLines(b.String(), height)
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
	var b strings.Builder
	if t.fieldForm.mode != fieldFormNone {
		b.WriteString(t.fieldFormLine())
		b.WriteString("\n\n")
	}
	c, ok := t.schemaCollection()
	if !ok || len(c.Fields) == 0 {
		b.WriteString(renderCleanListHeader(t.schemaName+" schema", "", width))
		b.WriteString("\n\n")
		b.WriteString(MutedStyle.Render("no fields in " + components.SanitizeOneLine(t.schemaName)))
		return clipLines(b.String(), height)
	}
	fields := c.Fields
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
	active := t.schemaCursor
	if t.fieldForm.mode != fieldFormNone {
		active = -1
	}
	b.WriteString(renderCleanList(t.schemaName+" schema", label, cols, rows, width, active))
	return clipLines(b.String(), height)
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
		if t.collectionForm.mode != collectionFormNone {
			return []components.HintItem{
				{Key: "enter", Desc: "next/save"},
				{Key: "esc", Desc: "cancel"},
			}
		}
		return []components.HintItem{
			{Key: "up/down", Desc: "select"},
			{Key: "n", Desc: "add"},
			{Key: "enter", Desc: "edit"},
			{Key: "s", Desc: "schema"},
			{Key: "d", Desc: "delete"},
			{Key: "r", Desc: "refresh"},
			{Key: "esc", Desc: "back"},
		}
	case settingsSchema:
		if t.fieldForm.mode != fieldFormNone {
			return []components.HintItem{
				{Key: "enter", Desc: "next/save"},
				{Key: "esc", Desc: "cancel"},
			}
		}
		return []components.HintItem{
			{Key: "up/down", Desc: "select"},
			{Key: "n", Desc: "add"},
			{Key: "enter", Desc: "edit"},
			{Key: "d", Desc: "delete"},
			{Key: "r", Desc: "refresh"},
			{Key: "esc", Desc: "back"},
		}
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

func (t SettingsTab) collectionFormLine() string {
	return AccentStyle.Render(collectionStepLabel(t.collectionForm.step)+": ") + t.collectionForm.input.View()
}

func collectionStepLabel(step collectionFormStep) string {
	switch step {
	case collectionStepName:
		return "name"
	case collectionStepPath:
		return "path"
	case collectionStepDescription:
		return "description"
	default:
		return "collection"
	}
}

func (t SettingsTab) fieldFormLine() string {
	return AccentStyle.Render(fieldStepLabel(t.fieldForm.step)+": ") + t.fieldForm.input.View()
}

func fieldStepLabel(step fieldFormStep) string {
	switch step {
	case fieldStepName:
		return "name"
	case fieldStepType:
		return "type"
	case fieldStepRequired:
		return "required"
	case fieldStepEnum:
		return "enum"
	default:
		return "field"
	}
}

func (t SettingsTab) selectedCollection() (service.CollectionInfo, bool) {
	if t.colCursor < 0 || t.colCursor >= len(t.collections) {
		return service.CollectionInfo{}, false
	}
	return t.collections[t.colCursor], true
}

func (t SettingsTab) schemaCollection() (service.CollectionInfo, bool) {
	for _, c := range t.collections {
		if c.Name == t.schemaName {
			return c, true
		}
	}
	return service.CollectionInfo{}, false
}

func collectionIsEditable(c service.CollectionInfo) bool {
	return strings.TrimSpace(c.Name) != "" && !strings.EqualFold(c.Name, "reference")
}

func (t *SettingsTab) clampCollectionCursors() {
	if t.colCursor >= len(t.collections) {
		t.colCursor = len(t.collections) - 1
	}
	if t.colCursor < 0 {
		t.colCursor = 0
	}
	if c, ok := t.schemaCollection(); ok {
		if t.schemaCursor >= len(c.Fields) {
			t.schemaCursor = len(c.Fields) - 1
		}
		if t.schemaCursor < 0 {
			t.schemaCursor = 0
		}
		return
	}
	if t.sub == settingsSchema {
		t.sub = settingsCollections
	}
	t.schemaCursor = 0
}

func validSchemaFieldType(typ string) bool {
	for _, allowed := range schemaFieldTypes {
		if typ == allowed {
			return true
		}
	}
	return false
}

func parseBoolInput(s string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "t", "yes", "y", "1":
		return true, true
	case "false", "f", "no", "n", "0", "":
		return false, true
	default:
		return false, false
	}
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		val := strings.TrimSpace(part)
		if val != "" {
			out = append(out, val)
		}
	}
	return out
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

// removeFieldAt returns a copy of s with the field at i removed; an
// out-of-range index returns a copy unchanged.
func removeFieldAt(s []collections.Field, i int) []collections.Field {
	if i < 0 || i >= len(s) {
		return append([]collections.Field{}, s...)
	}
	out := make([]collections.Field, 0, len(s)-1)
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
