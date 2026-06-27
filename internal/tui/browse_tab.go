package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
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
	renderedWidth   int
	docViewport     viewport.Model
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
	return BrowseTab{be: be, docViewport: components.NewExoViewport(80, 20)}
}

func newBrowseTab(be *backend) BrowseTab {
	return NewBrowseTab(be)
}

// Resize stores the latest terminal size.
func (t *BrowseTab) Resize(width, height int) {
	t.width = width
	t.height = height
	if t.level == levelRecord {
		t.refreshRecordViewport(false)
	}
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
			t.clearRecord()
		}
		return t, nil
	case recordLoadedMsg:
		t.loading = false
		t.err = msg.err
		if msg.err == nil {
			t.level = levelRecord
			t.record = msg.record
			t.rendered = msg.rendered
			t.renderedWidth = 0
			t.refreshRecordViewport(true)
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
			t.clearRecord()
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
	case "pgup", "pgdown", "home", "end":
		if t.level == levelRecord {
			t.updateRecordViewport(msg)
		}
		return t, nil
	case "down":
		if t.level == levelRecord {
			t.updateRecordViewport(msg)
			return t, nil
		}
		t.move(1)
		return t, nil
	case "up":
		if t.level == levelRecord {
			t.updateRecordViewport(msg)
			return t, nil
		}
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

func (t *BrowseTab) updateRecordViewport(msg tea.KeyPressMsg) {
	t.refreshRecordViewport(false)
	switch msg.String() {
	case "home":
		t.docViewport.GotoTop()
	case "end":
		t.docViewport.GotoBottom()
	default:
		t.docViewport, _ = t.docViewport.Update(msg)
	}
}

func (t *BrowseTab) refreshRecordViewport(reset bool) {
	width := t.recordViewportWidth()
	height := t.recordViewportHeight()
	t.docViewport.SetWidth(width)
	t.docViewport.SetHeight(height)

	if t.record.Body != "" && (t.rendered == "" || t.renderedWidth != width) {
		t.rendered = render.GlamourRender(t.record.Body, width)
		t.renderedWidth = width
	}
	t.docViewport.SetContent(t.rendered)
	if reset {
		t.docViewport.GotoTop()
	}
}

func (t *BrowseTab) clearRecord() {
	t.record = service.Record{}
	t.rendered = ""
	t.renderedWidth = 0
	t.docViewport.SetContent("")
	t.docViewport.GotoTop()
}

func (t BrowseTab) recordViewportWidth() int {
	return recordViewportWidthFor(t.width)
}

func (t BrowseTab) recordViewportHeight() int {
	return recordViewportHeightFor(t.height)
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
	var hints []components.HintItem
	if t.level == levelRecord {
		hints = []components.HintItem{
			{Key: "up/down", Desc: "scroll"},
			{Key: "pgup/pgdn", Desc: "page"},
			{Key: "home/end", Desc: "top/bottom"},
			{Key: "r", Desc: "refresh"},
			{Key: "esc", Desc: "back"},
		}
	} else {
		hints = []components.HintItem{
			{Key: "enter", Desc: "open"},
			{Key: "up/down", Desc: "select"},
			{Key: "r", Desc: "refresh"},
		}
		if t.level != levelCollections {
			hints = append(hints, components.HintItem{Key: "esc", Desc: "back"})
		}
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
	rows := make([]cleanListRow, 0, len(t.collections))
	for _, c := range t.collections {
		rows = append(rows, cleanListRow{Cells: []string{
			components.SanitizeOneLine(c.Name),
			fmt.Sprintf("%d", c.Records),
			components.SanitizeOneLine(c.Path),
			components.SanitizeOneLine(c.Description),
		}})
	}
	tableW := tableWidth(width)
	cols := []cleanListColumn{
		{Header: "Name", MinWidth: 12, MaxWidth: 24, Primary: true},
		{Header: "Records", MinWidth: 7, MaxWidth: 8, Align: lipgloss.Right, Count: true},
		{Header: "Path", MinWidth: 16, MaxWidth: 36, Muted: true, Underline: true},
		{Header: "Description", MinWidth: 20, MaxWidth: 72, Muted: true},
	}
	label := ""
	if t.collectionIndex >= 0 && t.collectionIndex < len(t.collections) {
		label = t.collections[t.collectionIndex].Name
	}
	list := renderCleanList("Collections", label, cols, rows, tableW, t.collectionIndex)
	return centerBlockUniform(animatedDoubleBox("", clipLines(list, height-4), t.frame), width)
}

func (t BrowseTab) viewRecords(width, height int) string {
	if len(t.records.Records) == 0 {
		return centerOverlay(animatedBox(MutedStyle.Render("no records in collection"), t.frame), width, height)
	}
	hasUpdated := false
	for _, r := range t.records.Records {
		if recordUpdated(r) != "" {
			hasUpdated = true
			break
		}
	}

	rows := make([]cleanListRow, 0, len(t.records.Records))
	for _, r := range t.records.Records {
		cells := []string{recordTitle(r)}
		if hasUpdated {
			cells = append(cells, recordUpdated(r))
		}
		cells = append(cells, components.SanitizeOneLine(r.Path))
		rows = append(rows, cleanListRow{Cells: cells})
	}
	tableW := tableWidth(width)
	cols := []cleanListColumn{
		{Header: "Title", MinWidth: 22, Primary: true},
	}
	if hasUpdated {
		cols = append(cols, cleanListColumn{Header: "Date", MinWidth: 10, MaxWidth: 18, Muted: true})
	}
	cols = append(cols, cleanListColumn{Header: "Path", MinWidth: 20, MaxWidth: 72, Muted: true, Underline: true})
	label := ""
	if t.recordIndex >= 0 && t.recordIndex < len(t.records.Records) {
		label = recordTitle(t.records.Records[t.recordIndex])
	}
	list := renderCleanList(t.records.Collection, label, cols, rows, tableW, t.recordIndex)
	return centerBlockUniform(animatedDoubleBox("", clipLines(list, height-4), t.frame), width)
}

func (t BrowseTab) viewRecord(width, height int) string {
	title := recordTitle(t.record)
	header := HeaderStyle.Render(components.SanitizeOneLine(title)) + "\n"
	header += MutedStyle.Render(components.SanitizeOneLine(t.record.Path)) + "\n\n"
	viewport := t.docViewport
	viewport.SetWidth(recordViewportWidthFor(width))
	viewport.SetHeight(recordViewportHeightFor(height))
	if viewport.GetContent() == "" && (t.rendered != "" || t.record.Body != "") {
		body := t.rendered
		if body == "" {
			body = render.GlamourRender(t.record.Body, recordViewportWidthFor(width))
		}
		viewport.SetContent(body)
	}
	content := header + viewport.View()
	return centerBlockUniform(animatedDoubleBox("", content, t.frame), width)
}

func recordViewportWidthFor(width int) int {
	viewportWidth := tableWidth(width) - 6
	if viewportWidth < 20 {
		viewportWidth = 20
	}
	return viewportWidth
}

func recordViewportHeightFor(height int) int {
	viewportHeight := height - 7
	if viewportHeight < 1 {
		viewportHeight = 1
	}
	return viewportHeight
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
			return shortDate(components.SanitizeOneLine(fmt.Sprint(v)))
		}
	}
	return ""
}

// shortDate trims a timestamp to its YYYY-MM-DD date prefix when one is present,
// so a record date renders as 2026-06-26 instead of 2026-06-26T00:00:00Z.
func shortDate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 10 && (s[10] == 'T' || s[10] == ' ') {
		return s[:10]
	}
	return s
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
