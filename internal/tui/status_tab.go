package tui

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/internal/tui/components"
)

// --- Status Messages ---

// statusLoadedMsg carries the full vault status back to the status tab.
type statusLoadedMsg struct {
	status service.VaultStatus
	err    error
}

// --- Status Tab ---

// StatusTab shows vault kind, collections, and index health.
type StatusTab struct {
	be     *backend
	loaded bool
	err    error
	status service.VaultStatus
	width  int
	height int
	frame  int
}

type statusTab = StatusTab

// NewStatusTab creates the status tab.
func NewStatusTab(be *backend) StatusTab {
	return StatusTab{be: be}
}

func newStatusTab(be *backend) StatusTab {
	return NewStatusTab(be)
}

// Resize stores the latest terminal size.
func (t *StatusTab) Resize(width, height int) {
	t.width = width
	t.height = height
}

// Init loads the status report.
func (t StatusTab) Init() tea.Cmd { return t.load() }

func (t StatusTab) load() tea.Cmd {
	be := t.be
	return func() tea.Msg {
		if be == nil || be.svc == nil {
			return statusLoadedMsg{err: fmt.Errorf("status unavailable: service is not open")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		status, err := service.GatherStatus(ctx, be.svc.Layout.Root)
		return statusLoadedMsg{status: status, err: err}
	}
}

// Update applies loaded status and handles reload.
func (t StatusTab) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	switch msg := msg.(type) {
	case statusLoadedMsg:
		t.loaded = true
		t.err = msg.err
		if msg.err == nil {
			t.status = msg.status
		}
	case tea.KeyPressMsg:
		if msg.String() == "r" {
			t.loaded = false
			return t, t.load()
		}
	}
	return t, nil
}

// View renders the status rows.
func (t StatusTab) View(width, height int) string {
	if width <= 0 {
		width = t.width
	}
	if height <= 0 {
		height = t.height
	}
	if t.err != nil {
		return centerBlockUniform(components.ErrorBox("status failed", t.err.Error(), tableWidth(width)), width)
	}
	if !t.loaded {
		return centerOverlay(animatedBox(MutedStyle.Render("loading status..."), t.frame), width, height)
	}

	cardW := tableWidth(width)
	leftWidth := (cardW / 2) - 2
	if leftWidth < 44 {
		leftWidth = 44
	}
	rightWidth := cardW - leftWidth - 4
	if rightWidth < 44 {
		rightWidth = 44
	}

	top := lipgloss.JoinHorizontal(
		lipgloss.Top,
		animatedRoundedBox("VAULT", t.summary(leftWidth), t.frame),
		"  ",
		animatedRoundedBox("INDEX", t.indexHealth(rightWidth), t.frame),
	)
	collections := animatedDoubleBox("", t.collections(cardW, height/2), t.frame)
	return centerBlockUniform(top+"\n\n"+collections, width)
}

// Hints returns the key hints for the status tab.
func (t StatusTab) Hints() []components.HintItem {
	return withCommonHints(components.HintItem{Key: "r", Desc: "refresh"})
}

// Focused reports whether status owns keyboard text.
func (t StatusTab) Focused() bool { return false }

// StatusLine returns the status tab status text.
func (t StatusTab) StatusLine() string {
	if !t.loaded {
		return MutedStyle.Render("status loading")
	}
	return MutedStyle.Render(fmt.Sprintf("%s · %d notes · vectors %s",
		t.status.Kind, t.status.Index.Notes, boolWord(t.status.Index.Vectors)))
}

// HeaderLabel returns the shared animated header label.
func (t StatusTab) HeaderLabel() string {
	return "status · vault health"
}

func (t StatusTab) summary(width int) string {
	rows := []components.TableRow{
		{Label: "Root", Value: t.status.Root},
		{Label: "Initialized", Value: boolWord(t.status.Initialized)},
		{Label: "Kind", Value: t.status.Kind},
	}
	if t.status.Hint != "" {
		rows = append(rows, components.TableRow{Label: "Hint", Value: t.status.Hint})
	}
	return components.Table("", rows, width)
}

func (t StatusTab) indexHealth(width int) string {
	last := t.status.Index.LastIndexed
	if len(last) > 12 {
		last = last[:12]
	}
	if last == "" {
		last = "unknown"
	}
	freshness := "unknown"
	if t.status.Index.HasCommitsBehind {
		freshness = fmt.Sprintf("%d commits behind HEAD", t.status.Index.CommitsBehind)
	}
	vectors := "off"
	if t.status.Index.Vectors {
		vectors = "on"
	} else if t.status.Index.VectorsReason != "" {
		vectors = "off (" + t.status.Index.VectorsReason + ")"
	}
	rows := []components.TableRow{
		{Label: "Notes", Value: fmt.Sprintf("%d", t.status.Index.Notes)},
		{Label: "Vectors", Value: vectors},
		{Label: "Freshness", Value: freshness},
		{Label: "Last indexed", Value: last},
		{Label: "Embed model", Value: t.status.Index.EmbedModel},
	}
	return components.Table("", rows, width)
}

func (t StatusTab) collections(width, height int) string {
	if len(t.status.Collections) == 0 {
		return MutedStyle.Render("no collections configured")
	}
	rows := make([]cleanListRow, 0, len(t.status.Collections))
	for _, c := range t.status.Collections {
		rows = append(rows, cleanListRow{Cells: []string{
			components.SanitizeOneLine(c.Name),
			fmt.Sprintf("%d", c.Records),
			components.SanitizeOneLine(c.Path),
			components.SanitizeOneLine(c.Description),
		}})
	}
	cols := []cleanListColumn{
		{Header: "Collection", MinWidth: 14, MaxWidth: 28, Primary: true},
		{Header: "Records", MinWidth: 7, MaxWidth: 8, Align: lipgloss.Right, Count: true},
		{Header: "Path", MinWidth: 16, MaxWidth: 40, Muted: true, Underline: true},
		{Header: "Description", MinWidth: 18, MaxWidth: 72, Muted: true},
	}
	return clipLines(renderCleanList("Collections", cleanListCountLabel(len(t.status.Collections), "collection"), cols, rows, width, -1), height)
}
