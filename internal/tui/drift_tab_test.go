package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/internal/tui/components"
)

func TestDriftLoadedStoresReports(t *testing.T) {
	tab := newDriftTab(nil)
	updated, _ := tab.Update(driftLoadedMsg{
		check: service.CheckResult{
			Errors:   1,
			Warnings: 2,
			Issues: []service.Issue{
				{Severity: "error", Kind: "broken-link", Path: "a.md", Detail: "missing"},
			},
		},
		drift: service.DriftResult{
			Docs: []service.DriftDoc{
				{DocPath: "docs/specs/a.md", Type: "spec", Bindings: []service.DriftBinding{{File: "internal/a.go", ChangedCommits: 3}}},
			},
			Markdown: "# Drifted Docs",
		},
		stale: service.StaleResult{
			Docs: []service.GovernedDoc{
				{DocPath: "docs/plans/a.md", Type: "plan", ChangedCommits: 2},
			},
			Markdown: "# Stale Docs",
		},
	})
	tab = updated.(driftTab)

	require.True(t, tab.loaded)
	require.Equal(t, 1, tab.check.Errors)
	require.Len(t, tab.drift.Docs, 1)
	require.Len(t, tab.stale.Docs, 1)
}

func TestDriftRendersSingleHeaders(t *testing.T) {
	tab := newDriftTab(nil)
	tab.loaded = true
	tab.drift = service.DriftResult{Docs: []service.DriftDoc{
		{DocPath: "docs/specs/a.md", Title: "Spec A", Type: "spec",
			Bindings: []service.DriftBinding{{File: "internal/a.go", ChangedCommits: 3}}},
	}}
	tab.stale = service.StaleResult{Docs: []service.GovernedDoc{
		{DocPath: "docs/plans/b.md", Title: "Plan B", Type: "plan", Status: "Implemented",
			ChangedCommits: 2, Matched: []string{"internal/b.go"}},
	}}
	out := components.SanitizeText(tab.View(120, 40))
	require.Equal(t, 1, strings.Count(out, "Drifted Docs"))
	require.Equal(t, 1, strings.Count(out, "Stale Docs"))
	require.Contains(t, out, "internal/a.go") // referenced moved file
	require.Contains(t, out, "Spec A")
	require.Contains(t, out, "Plan B")
}

func TestDriftAppRendersCoherenceSummaryOnce(t *testing.T) {
	app := newApp(nil)
	app.width = 140
	app.height = 40
	app.activeTab = TabDrift
	app.driftTab.loaded = true
	app.driftTab.check = service.CheckResult{Errors: 1, Warnings: 2}
	app.driftTab.drift = service.DriftResult{Docs: []service.DriftDoc{
		{DocPath: "docs/specs/a.md", Title: "Spec A", Type: "spec",
			Bindings: []service.DriftBinding{{File: "internal/a.go", ChangedCommits: 3}}},
	}}
	app.driftTab.stale = service.StaleResult{Docs: []service.GovernedDoc{
		{DocPath: "docs/plans/b.md", Title: "Plan B", Type: "plan", Status: "Implemented",
			ChangedCommits: 2, Matched: []string{"internal/b.go"}},
	}}

	out := components.SanitizeText(app.View().Content)
	require.Equal(t, 1, strings.Count(out, "1 errors · 2 warnings"))
}

func TestDriftEmptyStaleStaysCompact(t *testing.T) {
	tab := newDriftTab(nil)
	tab.loaded = true
	// many drifted docs, but no stale docs: the stale section must stay compact.
	for i := 0; i < 8; i++ {
		tab.drift.Docs = append(tab.drift.Docs, service.DriftDoc{
			DocPath: "docs/specs/a.md", Title: "Spec", Type: "spec",
			Bindings: []service.DriftBinding{{File: "internal/a.go", ChangedCommits: 1}},
		})
	}

	out := components.SanitizeText(tab.View(120, 60))
	require.Equal(t, 1, strings.Count(out, "Drifted Docs"))
	require.Equal(t, 1, strings.Count(out, "Stale Docs"))
	require.Contains(t, out, "no stale docs")

	// the empty stale section is its natural height, not padded to a full box:
	// its content is one header + blank + one message line.
	staleLines := strings.Count(components.SanitizeText(tab.staleList(110)), "\n") + 1
	require.LessOrEqual(t, staleLines, 3)
}

func TestFitStackHeightsKeepsSmallSections(t *testing.T) {
	// nothing overflows: every section keeps its natural height.
	got := fitStackHeights([]int{2, 1, 4}, 40, 3)
	require.Equal(t, []int{2, 1, 4}, got)

	// overflow shrinks only the tallest sections, never below minH.
	got = fitStackHeights([]int{30, 1, 20}, 24, 3)
	require.Equal(t, 24, got[0]+got[1]+got[2])
	require.Equal(t, 1, got[1])          // the small section is untouched
	require.GreaterOrEqual(t, got[0], 3) // never shrunk below minH
	require.GreaterOrEqual(t, got[2], 3)
}

func TestDriftCheckSummaryGroups(t *testing.T) {
	tab := newDriftTab(nil)
	tab.loaded = true
	tab.check = service.CheckResult{Errors: 2, Warnings: 1, Issues: []service.Issue{
		{Severity: "error", Kind: "broken-link", Path: "a.md", Detail: "x"},
		{Severity: "error", Kind: "broken-link", Path: "b.md", Detail: "y"},
		{Severity: "warn", Kind: "orphan", Path: "c.md", Detail: "z"},
	}}
	out := components.SanitizeText(tab.checkSummary(110))
	require.Contains(t, out, "broken-link")
	require.Contains(t, out, "orphan")
	require.NotContains(t, out, "|")
}

func TestDriftCheckSummaryWrapsFullFindingMessage(t *testing.T) {
	longDetail := `note name "skills/skill" is shared by 8 files (docs/skills/brand-check/SKILL.md, docs/skills/component-search/SKILL.md, docs/skills/doc-add-adr/SKILL.md, docs/skills/doc-add-plan/SKILL.md, docs/skills/doc-check/SKILL.md, docs/skills/doc-governance/SKILL.md, docs/skills/doc-link/SKILL.md, docs/skills/doc-status/SKILL.md); wikilinks to it are ambiguous`
	tab := newDriftTab(nil)
	tab.loaded = true
	tab.check = service.CheckResult{Warnings: 1, Issues: []service.Issue{
		{Severity: "warn", Kind: "duplicate-name", Path: "docs/skills/brand-check/SKILL.md", Detail: longDetail},
	}}

	out := components.SanitizeText(tab.checkSummary(88))
	require.Contains(t, strings.Join(strings.Fields(out), " "), longDetail)
	require.Contains(t, out, "duplicate-name")
	require.Contains(t, out, "warn")
	require.Contains(t, out, "1")
	require.NotContains(t, out, "…")
	require.NotContains(t, out, "...")
}
