package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/alxxpersonal/stardust/internal/render"
	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/internal/tui/components"
)

// --- Browse Messages ---

type browseLevel int

const (
	levelCollections browseLevel = iota
	levelRecords
	levelRecord
)

type collectionsLoadedMsg struct {
	collections []service.CollectionInfo
	err         error
}

type recordsLoadedMsg struct {
	collection string
	records    service.RecordList
	err        error
}

type recordLoadedMsg struct {
	record   service.Record
	rendered string
	err      error
}

// --- Browse Tab ---

// BrowseTab navigates collections, records, and rendered markdown documents.
type BrowseTab struct {
	be              *backend
	level           browseLevel
	collections     []service.CollectionInfo
	records         service.RecordList
	record          service.Record
	rendered        string
	collectionIndex int
	recordIndex     int
	loading         bool
	err             error
	width           int
	height          int
	frame           int
}

type browseTab = BrowseTab

// NewBrowseTab creates the browse tab.
func NewBrowseTab(be *backend) BrowseTab {
	return BrowseTab{be: be}
}

func newBrowseTab(be *backend) BrowseTab {
	return NewBrowseTab(be)
}

// Resize stores the latest terminal size.
func (t *BrowseTab) Resize(width, height int) {
	t.width = width
	t.height = height
}

// Init loads the collection list.
func (t BrowseTab) Init() tea.Cmd {
	return t.loadCollections()
}

// Update handles browse navigation and async service reads.
func (t BrowseTab) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	switch msg := msg.(type) {
	case collectionsLoadedMsg:
		t.loading = false
		t.err = msg.err
		t.collections = msg.collections
		if t.collectionIndex >= len(t.collections) {
			t.collectionIndex = max(0, len(t.collections)-1)
		}
		return t, nil
	case recordsLoadedMsg:
		t.loading = false
		t.err = msg.err
		if msg.err == nil {
			t.level = levelRecords
			t.records = msg.records
			t.recordIndex = 0
			t.rendered = ""
			t.record = service.Record{}
		}
		return t, nil
	case recordLoadedMsg:
		t.loading = false
		t.err = msg.err
		if msg.err == nil {
			t.level = levelRecord
			t.record = msg.record
			t.rendered = msg.rendered
		}
		return t, nil
	case tea.KeyPressMsg:
		return t.updateKey(msg)
	}
	return t, nil
}

func (t BrowseTab) updateKey(msg tea.KeyPressMsg) (TabModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		switch t.level {
		case levelCollections:
			if len(t.collections) == 0 {
				return t, nil
			}
			t.loading = true
			return t, t.loadRecords(t.collections[t.collectionIndex].Name)
		case levelRecords:
			if len(t.records.Records) == 0 {
				return t, nil
			}
			t.loading = true
			return t, t.loadRecord(t.records.Records[t.recordIndex].Path)
		}
	case "esc":
		switch t.level {
		case levelRecord:
			t.level = levelRecords
			t.record = service.Record{}
			t.rendered = ""
		case levelRecords:
			t.level = levelCollections
			t.records = service.RecordList{}
			t.recordIndex = 0
		}
		return t, nil
	case "r":
		t.loading = true
		switch t.level {
		case levelCollections:
			return t, t.loadCollections()
		case levelRecords:
			return t, t.loadRecords(t.records.Collection)
		case levelRecord:
			return t, t.loadRecord(t.record.Path)
		}
	case "down":
		t.move(1)
		return t, nil
	case "up":
		t.move(-1)
		return t, nil
	}
	return t, nil
}

func (t *BrowseTab) move(delta int) {
	switch t.level {
	case levelCollections:
		t.collectionIndex = clampIndex(t.collectionIndex+delta, len(t.collections))
	case levelRecords:
		t.recordIndex = clampIndex(t.recordIndex+delta, len(t.records.Records))
	}
}

