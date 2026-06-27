package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/collections"
	"github.com/alxxpersonal/stardust/internal/service"
)

// jobsVault builds a temp vault with a "jobs" collection schema
// (company/status/score) and returns its root.
func jobsVault(t *testing.T) string {
	t.Helper()
	root := emptyVault(t)
	schemaDir := filepath.Join(root, ".stardust", "collections", "jobs")
	require.NoError(t, os.MkdirAll(schemaDir, 0o755))
	schema := `
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
`
	require.NoError(t, os.WriteFile(filepath.Join(schemaDir, "config.toml"), []byte(schema), 0o644))
	return root
}

func TestRecordsRoundTrip(t *testing.T) {
	ctx := context.Background()
	root := jobsVault(t)
	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	// create two records
	r1, err := svc.CreateRecord(ctx, "jobs",
		map[string]any{"company": "Acme", "status": "open", "score": float64(8)}, "first body")
	require.NoError(t, err)
	require.Equal(t, "jobs/acme.md", r1.Path)
	require.Equal(t, "open", r1.Frontmatter["status"])
	require.Contains(t, r1.Body, "first body")

	r2, err := svc.CreateRecord(ctx, "jobs",
		map[string]any{"company": "Globex", "status": "closed", "score": float64(3)}, "second body")
	require.NoError(t, err)
	require.Equal(t, "jobs/globex.md", r2.Path)

	// both files exist on disk
	for _, p := range []string{r1.Path, r2.Path} {
		_, statErr := os.Stat(filepath.Join(root, p))
		require.NoError(t, statErr)
	}

	// ListCollections reports the schema and a record count of 2
	cols, err := svc.ListCollections(ctx)
	require.NoError(t, err)
	require.Len(t, cols, 1)
	require.Equal(t, "jobs", cols[0].Name)
	require.Equal(t, 2, cols[0].Records)
	require.Len(t, cols[0].Fields, 3)

	got, err := svc.GetCollection(ctx, "jobs")
	require.NoError(t, err)
	require.Equal(t, 2, got.Records)

	// ListRecords: filter status == open, sort -score
	list, err := svc.ListRecords(ctx, "jobs",
		[]service.Predicate{{Field: "status", Op: "eq", Value: "open"}}, "-score", 0, 0)
	require.NoError(t, err)
	require.Equal(t, "jobs", list.Folder)
	require.Len(t, list.Records, 1)
	require.Equal(t, "jobs/acme.md", list.Records[0].Path)

	// add a second open record with a higher score to prove the sort order
	r3, err := svc.CreateRecord(ctx, "jobs",
		map[string]any{"company": "Initech", "status": "open", "score": float64(10)}, "third")
	require.NoError(t, err)
	list, err = svc.ListRecords(ctx, "jobs",
		[]service.Predicate{{Field: "status", Op: "eq", Value: "open"}}, "-score", 0, 0)
	require.NoError(t, err)
	require.Len(t, list.Records, 2)
	require.Equal(t, r3.Path, list.Records[0].Path)        // score 10 first
	require.Equal(t, "jobs/acme.md", list.Records[1].Path) // score 8 second

	// PatchRecord: flip Acme's status to closed
	patched, err := svc.PatchRecord(ctx, "jobs/acme.md",
		map[string]any{"status": "closed"}, nil)
	require.NoError(t, err)
	require.Equal(t, "closed", patched.Frontmatter["status"])
	require.Equal(t, "Acme", patched.Frontmatter["company"]) // untouched field preserved
	require.Contains(t, patched.Body, "first body")          // body preserved on patch

	// the filter now excludes acme.md
	list, err = svc.ListRecords(ctx, "jobs",
		[]service.Predicate{{Field: "status", Op: "eq", Value: "open"}}, "-score", 0, 0)
	require.NoError(t, err)
	require.Len(t, list.Records, 1)
	require.Equal(t, r3.Path, list.Records[0].Path)

	// GetRecord includes frontmatter + body
	rec, err := svc.GetRecord(ctx, "jobs/globex.md")
	require.NoError(t, err)
	require.Equal(t, "Globex", rec.Frontmatter["company"])
	require.Contains(t, rec.Body, "second body")

	// ArchiveRecord removes globex and prunes the index. Acme + Initech remain.
	require.NoError(t, svc.ArchiveRecord(ctx, "jobs/globex.md"))
	_, statErr := os.Stat(filepath.Join(root, "jobs/globex.md"))
	require.True(t, os.IsNotExist(statErr))
	cols, err = svc.ListCollections(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, cols[0].Records) // acme + initech left after archiving globex
}

