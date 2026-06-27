package index

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/vault"
)

// openStore opens a fresh on-disk index in a temp dir (modernc sqlite needs a
// file path for the bundled JSON1 + FTS5 to behave like production).
func openStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(context.Background(), filepath.Join(t.TempDir(), "db.sqlite"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// chunk is a tiny helper to build a one-chunk note body for a path.
func chunk(path, title, body string) []vault.Chunk {
	return []vault.Chunk{{NotePath: path, Title: title, Body: body, Ord: 0}}
}

func TestUpsertPersistsFrontmatter(t *testing.T) {
	st := openStore(t)
	ctx := context.Background()
	fm := map[string]any{"company": "acme", "status": "open", "score": float64(7)}
	require.NoError(t, st.UpsertNote(ctx, "jobs/acme.md", "h1", chunk("jobs/acme.md", "Acme", "body"), nil, fm))

	rows, err := st.ListRecords(ctx, "jobs", nil, "path", 0, 0)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "jobs/acme.md", rows[0].Path)
	require.Equal(t, "Acme", rows[0].Title)
	require.Equal(t, "acme", rows[0].Frontmatter["company"])
	require.Equal(t, "open", rows[0].Frontmatter["status"])
	require.NotEmpty(t, rows[0].UpdatedAt)
}

func TestUpsertNilFrontmatterDefaultsEmpty(t *testing.T) {
	st := openStore(t)
	ctx := context.Background()
	require.NoError(t, st.UpsertNote(ctx, "jobs/bare.md", "h0", chunk("jobs/bare.md", "Bare", "body"), nil, nil))

	rows, err := st.ListRecords(ctx, "jobs", nil, "path", 0, 0)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Empty(t, rows[0].Frontmatter) // decoded from the "{}" default
}

func TestListRecordsScopesToFolder(t *testing.T) {
	st := openStore(t)
	ctx := context.Background()
	require.NoError(t, st.UpsertNote(ctx, "jobs/a.md", "h", chunk("jobs/a.md", "A", "x"), nil, map[string]any{"status": "open"}))
	require.NoError(t, st.UpsertNote(ctx, "jobs/sub/b.md", "h", chunk("jobs/sub/b.md", "B", "x"), nil, map[string]any{"status": "open"}))
	require.NoError(t, st.UpsertNote(ctx, "people/c.md", "h", chunk("people/c.md", "C", "x"), nil, map[string]any{"status": "open"}))

	rows, err := st.ListRecords(ctx, "jobs", nil, "path", 0, 0)
	require.NoError(t, err)
	require.Len(t, rows, 2) // jobs/a.md and jobs/sub/b.md, not people/c.md
	require.Equal(t, "jobs/a.md", rows[0].Path)
	require.Equal(t, "jobs/sub/b.md", rows[1].Path)
}

func TestListRecordsFilterAndSort(t *testing.T) {
	st := openStore(t)
	ctx := context.Background()
	mk := func(path, status string, score float64) {
		fm := map[string]any{"status": status, "score": score, "company": path}
		require.NoError(t, st.UpsertNote(ctx, path, "h", chunk(path, path, "x"), nil, fm))
	}
	mk("jobs/a.md", "open", 3)
	mk("jobs/b.md", "open", 9)
	mk("jobs/c.md", "closed", 5)

	// filter status == open, sort by score descending.
	rows, err := st.ListRecords(ctx, "jobs",
		[]Predicate{{Field: "status", Op: "eq", Value: "open"}}, "-score", 0, 0)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, "jobs/b.md", rows[0].Path) // score 9 first
	require.Equal(t, "jobs/a.md", rows[1].Path) // score 3 second

	// numeric gt predicate via json_extract.
	rows, err = st.ListRecords(ctx, "jobs",
		[]Predicate{{Field: "score", Op: "gte", Value: "5"}}, "score", 0, 0)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, "jobs/c.md", rows[0].Path) // 5
	require.Equal(t, "jobs/b.md", rows[1].Path) // 9

	// contains predicate.
	rows, err = st.ListRecords(ctx, "jobs",
		[]Predicate{{Field: "company", Op: "contains", Value: "/a."}}, "path", 0, 0)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "jobs/a.md", rows[0].Path)
}

func TestListRecordsLimitOffset(t *testing.T) {
	st := openStore(t)
	ctx := context.Background()
	for _, p := range []string{"jobs/a.md", "jobs/b.md", "jobs/c.md", "jobs/d.md"} {
		require.NoError(t, st.UpsertNote(ctx, p, "h", chunk(p, p, "x"), nil, map[string]any{"k": "v"}))
	}
	rows, err := st.ListRecords(ctx, "jobs", nil, "path", 2, 1)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, "jobs/b.md", rows[0].Path) // offset 1 skips a.md
	require.Equal(t, "jobs/c.md", rows[1].Path)
}

func TestListRecordsPathDescNewestFirst(t *testing.T) {
	st := openStore(t)
	ctx := context.Background()
	// date-prefixed filenames, inserted out of order on purpose.
	for _, p := range []string{
		"docs/specs/2026-01-01-old.md",
		"docs/specs/2026-06-25-new.md",
		"docs/specs/2026-03-15-mid.md",
	} {
		require.NoError(t, st.UpsertNote(ctx, p, "h", chunk(p, p, "x"), nil, nil))
	}

	rows, err := st.ListRecords(ctx, "docs/specs", nil, "-path", 0, 0)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	// descending path order surfaces the newest date-prefixed file first.
	require.Equal(t, "docs/specs/2026-06-25-new.md", rows[0].Path)
	require.Equal(t, "docs/specs/2026-03-15-mid.md", rows[1].Path)
	require.Equal(t, "docs/specs/2026-01-01-old.md", rows[2].Path)
}

func TestListRecordsRejectsBadOp(t *testing.T) {
	st := openStore(t)
	_, err := st.ListRecords(context.Background(), "jobs",
		[]Predicate{{Field: "status", Op: "regex", Value: "x"}}, "", 0, 0)
	require.Error(t, err)
}