func (t BrowseTab) loadCollections() tea.Cmd {
	be := t.be
	return func() tea.Msg {
		if be == nil || be.svc == nil {
			return collectionsLoadedMsg{err: fmt.Errorf("browse unavailable: service is not open")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cols, err := be.svc.ListCollections(ctx)
		return collectionsLoadedMsg{collections: cols, err: err}
	}
}

func (t BrowseTab) loadRecords(name string) tea.Cmd {
	be := t.be
	return func() tea.Msg {
		if be == nil || be.svc == nil {
			return recordsLoadedMsg{collection: name, err: fmt.Errorf("browse unavailable: service is not open")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		list, err := be.svc.ListRecords(ctx, name, nil, "-updated_at", 200, 0)
		return recordsLoadedMsg{collection: name, records: list, err: err}
	}
}

func (t BrowseTab) loadRecord(path string) tea.Cmd {
	be := t.be
	return func() tea.Msg {
		if be == nil || be.svc == nil {
			return recordLoadedMsg{err: fmt.Errorf("browse unavailable: service is not open")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		rec, err := be.svc.GetRecord(ctx, path)
		if err != nil {
			return recordLoadedMsg{err: err}
		}
		return recordLoadedMsg{record: rec, rendered: render.GlamourRender(rec.Body, 96)}
	}
}

// View renders the current browse level.
func (t BrowseTab) View(width, height int) string {
	if width <= 0 {
		width = t.width
	}
	if height <= 0 {
		height = t.height
	}
	if t.err != nil {
		return centerBlockUniform(components.ErrorBox("browse failed", t.err.Error(), tableWidth(width)), width)
	}
	if t.loading {
		return centerOverlay(animatedBox(MutedStyle.Render("loading..."), t.frame), width, height)
	}
	switch t.level {
	case levelRecords:
		return t.viewRecords(width, height)
	case levelRecord:
		return t.viewRecord(width, height)
	default:
		return t.viewCollections(width, height)
	}
}

// Hints returns the key hints for the browse tab.
func (t BrowseTab) Hints() []components.HintItem {
	hints := []components.HintItem{
		{Key: "enter", Desc: "open"},
		{Key: "up/down", Desc: "select"},
		{Key: "r", Desc: "refresh"},
	}
	if t.level != levelCollections {
		hints = append(hints, components.HintItem{Key: "esc", Desc: "back"})
	}
	return withCommonHints(hints...)
}

// Focused reports whether browse owns keyboard text.
func (t BrowseTab) Focused() bool { return false }

// StatusLine returns the browse tab status text.
func (t BrowseTab) StatusLine() string {
	switch t.level {
	case levelRecords:
		return MutedStyle.Render(fmt.Sprintf("%s · %d records", t.records.Collection, len(t.records.Records)))
	case levelRecord:
		return MutedStyle.Render(t.record.Path)
	default:
		return MutedStyle.Render(fmt.Sprintf("%d collections", len(t.collections)))
	}
}

// HeaderLabel returns the shared animated header label.
func (t BrowseTab) HeaderLabel() string {
	switch t.level {
	case levelRecords:
		return "browse · records"
	case levelRecord:
		return "browse · document"
	default:
		return "browse · collections"
	}
}

func (t BrowseTab) viewCollections(width, height int) string {
	if len(t.collections) == 0 {
		return centerOverlay(animatedBox(MutedStyle.Render("no collections configured"), t.frame), width, height)
	}
	rows := make([][]string, 0, len(t.collections))
	for _, c := range t.collections {
		rows = append(rows, []string{
			c.Name,
			fmt.Sprintf("%d", c.Records),
			c.Path,
			c.Description,
		})
	}
	tableW := tableWidth(width)
	cols := []components.TableColumn{
		{Header: "Name", Width: 20, Align: lipgloss.Left},
		{Header: "Records", Width: 8, Align: lipgloss.Right},
		{Header: "Path", Width: 30, Align: lipgloss.Left},
		{Header: "Description", Width: tableW - 64, Align: lipgloss.Left},
	}
	if cols[3].Width < 24 {
		cols[3].Width = 24
	}
	grid := components.TableGridWithActiveRow(cols, rows, tableW, t.collectionIndex)
	return centerBlockUniform(animatedDoubleBox("COLLECTIONS", clipLines(grid, height-4), t.frame), width)
}

func (t BrowseTab) viewRecords(width, height int) string {
	if len(t.records.Records) == 0 {
		return centerOverlay(animatedBox(MutedStyle.Render("no records in collection"), t.frame), width, height)
	}
	rows := make([][]string, 0, len(t.records.Records))
	for _, r := range t.records.Records {
		rows = append(rows, []string{
			recordTitle(r),
			r.Path,
			recordUpdated(r),
		})
	}
	tableW := tableWidth(width)
	cols := []components.TableColumn{
		{Header: "Title", Width: 34, Align: lipgloss.Left},
		{Header: "Path", Width: tableW - 54, Align: lipgloss.Left},
		{Header: "Updated", Width: 16, Align: lipgloss.Left},
	}
	if cols[1].Width < 34 {
		cols[1].Width = 34
	}
	grid := components.TableGridWithActiveRow(cols, rows, tableW, t.recordIndex)
	return centerBlockUniform(animatedDoubleBox(strings.ToUpper(t.records.Collection), clipLines(grid, height-4), t.frame), width)
}

func (t BrowseTab) viewRecord(width, height int) string {
	title := recordTitle(t.record)
	header := HeaderStyle.Render(components.SanitizeOneLine(title)) + "\n"
	header += MutedStyle.Render(components.SanitizeOneLine(t.record.Path)) + "\n\n"
	body := t.rendered
	if body == "" {
		body = render.GlamourRender(t.record.Body, tableWidth(width)-8)
	}
	content := header + clipLines(body, height-6)
	return centerBlockUniform(animatedDoubleBox("", content, t.frame), width)
}

func recordTitle(r service.Record) string {
	if strings.TrimSpace(r.Title) != "" {
		return components.SanitizeOneLine(r.Title)
	}
	return components.SanitizeOneLine(r.Path)
}

func recordUpdated(r service.Record) string {
	for _, key := range []string{"updated_at", "updated", "modified"} {
		if v, ok := r.Frontmatter[key]; ok && v != nil {
			return components.SanitizeOneLine(fmt.Sprint(v))
		}
	}
	return ""
}

func clampIndex(index, length int) int {
	if length <= 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= length {
		return length - 1
	}
	return index
}
