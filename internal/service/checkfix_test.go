package service_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/internal/vault"
)

func fixVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "specs"), 0o755))
	return root
}

func TestCheckFixReplacesForbiddenDashes(t *testing.T) {
	root := fixVault(t)
	rel := filepath.Join("docs", "specs", "2026-06-22-1000-dashy.md")
	body := "---\ntitle: Dashy\ntype: spec\nstatus: Draft\ncreated: 2026-06-22\nupdated: 2026-06-22\n---\n# Dashy\n\none \u2014 two \u2013 three \u2014 four\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644))

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.CheckFix(context.Background())
	require.NoError(t, err)
	require.Greater(t, res.Fixed, 0)

	fixed, err := os.ReadFile(filepath.Join(root, rel))
	require.NoError(t, err)
	require.NotContains(t, string(fixed), "\u2014")
	require.NotContains(t, string(fixed), "\u2013")
	require.Contains(t, string(fixed), "one - two - three - four")

	// A re-check should no longer report a forbidden-dash error.
	check, err := svc.Check(context.Background())
	require.NoError(t, err)
	require.False(t, hasCheckIssue(check.Issues, "forbidden-dash"))
}

func TestCheckFixRewritesBadDocType(t *testing.T) {
	root := fixVault(t)
	rel := filepath.Join("docs", "specs", "2026-06-22-1000-wrong-type.md")
	body := "---\ntitle: Wrong\ntype: plan\nstatus: Draft\ncreated: 2026-06-22\nupdated: 2026-06-22\n---\n# Wrong\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644))

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.CheckFix(context.Background())
	require.NoError(t, err)
	require.Greater(t, res.Fixed, 0)

	note, err := vault.Parse(root, filepath.ToSlash(rel))
	require.NoError(t, err)
	require.Equal(t, "spec", note.Frontmatter["type"])

	check, err := svc.Check(context.Background())
	require.NoError(t, err)
	require.False(t, hasCheckIssue(check.Issues, "bad-doc-type"))
}

func TestCheckFixFillsMissingFields(t *testing.T) {
	root := fixVault(t)
	rel := filepath.Join("docs", "specs", "2026-06-22-1000-missing.md")
	// Missing type, created, updated. Title and status present so they are not the gap.
	body := "---\ntitle: Missing\nstatus: Draft\n---\n# Missing\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644))

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.CheckFix(context.Background())
	require.NoError(t, err)
	require.Greater(t, res.Fixed, 0)

	note, err := vault.Parse(root, filepath.ToSlash(rel))
	require.NoError(t, err)
	require.Equal(t, "spec", note.Frontmatter["type"])
	require.NotEmpty(t, note.Frontmatter["created"])
	require.NotEmpty(t, note.Frontmatter["updated"])

	check, err := svc.Check(context.Background())
	require.NoError(t, err)
	for _, is := range check.Issues {
		if is.Kind == "missing-doc-field" && is.Path == filepath.ToSlash(rel) {
			require.Contains(t, []string{"title", "status"}, strings.Fields(is.Detail)[0])
		}
	}
}

func TestCheckFixLeavesAmbiguousIssuesAlone(t *testing.T) {
	root := fixVault(t)
	// bad-doc-status is a judgment call: must not be touched.
	rel := filepath.Join("docs", "specs", "2026-06-22-1000-weird-status.md")
	body := "---\ntitle: Weird\ntype: spec\nstatus: Weird\ncreated: 2026-06-22\nupdated: 2026-06-22\n---\n# Weird\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644))

	svc, err := service.Open(context.Background(), root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	res, err := svc.CheckFix(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, res.Fixed)

	note, err := vault.Parse(root, filepath.ToSlash(rel))
	require.NoError(t, err)
	require.Equal(t, "Weird", note.Frontmatter["status"])

	check, err := svc.Check(context.Background())
	require.NoError(t, err)
	require.True(t, hasCheckIssue(check.Issues, "bad-doc-status"))
}
