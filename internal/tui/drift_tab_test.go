package tui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/service"
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
