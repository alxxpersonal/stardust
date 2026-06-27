package collections

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeCollection(t *testing.T, dir, name, toml string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, name), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name, "config.toml"), []byte(toml), 0o644))
}

func TestLoadParsesSchema(t *testing.T) {
	dir := t.TempDir()
	writeCollection(t, dir, "jobs", `
path = "jobs"
description = "job applications"

[[fields]]
name = "company"
type = "string"
required = true

[[fields]]
name = "status"
type = "enum"
enum = ["open", "closed"]

[[fields]]
name = "score"
type = "number"
`)
	cols, err := Load(dir)
	require.NoError(t, err)
	require.Len(t, cols, 1)
	require.Equal(t, "jobs", cols[0].Name)
	require.Equal(t, "jobs", cols[0].Cfg.Path)
	require.Len(t, cols[0].Cfg.Fields, 3)
	require.Equal(t, "company", cols[0].Cfg.Fields[0].Name)
	require.True(t, cols[0].Cfg.Fields[0].Required)
	require.Equal(t, []string{"open", "closed"}, cols[0].Cfg.Fields[1].Enum)
}

func TestLoadMissingDir(t *testing.T) {
	cols, err := Load(filepath.Join(t.TempDir(), "nope"))
	require.NoError(t, err)
	require.Nil(t, cols)
}

func TestLoadRequiresPath(t *testing.T) {
	dir := t.TempDir()
	writeCollection(t, dir, "broken", "description = \"no path\"\n")
	_, err := Load(dir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "path is required")
}

func TestLoadRejectsBadType(t *testing.T) {
	dir := t.TempDir()
	writeCollection(t, dir, "bad", "path = \"x\"\n\n[[fields]]\nname = \"f\"\ntype = \"wat\"\n")
	_, err := Load(dir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported type")
}

func TestLoadSortsByName(t *testing.T) {
	dir := t.TempDir()
	writeCollection(t, dir, "zeta", "path = \"zeta\"\n")
	writeCollection(t, dir, "alpha", "path = \"alpha\"\n")
	cols, err := Load(dir)
	require.NoError(t, err)
	require.Len(t, cols, 2)
	require.Equal(t, "alpha", cols[0].Name)
	require.Equal(t, "zeta", cols[1].Name)
}

func TestValidateRequired(t *testing.T) {
	fields := []Field{{Name: "company", Type: TypeString, Required: true}}
	require.Error(t, Validate(map[string]any{}, fields))                    // missing
	require.Error(t, Validate(map[string]any{"company": nil}, fields))      // nil present
	require.NoError(t, Validate(map[string]any{"company": "acme"}, fields)) // ok
	require.Error(t, Validate(map[string]any{"company": 7}, fields))        // wrong type
}

func TestValidateEnum(t *testing.T) {
	fields := []Field{{Name: "status", Type: TypeEnum, Enum: []string{"open", "closed"}}}
	require.NoError(t, Validate(map[string]any{"status": "open"}, fields))
	require.Error(t, Validate(map[string]any{"status": "weird"}, fields))
}

func TestValidateTypes(t *testing.T) {
	require.NoError(t, Validate(map[string]any{"n": float64(3)}, []Field{{Name: "n", Type: TypeNumber}}))
	require.NoError(t, Validate(map[string]any{"n": 3}, []Field{{Name: "n", Type: TypeNumber}}))
	require.Error(t, Validate(map[string]any{"n": "three"}, []Field{{Name: "n", Type: TypeNumber}}))

	require.NoError(t, Validate(map[string]any{"b": true}, []Field{{Name: "b", Type: TypeBool}}))
	require.Error(t, Validate(map[string]any{"b": "yes"}, []Field{{Name: "b", Type: TypeBool}}))

	require.NoError(t, Validate(map[string]any{"d": "2026-06-12"}, []Field{{Name: "d", Type: TypeDate}}))
	require.Error(t, Validate(map[string]any{"d": "not-a-date"}, []Field{{Name: "d", Type: TypeDate}}))

	require.NoError(t, Validate(map[string]any{"t": []any{"a", "b"}}, []Field{{Name: "t", Type: TypeTags}}))
	require.Error(t, Validate(map[string]any{"t": []any{"a", 2}}, []Field{{Name: "t", Type: TypeTags}}))
}

func TestValidateRefAcceptsStringOrList(t *testing.T) {
	fields := []Field{{Name: "related", Type: TypeRef}}
	require.NoError(t, Validate(map[string]any{"related": "docs/x.md"}, fields))                     // single ref
	require.NoError(t, Validate(map[string]any{"related": []any{"docs/x.md", "docs/y.md"}}, fields)) // list of refs
	require.NoError(t, Validate(map[string]any{"related": []string{"docs/x.md"}}, fields))           // []string list
	require.Error(t, Validate(map[string]any{"related": []any{"docs/x.md", 7}}, fields))             // non-string element
	require.Error(t, Validate(map[string]any{"related": 7}, fields))                                 // not a ref at all
}

func TestValidateIgnoresUnknownFields(t *testing.T) {
	// extra frontmatter keys not in the schema are allowed.
	fields := []Field{{Name: "company", Type: TypeString, Required: true}}
	require.NoError(t, Validate(map[string]any{"company": "acme", "extra": 123}, fields))
}

func TestSaveRoundTripsCollection(t *testing.T) {
	collectionsDir := t.TempDir()
	want := Collection{
		Name: "jobs",
		Cfg: Config{
			Path:        "docs/jobs",
			Description: "job applications",
			Fields: []Field{
				{Name: "company", Type: TypeString, Required: true},
				{Name: "status", Type: TypeEnum, Enum: []string{"open", "closed"}},
				{Name: "score", Type: TypeNumber},
				{Name: "tags", Type: TypeTags},
			},
		},
	}

	require.NoError(t, Save(collectionsDir, want))

	got, err := LoadOne(collectionsDir, "jobs")
	require.NoError(t, err)
	require.Equal(t, want, got)

	all, err := Load(collectionsDir)
	require.NoError(t, err)
	require.Equal(t, []Collection{want}, all)

	data, err := os.ReadFile(filepath.Join(collectionsDir, "jobs", "config.toml"))
	require.NoError(t, err)
	require.Contains(t, string(data), "[[fields]]")
}

func TestSaveRejectsUnsafeNameAndEmptyPath(t *testing.T) {
	collectionsDir := t.TempDir()

	require.Error(t, Save(collectionsDir, Collection{
		Name: "../jobs",
		Cfg:  Config{Path: "docs/jobs"},
	}))
	require.Error(t, Save(collectionsDir, Collection{
		Name: "jobs",
		Cfg:  Config{},
	}))
}

func TestRemoveDeletesConfigButLeavesDocs(t *testing.T) {
	root := t.TempDir()
	collectionsDir := filepath.Join(root, ".stardust", "collections")
	docPath := filepath.Join(root, "docs", "jobs", "acme.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(docPath), 0o755))
	require.NoError(t, os.WriteFile(docPath, []byte("# acme\n"), 0o644))

	require.NoError(t, Save(collectionsDir, Collection{
		Name: "jobs",
		Cfg: Config{
			Path:        "docs/jobs",
			Description: "job applications",
		},
	}))

	require.NoError(t, Remove(collectionsDir, "jobs"))

	_, err := os.Stat(filepath.Join(collectionsDir, "jobs"))
	require.True(t, os.IsNotExist(err))
	_, err = os.Stat(docPath)
	require.NoError(t, err)
}
