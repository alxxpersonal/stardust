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

// View renders the coherence report as a clean full-width vertical stack:
// DRIFTED DOCS, STALE DOCS, and CHECK FINDINGS, each a single-header clean-list
// box. Each box renders the typed result slices (not the service Markdown) and
// carries its own keycap header, so the box wrappers pass an empty title and
// exactly one header shows per section. Boxes are sized to their content (an
// empty list stays one line, never a full-height box).
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
	drift := t.driftList(cardW)
	stale := t.staleList(cardW)
	check := t.checkSummary(cardW)

	avail := height - 6
	if avail < 12 {
		avail = 12
	}
	heights := fitStackHeights(
		[]int{countViewLines(drift), countViewLines(stale), countViewLines(check)},
		avail, 3)

	var b strings.Builder
	b.WriteString(animatedDoubleBox("", clipLines(drift, heights[0]), t.frame))
	b.WriteString("\n\n")
	b.WriteString(animatedDoubleBox("", clipLines(stale, heights[1]), t.frame))
	b.WriteString("\n\n")
	b.WriteString(animatedRoundedBox("", clipLines(check, heights[2]), t.frame))
	return centerBlockUniform(b.String(), width)
}

// fitStackHeights returns a per-section line budget that sizes each section to
// its natural height, shrinking only the sections that overflow the available
// space (never below minH) so short or empty sections keep their content height
// and never claim a full box.
func fitStackHeights(natural []int, avail, minH int) []int {
	out := make([]int, len(natural))
	total := 0
	for i, n := range natural {
		if n < 1 {
			n = 1
		}
		out[i] = n
		total += n
	}
	if avail <= 0 {
		return out
	}
	for total > avail {
		tallest := -1
		for i := range out {
			if out[i] > minH && (tallest == -1 || out[i] > out[tallest]) {
				tallest = i
			}
		}
		if tallest == -1 {
			break // every section already at minH; cannot shrink further
		}
		out[tallest]--
		total--
	}
	return out
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

// driftList renders drifted docs, one row per moved binding, in fitted columns.
// It always emits exactly one keycap header (even when empty) so the box wrapper
// can pass an empty title and no duplicate header appears.
func (t DriftTab) driftList(width int) string {
	if len(t.drift.Docs) == 0 {
		body := renderCleanListHeader("Drifted Docs", "", width) + "\n\n" + SuccessStyle.Render("no drifted docs")
		return lipgloss.NewStyle().Width(width).Render(body)
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
	return renderCleanList("Drifted Docs", label, cols, rows, width, -1)
}

// staleList renders implemented docs whose governed code changed after the doc.
// Like driftList it always emits a single keycap header so the empty state stays
// a one-line section, not a full-height box.
func (t DriftTab) staleList(width int) string {
	if len(t.stale.Docs) == 0 {
		body := renderCleanListHeader("Stale Docs", "", width) + "\n\n" + SuccessStyle.Render("no stale docs")
		return lipgloss.NewStyle().Width(width).Render(body)
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
	return renderCleanList("Stale Docs", label, cols, rows, width, -1)
}

// checkSummary renders check findings with wrapped messages. It groups exact
// severity, kind, and message matches for a compact count while preserving every
// distinct full message.
func (t DriftTab) checkSummary(width int) string {
	if len(t.check.Issues) == 0 {
		return lipgloss.NewStyle().Width(width).Render(renderCleanListHeader("Check Findings", "", width) + "\n\n" + SuccessStyle.Render("clean"))
	}
	type group struct {
		severity string
		kind     string
		detail   string
		count    int
	}
	order := make([]string, 0, len(t.check.Issues))
	byFinding := map[string]*group{}
	for _, is := range t.check.Issues {
		severity := components.SanitizeOneLine(is.Severity)
		kind := components.SanitizeOneLine(is.Kind)
		detail := components.SanitizeOneLine(is.Detail)
		key := severity + "\x00" + kind + "\x00" + detail
		g, ok := byFinding[key]
		if !ok {
			g = &group{severity: severity, kind: kind, detail: detail}
			byFinding[key] = g
			order = append(order, key)
		}
		g.count++
	}
	sort.SliceStable(order, func(i, j int) bool {
		gi, gj := byFinding[order[i]], byFinding[order[j]]
		if isErrorSeverity(gi.severity) != isErrorSeverity(gj.severity) {
			return isErrorSeverity(gi.severity)
		}
		if gi.kind != gj.kind {
			return gi.kind < gj.kind
		}
		return gi.detail < gj.detail
	})

	var body strings.Builder
	for _, k := range order {
		g := byFinding[k]
		if body.Len() > 0 {
			body.WriteString("\n\n")
		}
		body.WriteString(checkFindingPrefix(g.severity, g.kind, g.count))
		body.WriteString("\n")
		body.WriteString(renderWrappedCheckDetail(g.detail, width, 2))
	}

	label := fmt.Sprintf("%d error / %d warn", t.check.Errors, t.check.Warnings)
	return lipgloss.NewStyle().Width(width).Render(renderCleanListHeader("Check Findings", label, width) + "\n\n" + body.String())
}

func isErrorSeverity(severity string) bool {
	return strings.EqualFold(strings.TrimSpace(severity), "error")
}

func checkFindingPrefix(severity, kind string, count int) string {
	sev := components.SanitizeOneLine(severity)
	if sev == "" {
		sev = "warn"
	}
	return severityStyle(sev).Render(sev) + "  " +
		MutedStyle.Render(components.SanitizeOneLine(kind)) + "  " +
		countValueStyle(fmt.Sprintf("%d", count)).Render(fmt.Sprintf("%d", count))
}

func renderWrappedCheckDetail(detail string, width int, indent int) string {
	if indent < 0 {
		indent = 0
	}
	available := width - indent
	if available < 8 {
		available = 8
	}
	prefix := strings.Repeat(" ", indent)
	lines := wrapDisplayWords(components.SanitizeOneLine(detail), available)
	for i := range lines {
		lines[i] = prefix + NormalStyle.Render(lines[i])
	}
	return strings.Join(lines, "\n")
}

func wrapDisplayWords(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	lines := []string{}
	current := ""
	for _, word := range words {
		if current == "" {
			lines = appendLongWord(lines, word, width, &current)
			continue
		}
		next := current + " " + word
		if lipgloss.Width(next) <= width {
			current = next
			continue
		}
		lines = append(lines, current)
		current = ""
		lines = appendLongWord(lines, word, width, &current)
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func appendLongWord(lines []string, word string, width int, current *string) []string {
	if lipgloss.Width(word) <= width {
		*current = word
		return lines
	}
	var b strings.Builder
	for _, r := range word {
		next := b.String() + string(r)
		if lipgloss.Width(next) > width && b.Len() > 0 {
			lines = append(lines, b.String())
			b.Reset()
		}
		b.WriteRune(r)
	}
	*current = b.String()
	return lines
}
