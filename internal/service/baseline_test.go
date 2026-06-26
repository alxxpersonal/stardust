package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/service"
)

func TestFingerprintStableAndDistinct(t *testing.T) {
	base := service.Issue{Severity: "warn", Kind: "orphan", Path: "a.md", Detail: "no links in or out"}

	// stable across calls for the same issue.
	require.Equal(t, service.Fingerprint(base), service.Fingerprint(base))

	// a varying integer in the detail does not churn the fingerprint, so a drift
	// commit count stays a single baselined issue.
	driftA := service.Issue{Severity: "warn", Kind: "drift", Path: "a.md", Detail: "moved 3 commits since last touched"}
	driftB := service.Issue{Severity: "warn", Kind: "drift", Path: "a.md", Detail: "moved 12 commits since last touched"}
	require.Equal(t, service.Fingerprint(driftA), service.Fingerprint(driftB))

	// distinct across kind, path, and textual detail.
	byKind := base
	byKind.Kind = "broken-link"
	require.NotEqual(t, service.Fingerprint(base), service.Fingerprint(byKind))

	byPath := base
	byPath.Path = "b.md"
	require.NotEqual(t, service.Fingerprint(base), service.Fingerprint(byPath))

	byDetail := base
	byDetail.Detail = "an entirely different reason"
	require.NotEqual(t, service.Fingerprint(base), service.Fingerprint(byDetail))
}

func TestCheckCIRatchet(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	write := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(content), 0o644))
	}
	// a backlog the baseline will absorb: an orphan note (warn).
	write("orphan.md", "---\ntitle: Orphan\n---\n# Orphan\nno links here")

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	// snapshot the current backlog.
	snap, err := svc.UpdateBaseline(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, snap.Issues)

	// the committed baseline now covers every current issue: no new issues.
	ci, err := svc.CheckCI(context.Background())
	require.NoError(t, err)
	require.Empty(t, ci.New)
	require.Equal(t, 0, ci.NewErrors)

	// introduce exactly one new error: a forbidden unicode dash in the orphan body.
	write("orphan.md", "---\ntitle: Orphan\n---\n# Orphan\nno links here "+string(rune(0x2014))+" dash")

	ci, err = svc.CheckCI(context.Background())
	require.NoError(t, err)
	require.Len(t, ci.New, 1)
	require.Equal(t, "forbidden-dash", ci.New[0].Kind)
	require.Equal(t, 1, ci.NewErrors)

	// re-snapshotting absorbs the new issue, so the gate is green again.
	_, err = svc.UpdateBaseline(context.Background())
	require.NoError(t, err)
	ci, err = svc.CheckCI(context.Background())
	require.NoError(t, err)
	require.Empty(t, ci.New)
}
