package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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

// View renders the coherence report as a vertical stack of clean lists. Each box
// renders the typed result slices (not the service Markdown), so exactly one
// keycap header is emitted per list and duplicate headers are impossible.
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

// summaryLine renders the one-line coherence summary, colored by worst severity.
func (t DriftTab) summaryLine(width int) string {
	_ = width
	style := MutedStyle
	if t.check.Errors > 0 {
		style = ErrorStyle
	} else if t.check.Warnings > 0 || len(t.drift.Docs) > 0 || len(t.stale.Docs) > 0 {
		style = WarningStyle
	}
	return style.Render(fmt.Sprintf("%d errors · %d warnings · %d drifted · %d stale",
		t.check.Errors, t.check.Warnings, len(t.drift.Docs), len(t.stale.Docs)))
}

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

// checkSummary groups check findings by kind with a count and a sample, errors
// first, so the report is scannable instead of one row per finding.
func (t DriftTab) checkSummary(width, height int) string {
	if len(t.check.Issues) == 0 {
		return SuccessStyle.Render("clean")
	}
	type group struct {
		severity string
		kind     string
		sample   string
		count    int
	}
	order := []string{}
	byKind := map[string]*group{}
	for _, is := range t.check.Issues {
		g, ok := byKind[is.Kind]
		if !ok {
			g = &group{severity: is.Severity, kind: is.Kind, sample: is.Detail}
			byKind[is.Kind] = g
			order = append(order, is.Kind)
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
			g.severity,
			g.kind,
			fmt.Sprintf("%d", g.count),
			components.SanitizeOneLine(g.sample),
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
