package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
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

// --- Search Tab ---

// SearchTab is the interactive query surface over the service search engine.
type SearchTab struct {
	be       *backend
	input    textinput.Model
	spinner  spinner.Model
	loading  bool
	result   service.QueryResult
	previews map[string]string
	err      error
	cursor   int
	width    int
	height   int
	frame    int
}

type searchTab = SearchTab

// NewSearchTab creates the search tab.
func NewSearchTab(be *backend) SearchTab {
	ti := components.NewExoTextInput("search the vault")
	sp := components.NewExoSpinner()
	return SearchTab{be: be, input: ti, spinner: sp, previews: map[string]string{}}
}

func newSearchTab(be *backend) SearchTab {
	return NewSearchTab(be)
}

// Resize stores the latest terminal size.
func (t *SearchTab) Resize(width, height int) {
	t.width = width
	t.height = height
}

// Init focuses the input.
func (t SearchTab) Init() tea.Cmd {
	return t.input.Focus()
}

// Update handles search input, navigation, and async results.
func (t SearchTab) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		t.spinner, cmd = t.spinner.Update(msg)
		return t, cmd
	case searchDoneMsg:
		if msg.query != strings.TrimSpace(t.input.Value()) {
			return t, nil
		}
		t.loading = false
		t.result = msg.result
		t.previews = msg.previews
		t.err = msg.err
		t.cursor = 0
		return t, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			q := strings.TrimSpace(t.input.Value())
			if q == "" {
				return t, nil
			}
			t.loading = true
			t.err = nil
			return t, tea.Batch(t.runSearch(q), t.spinner.Tick)
		case "down":
			if t.cursor < len(t.result.Hits)-1 {
				t.cursor++
			}
			return t, nil
		case "up":
			if t.cursor > 0 {
				t.cursor--
			}
			return t, nil
		}
	}
	var cmd tea.Cmd
	t.input, cmd = t.input.Update(msg)
	return t, cmd
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

	if t.loading {
		msg := t.spinner.View() + " " + MutedStyle.Render("searching...")
		return centerOverlay(animatedBox(msg, t.frame), width, height)
	}
	if t.err != nil {
		return centerBlockUniform(components.ErrorBox("search failed", t.err.Error(), tableWidth(width)), width)
	}

	var b strings.Builder
	cardW := tableWidth(width)
	pill := pillStyle.Render("retrieval: " + t.retrievalLabel())
	b.WriteString(centerBlockUniform(pill, width) + "\n")
	b.WriteString(centerBlockUniform(t.input.View(), width) + "\n\n")

	if len(t.result.Hits) == 0 && strings.TrimSpace(t.input.Value()) == "" {
		return b.String() + centerOverlay(animatedBox(MutedStyle.Render("type a query and press enter"), t.frame), width, height-3)
	}
	if len(t.result.Hits) == 0 {
		return b.String() + centerOverlay(animatedBox(MutedStyle.Render("no matches"), t.frame), width, height-3)
	}

	listWidth := (cardW * 44) / 100
	if listWidth < 44 {
		listWidth = 44
	}
	previewWidth := cardW - listWidth - 4
	if previewWidth < 44 {
		previewWidth = 44
	}
	bodyHeight := height - 4
	if bodyHeight < 5 {
		bodyHeight = 5
	}

	list := animatedRoundedBox("RESULTS", t.renderHitTable(listWidth, bodyHeight), t.frame)
	preview := animatedRoundedBox("PREVIEW", t.renderPreview(previewWidth, bodyHeight), t.frame)
	body := lipgloss.JoinHorizontal(lipgloss.Top, list, "  ", preview)
	b.WriteString(centerBlockUniform(body, width))
	return b.String()
}

// Hints returns the key hints for the search tab.
func (t SearchTab) Hints() []components.HintItem {
	return withCommonHints(
		components.HintItem{Key: "enter", Desc: "search"},
		components.HintItem{Key: "up/down", Desc: "select"},
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

func (t SearchTab) renderHitTable(width, height int) string {
	rows := make([][]string, 0, len(t.result.Hits))
	for i, hit := range t.result.Hits {
		rows = append(rows, []string{
			fmt.Sprintf("%02d", i+1),
			fmt.Sprintf("%.4f", hit.Score),
			hitTitle(hit),
			hit.Path,
		})
	}
	cols := []components.TableColumn{
		{Header: "#", Width: 3, Align: lipgloss.Right},
		{Header: "Score", Width: 8, Align: lipgloss.Right},
		{Header: "Title", Width: 28, Align: lipgloss.Left},
		{Header: "Path", Width: width - 47, Align: lipgloss.Left},
	}
	if cols[3].Width < 16 {
		cols[3].Width = 16
	}
	return clipLines(components.TableGridWithActiveRow(cols, rows, width, t.cursor), height)
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
	rendered := clipLines(render.GlamourRender(md, width-4), height-4)
	return header + rendered
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

func hitTitle(hit index.Hit) string {
	if strings.TrimSpace(hit.Title) != "" {
		return components.SanitizeOneLine(hit.Title)
	}
	if strings.TrimSpace(hit.Heading) != "" {
		return components.SanitizeOneLine(hit.Heading)
	}
	return components.SanitizeOneLine(hit.Path)
}
