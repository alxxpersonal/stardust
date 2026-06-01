package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/alxxpersonal/stardust/internal/index"
	"github.com/alxxpersonal/stardust/internal/render"
)

// searchDoneMsg carries hybrid search results back to the search tab.
type searchDoneMsg struct {
	query string
	hits  []index.Hit
}

// searchTab is the interactive query surface over the hybrid index.
type searchTab struct {
	be      *backend
	input   textinput.Model
	spinner spinner.Model
	loading bool
	hits    []index.Hit
	cursor  int
}

// newSearchTab constructs the search tab with a focused input.
func newSearchTab(be *backend) searchTab {
	ti := textinput.New()
	ti.Placeholder = "search the vault, then press enter"
	ti.Focus()
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorAccent)
	return searchTab{be: be, input: ti, spinner: sp}
}

// Init focuses the input.
func (t searchTab) Init() tea.Cmd { return t.input.Focus() }

// Update handles search input, navigation, and async results.
func (t searchTab) Update(msg tea.Msg) (searchTab, tea.Cmd) {
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
		t.hits = msg.hits
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
			return t, tea.Batch(t.runSearch(q), t.spinner.Tick)
		case "down":
			if t.cursor < len(t.hits)-1 {
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

// runSearch embeds the query (when vectors are available) and runs hybrid search.
func (t searchTab) runSearch(query string) tea.Cmd {
	be := t.be
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var queryVec []float32
		if be.hasVec {
			if vecs, err := be.embed.Embed(ctx, []string{query}); err == nil && len(vecs) == 1 {
				queryVec = vecs[0]
			}
		}
		hits, err := be.store.Hybrid(ctx, query, queryVec, 12)
		if err != nil {
			return searchDoneMsg{query: query, hits: nil}
		}
		return searchDoneMsg{query: query, hits: hits}
	}
}

// View renders the input, the results list, and a preview of the selection.
func (t searchTab) View(width, height int) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Query ") + t.input.View() + "\n\n")

	switch {
	case t.loading:
		b.WriteString(t.spinner.View() + " " + mutedStyle.Render("searching..."))
		return b.String()
	case len(t.hits) == 0 && strings.TrimSpace(t.input.Value()) == "":
		b.WriteString(mutedStyle.Render("type a query and press enter for hybrid keyword + semantic search"))
		return b.String()
	case len(t.hits) == 0:
		b.WriteString(mutedStyle.Render("no matches"))
		return b.String()
	}

	listWidth := width/2 - 2
	if listWidth < 24 {
		listWidth = 24
	}
	previewWidth := width - listWidth - 4
	if previewWidth < 24 {
		previewWidth = 24
	}

	var list strings.Builder
	for i, h := range t.hits {
		title := h.Title
		if title == "" {
			title = h.Path
		}
		line := truncate(title, listWidth-3)
		if i == t.cursor {
			list.WriteString(accentStyle.Render("› "+line) + "\n")
		} else {
			list.WriteString(textStyle.Render("  "+line) + "\n")
		}
	}

	preview := t.renderPreview(t.hits[t.cursor], previewWidth, height-4)
	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		boxStyle.Width(listWidth).Height(height-4).Render(list.String()),
		" ",
		preview,
	)
	b.WriteString(body)
	return b.String()
}

// renderPreview glamour-renders the selected note's file, falling back to the
// matched snippet when the file cannot be read.
func (t searchTab) renderPreview(h index.Hit, width, height int) string {
	md := h.Snippet
	if raw, err := os.ReadFile(filepath.Join(t.be.layout.Root, h.Path)); err == nil {
		md = string(raw)
	}
	header := titleStyle.Render(h.Path) + "\n\n"
	rendered := clipLines(render.GlamourRender(md, width-2), height-3)
	return boxStyle.Width(width).Height(height).Render(header + rendered)
}
