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

func TestDriftCheckSummaryGroups(t *testing.T) {
	tab := newDriftTab(nil)
	tab.loaded = true
	tab.check = service.CheckResult{Errors: 2, Warnings: 1, Issues: []service.Issue{
		{Severity: "error", Kind: "broken-link", Path: "a.md", Detail: "x"},
		{Severity: "error", Kind: "broken-link", Path: "b.md", Detail: "y"},
		{Severity: "warn", Kind: "orphan", Path: "c.md", Detail: "z"},
	}}
	out := components.SanitizeText(tab.checkSummary(110, 20))
	require.Contains(t, out, "broken-link")
	require.Contains(t, out, "orphan")
	require.NotContains(t, out, "|")
}
