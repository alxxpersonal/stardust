package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/service"
)

// docsVault builds a temp vault with "specs" and "plans" collection schemas
// mapped at docs/specs and docs/plans, plus two timestamped sample docs under
// docs/specs carrying title/status frontmatter. It returns the vault root.
func docsVault(t *testing.T) string {
	t.Helper()
	root := emptyVault(t)

	writeCollection := func(name, path string) {
		dir := filepath.Join(root, ".stardust", "collections", name)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		cfg := "path = \"" + path + "\"\n" +
			"description = \"docs " + name + "\"\n\n" +
			"[[fields]]\nname = \"title\"\ntype = \"string\"\nrequired = true\n\n" +
			"[[fields]]\nname = \"status\"\ntype = \"enum\"\nenum = [\"Draft\", \"Approved\"]\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(cfg), 0o644))
	}
	writeCollection("specs", "docs/specs")
	writeCollection("plans", "docs/plans")

	specsDir := filepath.Join(root, "docs", "specs")
	require.NoError(t, os.MkdirAll(specsDir, 0o755))

	older := "---\ntitle: First Spec\nstatus: Approved\n---\n\n# First Spec\nbody\n"
	require.NoError(t, os.WriteFile(filepath.Join(specsDir, "2026-06-20-1000-first-spec.md"), []byte(older), 0o644))
	newer := "---\ntitle: Second Spec\nstatus: Draft\n---\n\n# Second Spec\nbody\n"
	require.NoError(t, os.WriteFile(filepath.Join(specsDir, "2026-06-22-2238-second-spec.md"), []byte(newer), 0o644))

	return root
}

func TestRegistry(t *testing.T) {
	ctx := context.Background()
	root := docsVault(t)
	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	_, err = svc.Index(ctx, "")
	require.NoError(t, err)

	groups, err := svc.Registry([]string{"specs", "plans", "adr", "research"})
	require.NoError(t, err)

	// Four groups, in the requested order, even though adr/research are absent.
	require.Len(t, groups, 4)
	require.Equal(t, "specs", groups[0].Name)
	require.Equal(t, "plans", groups[1].Name)
	require.Equal(t, "adr", groups[2].Name)
	require.Equal(t, "research", groups[3].Name)

	// specs holds both records, newest filename first.
	require.Len(t, groups[0].Records, 2)
	first := groups[0].Records[0]
	require.Equal(t, "Second Spec", first.Title)
	require.Equal(t, "Draft", first.Status)
	require.Equal(t, "docs/specs/2026-06-22-2238-second-spec.md", first.Path)
	require.Equal(t, "2026-06-22", first.Date)

	second := groups[0].Records[1]
	require.Equal(t, "First Spec", second.Title)
	require.Equal(t, "Approved", second.Status)
	require.Equal(t, "docs/specs/2026-06-20-1000-first-spec.md", second.Path)
	require.Equal(t, "2026-06-20", second.Date)

	// plans collection exists but has no docs: empty group, no error.
	require.Empty(t, groups[1].Records)
	// adr and research have no config at all: empty groups, no error.
	require.Empty(t, groups[2].Records)
	require.Empty(t, groups[3].Records)
}

func TestRegistryADRNumberAndDateField(t *testing.T) {
	ctx := context.Background()
	root := emptyVault(t)

	adrDir := filepath.Join(root, ".stardust", "collections", "adr")
	require.NoError(t, os.MkdirAll(adrDir, 0o755))
	adrCfg := "path = \"docs/adr\"\ndescription = \"decisions\"\n\n" +
		"[[fields]]\nname = \"title\"\ntype = \"string\"\nrequired = true\n\n" +
		"[[fields]]\nname = \"status\"\ntype = \"enum\"\nenum = [\"Accepted\"]\n"
	require.NoError(t, os.WriteFile(filepath.Join(adrDir, "config.toml"), []byte(adrCfg), 0o644))

	docDir := filepath.Join(root, "docs", "adr")
	require.NoError(t, os.MkdirAll(docDir, 0o755))
	note := "---\ntitle: Adopt collections\nstatus: Accepted\ndate: 2026-06-15\n---\n\n# Adopt collections\nbody\n"
	require.NoError(t, os.WriteFile(filepath.Join(docDir, "0001-adopt-collections.md"), []byte(note), 0o644))

	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()
	_, err = svc.Index(ctx, "")
	require.NoError(t, err)

	groups, err := svc.Registry([]string{"adr"})
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Len(t, groups[0].Records, 1)

	rec := groups[0].Records[0]
	require.Equal(t, "0001", rec.Number)
	require.Equal(t, "Adopt collections", rec.Title)
	require.Equal(t, "Accepted", rec.Status)
	require.Equal(t, "docs/adr/0001-adopt-collections.md", rec.Path)
	// date frontmatter field wins over the filename prefix (which has none here).
	require.Equal(t, "2026-06-15", rec.Date)
}
