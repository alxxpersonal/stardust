package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/alxxpersonal/stardust/internal/index"
	"github.com/alxxpersonal/stardust/internal/render"
	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/internal/tui/components"
)

// --- Search Messages ---

// searchDoneMsg carries service search results back to the search tab.
type searchDoneMsg struct {
	query    string
	result   service.QueryResult
	previews map[string]string
	err      error
}

type searchDebounceMsg struct {
	query string
}

const searchDebounceDelay = 180 * time.Millisecond

// --- Search Tab ---

// SearchTab is the interactive query surface over the service search engine.
type SearchTab struct {
	be              *backend
	input           textinput.Model
	spinner         spinner.Model
	loading         bool
	result          service.QueryResult
	previews        map[string]string
	err             error
	cursor          int
	previewViewport viewport.Model
	previewPath     string
	previewRendered string
	previewWidth    int
	width           int
	height          int
	frame           int
}

type searchTab = SearchTab

// NewSearchTab creates the search tab.
func NewSearchTab(be *backend) SearchTab {
	ti := components.NewExoTextInput("search the vault")
	sp := components.NewExoSpinner()
	return SearchTab{
		be:              be,
		input:           ti,
		spinner:         sp,
		previews:        map[string]string{},
		previewViewport: components.NewExoViewport(80, 20),
	}
}

func newSearchTab(be *backend) SearchTab {
	return NewSearchTab(be)
}

// Resize stores the latest terminal size.
func (t *SearchTab) Resize(width, height int) {
	t.width = width
	t.height = height
	t.refreshPreviewViewport(false)
}

// Init focuses the input.
func (t SearchTab) Init() tea.Cmd {
	return t.input.Focus()
}

// Update handles search input, navigation, and async results.
func (t SearchTab) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if !t.loading {
			return t, nil
		}
		var cmd tea.Cmd
		t.spinner, cmd = t.spinner.Update(msg)
		return t, cmd
	case searchDebounceMsg:
		q := strings.TrimSpace(t.input.Value())
		if q == "" || msg.query != q {
			return t, nil
		}
		return t.startSearch(q)
	case searchDoneMsg:
		if msg.query != strings.TrimSpace(t.input.Value()) {
			return t, nil
		}
		t.loading = false
		t.result = msg.result
		t.previews = msg.previews
		t.err = msg.err
		t.cursor = 0
		t.refreshPreviewViewport(true)
		return t, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			q := strings.TrimSpace(t.input.Value())
			if q == "" {
				return t, nil
			}
			return t.startSearch(q)
		case "down":
			if t.cursor < len(t.result.Hits)-1 {
				t.cursor++
				t.refreshPreviewViewport(true)
			}
			return t, nil
		case "up":
			if t.cursor > 0 {
				t.cursor--
				t.refreshPreviewViewport(true)
			}
			return t, nil
		case "pgup", "pgdown", "home", "end":
			t.updatePreviewViewport(msg)
			return t, nil
		}
	}
	before := strings.TrimSpace(t.input.Value())
	var cmd tea.Cmd
	t.input, cmd = t.input.Update(msg)
	after := strings.TrimSpace(t.input.Value())
	if after == before {
		return t, cmd
	}
	t.err = nil
	t.cursor = 0
	t.result = service.QueryResult{Query: after, RetrievalMode: t.result.RetrievalMode, Mode: t.result.Mode}
	t.previews = map[string]string{}
	t.clearPreviewViewport()
	if after == "" {
		t.loading = false
		return t, cmd
	}
	t.loading = true
	return t, tea.Batch(cmd, t.debounceSearch(after), t.spinner.Tick)
}

func (t SearchTab) startSearch(query string) (TabModel, tea.Cmd) {
	t.loading = true
	t.err = nil
	return t, tea.Batch(t.runSearch(query), t.spinner.Tick)
}

func (t SearchTab) debounceSearch(query string) tea.Cmd {
	return tea.Tick(searchDebounceDelay, func(time.Time) tea.Msg {
		return searchDebounceMsg{query: query}
	})
}