func TestCreateRecordValidationFails(t *testing.T) {
	ctx := context.Background()
	svc, err := service.Open(ctx, jobsVault(t))
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	// missing required "company"
	_, err = svc.CreateRecord(ctx, "jobs", map[string]any{"status": "open"}, "x")
	require.Error(t, err)

	// invalid enum value
	_, err = svc.CreateRecord(ctx, "jobs",
		map[string]any{"company": "Acme", "status": "weird"}, "x")
	require.Error(t, err)
}

func TestCreateRecordUniqueFilenames(t *testing.T) {
	ctx := context.Background()
	svc, err := service.Open(ctx, jobsVault(t))
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	r1, err := svc.CreateRecord(ctx, "jobs", map[string]any{"company": "Acme", "status": "open"}, "a")
	require.NoError(t, err)
	require.Equal(t, "jobs/acme.md", r1.Path)

	r2, err := svc.CreateRecord(ctx, "jobs", map[string]any{"company": "Acme", "status": "closed"}, "b")
	require.NoError(t, err)
	require.Equal(t, "jobs/acme-2.md", r2.Path) // collision-suffixed
}

func TestCollectionMutatorsPersistAndReflectThroughList(t *testing.T) {
	ctx := context.Background()
	root := emptyVault(t)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "research"), 0o755))
	docPath := filepath.Join(root, "docs", "research", "note.md")
	require.NoError(t, os.WriteFile(docPath, []byte("# note\n"), 0o644))

	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	info := service.CollectionInfo{
		Name:        "research",
		Path:        "docs/research",
		Description: "research notes",
		Fields: []collections.Field{
			{Name: "status", Type: collections.TypeEnum, Required: true, Enum: []string{"draft", "done"}},
		},
	}
	require.NoError(t, svc.SaveCollection(ctx, info))

	cols, err := svc.ListCollections(ctx)
	require.NoError(t, err)
	require.Len(t, cols, 1)
	require.Equal(t, "research", cols[0].Name)
	require.Equal(t, "docs/research", cols[0].Path)
	require.Equal(t, "research notes", cols[0].Description)
	require.Equal(t, info.Fields, cols[0].Fields)

	info.Description = "edited research notes"
	info.Fields = append(info.Fields, collections.Field{Name: "owner", Type: collections.TypeString})
	require.NoError(t, svc.SaveCollection(ctx, info))

	got, err := svc.GetCollection(ctx, "research")
	require.NoError(t, err)
	require.Equal(t, "edited research notes", got.Description)
	require.Len(t, got.Fields, 2)

	require.NoError(t, svc.DeleteCollection(ctx, "research"))
	cols, err = svc.ListCollections(ctx)
	require.NoError(t, err)
	require.Empty(t, cols)
	_, err = os.Stat(docPath)
	require.NoError(t, err)

	require.Error(t, svc.DeleteCollection(ctx, "research"))
	require.Error(t, svc.SaveCollection(ctx, service.CollectionInfo{Name: "../bad", Path: "docs/bad"}))
}

func TestNoteGetIncludesFrontmatter(t *testing.T) {
	ctx := context.Background()
	root := emptyVault(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "n.md"),
		[]byte("---\ntitle: N\ncompany: acme\n---\n# N\nbody"), 0o644))
	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	n, err := svc.GetNote(ctx, "n.md")
	require.NoError(t, err)
	require.Equal(t, "acme", n.Frontmatter["company"])
}
