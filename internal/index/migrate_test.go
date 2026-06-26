package index

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// hasColumn reports whether table has a column named col.
func hasColumn(t *testing.T, st *Store, table, col string) bool {
	t.Helper()
	rows, err := st.db.QueryContext(context.Background(), "PRAGMA table_info("+table+")")
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt any
		require.NoError(t, rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk))
		if name == col {
			return true
		}
	}
	require.NoError(t, rows.Err())
	return false
}

// TestChunkHashMigration asserts the chunk_hash column is added and that running
// migrations twice (reopening the same db) is idempotent.
func TestChunkHashMigration(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "db.sqlite")

	st, err := Open(ctx, dbPath)
	require.NoError(t, err)
	require.True(t, hasColumn(t, st, "chunks", "chunk_hash"))
	require.NoError(t, st.Close())

	// reopen: migrations re-run as a no-op, the column persists.
	st2, err := Open(ctx, dbPath)
	require.NoError(t, err)
	require.True(t, hasColumn(t, st2, "chunks", "chunk_hash"))
	require.NoError(t, st2.Close())
}
