package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/service"
)

func TestGoverningDocsFindsSpecsAndPlans(t *testing.T) {
	ctx := context.Background()
	root := governsVault(t)
	writeGovernedCode(t, root, "internal/service/check.go")
	writeGovernedDoc(t, root, "docs/specs/2026-06-22-1000-check-spec.md", "Check Spec", "spec", "Approved", "internal/service/*.go")
	writeGovernedDoc(t, root, "docs/plans/2026-06-22-1100-check-plan.md", "Check Plan", "plan", "Active", "internal/service/*.go")
	writeGovernedDoc(t, root, "docs/specs/2026-06-22-1200-other-spec.md", "Other Spec", "spec", "Approved", "")

	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(ctx, "")
	require.NoError(t, err)

	res, err := svc.GoverningDocs(ctx, "internal/service/check.go")
	require.NoError(t, err)
	require.Equal(t, "internal/service/check.go", res.Path)
	require.Len(t, res.Docs, 2)
	require.Equal(t, "spec", res.Docs[0].Type)
	require.Equal(t, "Check Spec", res.Docs[0].Title)
	require.Equal(t, "plan", res.Docs[1].Type)
	require.Equal(t, "Check Plan", res.Docs[1].Title)
	require.Equal(t, []string{"internal/service/check.go"}, res.Docs[0].Matched)
	require.Contains(t, res.Markdown, "Check Spec")
	require.NotContains(t, res.Markdown, "Other Spec")
}

func TestGoverningDocsMissingCollectionsIsEmpty(t *testing.T) {
	ctx := context.Background()
	root := emptyVault(t)
	writeGovernedCode(t, root, "internal/service/check.go")

	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(ctx, "")
	require.NoError(t, err)

	res, err := svc.GoverningDocs(ctx, "internal/service/check.go")
	require.NoError(t, err)
	require.Empty(t, res.Docs)
	require.Contains(t, res.Markdown, "No governing docs found")
}

func governsVault(t *testing.T) string {
	t.Helper()
	root := emptyVault(t)
	writeGovernCollection(t, root, "specs", "docs/specs")
	writeGovernCollection(t, root, "plans", "docs/plans")
	return root
}

func writeGovernCollection(t *testing.T, root, name, path string) {
	t.Helper()
	dir := filepath.Join(root, ".stardust", "collections", name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	cfg := "path = \"" + path + "\"\n" +
		"description = \"docs " + name + "\"\n\n" +
		"[[fields]]\nname = \"title\"\ntype = \"string\"\nrequired = true\n\n" +
		"[[fields]]\nname = \"status\"\ntype = \"string\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(cfg), 0o644))
}

func writeGovernedCode(t *testing.T, root, rel string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("package service\n"), 0o644))
}

func writeGovernedDoc(t *testing.T, root, rel, title, typ, status, governs string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	body := "---\n" +
		"title: " + title + "\n" +
		"type: " + typ + "\n" +
		"status: " + status + "\n"
	if governs != "" {
		body += "governs: [\"" + governs + "\"]\n"
	}
	body += "---\n# " + title + "\n"
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
}
