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

// --- Drift Messages ---

// driftLoadedMsg carries coherence reports back to the drift tab.
type driftLoadedMsg struct {
	check service.CheckResult
	drift service.DriftResult
	stale service.StaleResult
	err   error
}

// --- Drift Tab ---

// DriftTab shows check, drift, and stale-doc reports.
type DriftTab struct {
	be     *backend
	loaded bool
	err    error
	check  service.CheckResult
	drift  service.DriftResult
	stale  service.StaleResult
	width  int
	height int
	frame  int
}

type driftTab = DriftTab

// NewDriftTab creates the drift tab.
func NewDriftTab(be *backend) DriftTab {
	return DriftTab{be: be}
}

func newDriftTab(be *backend) DriftTab {
	return NewDriftTab(be)
}

// Resize stores the latest terminal size.
func (t *DriftTab) Resize(width, height int) {
	t.width = width
	t.height = height
}

// Init loads the coherence reports.
func (t DriftTab) Init() tea.Cmd { return t.load() }

func (t DriftTab) load() tea.Cmd {
	be := t.be
	return func() tea.Msg {
		if be == nil || be.svc == nil {
			return driftLoadedMsg{err: fmt.Errorf("drift unavailable: service is not open")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		check, err := be.svc.Check(ctx)
		if err != nil {
			return driftLoadedMsg{err: err}
		}
		drift, err := be.svc.DriftDocs(ctx)
		if err != nil {
			return driftLoadedMsg{err: err}
		}
		stale, err := be.svc.StaleDocs(ctx)
		if err != nil {
			return driftLoadedMsg{err: err}
		}
		return driftLoadedMsg{check: check, drift: drift, stale: stale}
	}
}

// Update applies coherence reports and handles reload.
func (t DriftTab) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	switch msg := msg.(type) {
	case driftLoadedMsg:
		t.loaded = true
		t.err = msg.err
		if msg.err == nil {
			t.check = msg.check
			t.drift = msg.drift
			t.stale = msg.stale
		}
	case tea.KeyPressMsg:
		if msg.String() == "r" {
			t.loaded = false
			return t, t.load()
		}
	}
	return t, nil
}

// View renders the coherence report.
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
	leftWidth := (cardW * 48) / 100
	if leftWidth < 48 {
		leftWidth = 48
	}
	rightWidth := cardW - leftWidth - 4
	if rightWidth < 48 {
		rightWidth = 48
	}
	boxHeight := height - 2
	if boxHeight < 8 {
		boxHeight = 8
	}

	left := animatedRoundedBox("CHECK REPORT", t.issueBox(leftWidth, boxHeight), t.frame)
	right := animatedRoundedBox("DRIFT + STALE", t.markdownBox(rightWidth, boxHeight), t.frame)
	return centerBlockUniform(lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right), width)
}

// Hints returns the key hints for the drift tab.
func (t DriftTab) Hints() []components.HintItem {
	return withCommonHints(components.HintItem{Key: "r", Desc: "refresh"})
}

// Focused reports whether drift owns keyboard text.
func (t DriftTab) Focused() bool { return false }

// StatusLine returns the drift tab status text.
func (t DriftTab) StatusLine() string {
	if !t.loaded {
		return MutedStyle.Render("drift loading")
	}
	style := MutedStyle
	if t.check.Errors > 0 {
		style = ErrorStyle
	} else if t.check.Warnings > 0 || len(t.drift.Docs) > 0 || len(t.stale.Docs) > 0 {
		style = WarningStyle
	}
	return style.Render(fmt.Sprintf("%d errors · %d warnings · %d drifted docs · %d stale docs",
		t.check.Errors, t.check.Warnings, len(t.drift.Docs), len(t.stale.Docs)))
}

// HeaderLabel returns the shared animated header label.
func (t DriftTab) HeaderLabel() string {
	return "drift · coherence checks"
}

func (t DriftTab) issueBox(width, height int) string {
	if len(t.check.Issues) == 0 {
		return SuccessStyle.Render("clean")
	}
	rows := make([][]string, 0, len(t.check.Issues))
	for _, issue := range t.check.Issues {
		rows = append(rows, []string{
			issue.Severity,
			issue.Kind,
			components.SanitizeOneLine(issue.Path),
			components.SanitizeOneLine(issue.Detail),
		})
	}
	cols := []components.TableColumn{
		{Header: "Severity", Width: 8, Align: lipgloss.Left},
		{Header: "Kind", Width: 16, Align: lipgloss.Left},
		{Header: "Path", Width: 26, Align: lipgloss.Left},
		{Header: "Detail", Width: width - 56, Align: lipgloss.Left},
	}
	if cols[3].Width < 24 {
		cols[3].Width = 24
	}
	return clipLines(components.TableGrid(cols, rows, width), height)
}

func (t DriftTab) markdownBox(width, height int) string {
	var md strings.Builder
	md.WriteString("# Drifted Docs\n\n")
	if strings.TrimSpace(t.drift.Markdown) != "" {
		md.WriteString(t.drift.Markdown)
	} else {
		md.WriteString("No drifted docs found.\n")
	}
	md.WriteString("\n\n# Stale Docs\n\n")
	if strings.TrimSpace(t.stale.Markdown) != "" {
		md.WriteString(t.stale.Markdown)
	} else {
		md.WriteString("No stale docs found.\n")
	}
	return clipLines(render.GlamourRender(md.String(), width-4), height)
}