func (t SearchTab) runSearch(query string) tea.Cmd {
	be := t.be
	return func() tea.Msg {
		if be == nil || be.svc == nil {
			return searchDoneMsg{query: query, err: fmt.Errorf("search unavailable: service is not open")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := be.svc.Query(ctx, query, 12)
		if err != nil {
			return searchDoneMsg{query: query, err: err}
		}
		previews := make(map[string]string, len(result.Hits))
		for _, hit := range result.Hits {
			note, noteErr := be.svc.GetNote(ctx, hit.Path)
			if noteErr != nil {
				previews[hit.Path] = hit.Snippet
				continue
			}
			previews[hit.Path] = note.Body
		}
		return searchDoneMsg{query: query, result: result, previews: previews}
	}
}

func (t *SearchTab) updatePreviewViewport(msg tea.KeyPressMsg) {
	t.refreshPreviewViewport(false)
	switch msg.String() {
	case "home":
		t.previewViewport.GotoTop()
	case "end":
		t.previewViewport.GotoBottom()
	case "pgup":
		t.previewViewport.PageUp()
	case "pgdown":
		t.previewViewport.PageDown()
	}
}

func (t *SearchTab) refreshPreviewViewport(reset bool) {
	paneWidth, paneHeight := t.previewPaneSize(t.width, t.height)
	width := previewViewportWidthFor(paneWidth)
	height := previewViewportHeightFor(paneHeight)
	t.previewViewport.SetWidth(width)
	t.previewViewport.SetHeight(height)

	hit := t.selectedHit()
	if hit.Path == "" {
		t.clearPreviewViewport()
		return
	}

	oldPath := t.previewPath
	if t.previewRendered == "" || oldPath != hit.Path || t.previewWidth != width {
		t.previewRendered = render.GlamourRender(t.previewMarkdown(hit), width)
		t.previewWidth = width
		t.previewPath = hit.Path
		t.previewViewport.SetContent(t.previewRendered)
	}
	if reset || oldPath != hit.Path {
		t.previewViewport.GotoTop()
	}
}

func (t *SearchTab) clearPreviewViewport() {
	t.previewPath = ""
	t.previewRendered = ""
	t.previewWidth = 0
	t.previewViewport.SetContent("")
	t.previewViewport.GotoTop()
}

// View renders the input, hit list, retrieval mode, and selected markdown preview.
func (t SearchTab) View(width, height int) string {
	if width <= 0 {
		width = t.width
	}
	if height <= 0 {
		height = t.height
	}
	if width < 60 {
		width = 60
	}
	if height < 6 {
		height = 6
	}

	if t.err != nil {
		return centerBlockUniform(components.ErrorBox("search failed", t.err.Error(), tableWidth(width)), width)
	}

	boxW := searchBoxWidth(width)
	contentW := boxW - 6
	if contentW < 40 {
		contentW = 40
	}
	contentH := height - 4
	if contentH < 5 {
		contentH = 5
	}

	header := t.renderSearchControls(contentW)
	headerLines := countViewLines(header)
	bodyH := contentH - headerLines - 1
	if bodyH < 3 {
		bodyH = 3
	}

	body := t.renderSearchBody(contentW, bodyH)
	content := header + "\n" + body
	return centerBlockUniform(animatedDoubleBox("", content, t.frame), width)
}

func searchBoxWidth(width int) int {
	boxW := tableWidth(width)
	if width > 0 && boxW > width-2 {
		boxW = width - 2
	}
	if boxW < 46 {
		boxW = 46
	}
	return boxW
}

func (t SearchTab) renderSearchControls(width int) string {
	pill := pillStyle.Render("retrieval: " + t.retrievalLabel())
	if t.loading {
		pill = pillStyle.Render("retrieval: "+t.retrievalLabel()) + " " + t.spinner.View()
	}

	input := t.input
	pillW := lipgloss.Width(pill)
	inputW := width - pillW - 2
	if inputW < 22 {
		input.SetWidth(width - lipgloss.Width(input.Prompt))
		return fitBlock(input.View()+"\n"+pill, width, 2)
	}
	input.SetWidth(inputW - lipgloss.Width(input.Prompt))
	if input.Width() < 1 {
		input.SetWidth(inputW)
	}
	line := input.View() + strings.Repeat(" ", 2) + pill
	return padStyledLine(line, width)
}

func (t SearchTab) renderSearchBody(width, height int) string {
	if len(t.result.Hits) == 0 {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, t.emptyHint())
	}
	if width < 92 {
		return t.renderVerticalSearchBody(width, height)
	}
	return t.renderHorizontalSearchBody(width, height)
}

func (t SearchTab) emptyHint() string {
	query := strings.TrimSpace(t.input.Value())
	switch {
	case t.loading:
		return MutedStyle.Render("Searching your vault")
	case query == "":
		return MutedStyle.Render("Type to search your vault")
	default:
		return MutedStyle.Render("No matches yet")
	}
}

func (t SearchTab) renderHorizontalSearchBody(width, height int) string {
	listW, previewW := horizontalSearchPaneWidths(width)
	if listW < 24 {
		return t.renderVerticalSearchBody(width, height)
	}

	list := fitBlock(t.renderResultsList(listW, height), listW, height)
	preview := fitBlock(t.renderPreview(previewW, height), previewW, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, list, " ", verticalRule(height), " ", preview)
}

func (t SearchTab) renderVerticalSearchBody(width, height int) string {
	listH, previewH := verticalSearchPaneHeights(height)
	list := fitBlock(t.renderResultsList(width, listH), width, listH)
	preview := fitBlock(t.renderPreview(width, previewH), width, previewH)
	return list + "\n" + Divider(width) + "\n" + preview
}

// Hints returns the key hints for the search tab.
func (t SearchTab) Hints() []components.HintItem {
	return withCommonHints(
		components.HintItem{Key: "type", Desc: "search"},
		components.HintItem{Key: "enter", Desc: "run now"},
		components.HintItem{Key: "up/down", Desc: "results"},
		components.HintItem{Key: "pgup/pgdn", Desc: "preview"},
	)
}

// Focused reports whether the search input owns keyboard text.
func (t SearchTab) Focused() bool {
	return t.input.Focused()
}

// StatusLine returns the search tab status text.
func (t SearchTab) StatusLine() string {
	if t.err != nil {
		return ErrorStyle.Render(t.err.Error())
	}
	if t.loading {
		return MutedStyle.Render("searching...")
	}
	if t.result.RetrievalReason != "" {
		return MutedStyle.Render(t.result.RetrievalReason)
	}
	return MutedStyle.Render(fmt.Sprintf("%d results · %s", len(t.result.Hits), t.retrievalLabel()))
}

// HeaderLabel returns the shared animated header label.
func (t SearchTab) HeaderLabel() string {
	return "search · hybrid retrieval"
}

func (t SearchTab) retrievalLabel() string {
	switch t.result.RetrievalMode {
	case service.RetrievalHybridSemantic:
		return "hybrid"
	case service.RetrievalFTSOnly:
		return "keyword"
	}
	if t.result.Mode != "" {
		return t.result.Mode
	}
	return "hybrid"
}

func (t SearchTab) renderResultsList(width, height int) string {
	rows := make([]cleanListRow, 0, len(t.result.Hits))
	for _, hit := range t.result.Hits {
		rows = append(rows, cleanListRow{Cells: []string{
			hitTitle(hit),
			components.SanitizeOneLine(hit.Path),
			searchHitSnippet(hit),
		}})
	}
	cols := []cleanListColumn{
		{Header: "Title", MinWidth: 10, MaxWidth: 36, Primary: true},
		{Header: "Path", MinWidth: 10, MaxWidth: 42, Muted: true, Underline: true},
		{Header: "Match", MinWidth: 10, MaxWidth: 56, Muted: true},
	}
	label := t.retrievalLabel()
	if len(t.result.Hits) > 0 {
		label = cleanListCountLabel(len(t.result.Hits), "hit")
	}
	return clipLines(renderCleanList("Results", label, cols, rows, width, t.cursor), height)
}

func (t SearchTab) renderPreview(width, height int) string {
	hit := t.selectedHit()
	if hit.Path == "" {
		return MutedStyle.Render("no preview")
	}
	md := t.previews[hit.Path]
	if strings.TrimSpace(md) == "" {
		md = hit.Snippet
	}
	header := HeaderStyle.Render(components.SanitizeOneLine(hitTitle(hit))) + "\n"
	header += MutedStyle.Render(components.SanitizeOneLine(hit.Path)) + "\n\n"
	viewport := t.previewViewport
	viewport.SetWidth(previewViewportWidthFor(width))
	viewport.SetHeight(previewViewportHeightFor(height))
	if viewport.GetContent() == "" || t.previewPath != hit.Path || t.previewWidth != viewport.Width() {
		viewport.SetContent(render.GlamourRender(md, viewport.Width()))
		viewport.GotoTop()
	}
	return header + viewport.View()
}

func (t SearchTab) selectedHit() index.Hit {
	if len(t.result.Hits) == 0 {
		return index.Hit{}
	}
	cursor := t.cursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(t.result.Hits) {
		cursor = len(t.result.Hits) - 1
	}
	return t.result.Hits[cursor]
}

func (t SearchTab) previewMarkdown(hit index.Hit) string {
	md := t.previews[hit.Path]
	if strings.TrimSpace(md) == "" {
		md = hit.Snippet
	}
	return md
}

func (t SearchTab) previewPaneSize(width, height int) (int, int) {
	if width <= 0 {
		width = t.width
	}
	if height <= 0 {
		height = t.height
	}
	if width < 60 {
		width = 60
	}
	if height < 6 {
		height = 6
	}

	boxW := searchBoxWidth(width)
	contentW := boxW - 6
	if contentW < 40 {
		contentW = 40
	}
	contentH := height - 4
	if contentH < 5 {
		contentH = 5
	}

	header := t.renderSearchControls(contentW)
	bodyH := contentH - countViewLines(header) - 1
	if bodyH < 3 {
		bodyH = 3
	}

	if contentW < 92 {
		_, previewH := verticalSearchPaneHeights(bodyH)
		return contentW, previewH
	}
	listW, previewW := horizontalSearchPaneWidths(contentW)
	if listW < 24 {
		_, previewH := verticalSearchPaneHeights(bodyH)
		return contentW, previewH
	}
	return previewW, bodyH
}

func horizontalSearchPaneWidths(width int) (int, int) {
	gapW := 3
	listW := (width * 40) / 100
	if listW < 36 {
		listW = 36
	}
	previewW := width - listW - gapW
	if previewW < 40 {
		previewW = 40
		listW = width - previewW - gapW
	}
	return listW, previewW
}

func verticalSearchPaneHeights(height int) (int, int) {
	listH := (height * 45) / 100
	if listH < 5 {
		listH = 5
	}
	previewH := height - listH - 1
	if previewH < 3 {
		previewH = 3
		listH = height - previewH - 1
	}
	return listH, previewH
}

func previewViewportWidthFor(width int) int {
	viewportWidth := width - 4
	if viewportWidth < 20 {
		viewportWidth = 20
	}
	return viewportWidth
}

func previewViewportHeightFor(height int) int {
	viewportHeight := height - 4
	if viewportHeight < 1 {
		viewportHeight = 1
	}
	return viewportHeight
}

func hitTitle(hit index.Hit) string {
	if strings.TrimSpace(hit.Title) != "" {
		return components.SanitizeOneLine(hit.Title)
	}
	if strings.TrimSpace(hit.Heading) != "" {
		return components.SanitizeOneLine(hit.Heading)
	}
	return components.SanitizeOneLine(hit.Path)
}

func searchHitSnippet(hit index.Hit) string {
	if strings.TrimSpace(hit.Snippet) == "" {
		return ""
	}
	return components.SanitizeOneLine(hit.Snippet)
}

func fitBlock(block string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := strings.Split(clipLines(block, height), "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i := range lines {
		lines[i] = padStyledLine(lines[i], width)
	}
	return strings.Join(lines, "\n")
}

func padStyledLine(line string, width int) string {
	if width <= 0 {
		return line
	}
	pad := width - lipgloss.Width(line)
	if pad <= 0 {
		return line
	}
	return line + strings.Repeat(" ", pad)
}

func verticalRule(height int) string {
	if height <= 0 {
		return ""
	}
	lines := make([]string, height)
	for i := range lines {
		lines[i] = DividerStyle.Render("│")
	}
	return strings.Join(lines, "\n")
}
