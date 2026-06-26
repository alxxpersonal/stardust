package index

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/alxxpersonal/stardust/internal/vault"
)

// --- Catalog ---

// Catalog returns the path -> content-hash map of everything currently indexed.
// It is the basis for content-hash diffing (mtime is unreliable on cloud-synced storage).
func (s *Store) Catalog(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT path, content_hash FROM catalog`)
	if err != nil {
		return nil, fmt.Errorf("read catalog: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := map[string]string{}
	for rows.Next() {
		var path, hash string
		if err := rows.Scan(&path, &hash); err != nil {
			return nil, fmt.Errorf("scan catalog: %w", err)
		}
		out[path] = hash
	}
	return out, rows.Err()
}

// ChunkVectors returns the stored embedding for each chunk of path, keyed by the
// chunk content hash, so a reindex can reuse vectors for unchanged chunks instead
// of re-embedding them. Chunks without a vector (FTS-only) are omitted.
func (s *Store) ChunkVectors(ctx context.Context, path string) (map[string][]float32, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.chunk_hash, v.vec
		FROM chunks c JOIN vectors v ON v.chunk_id = c.id
		WHERE c.path = ?`, path)
	if err != nil {
		return nil, fmt.Errorf("read chunk vectors for %s: %w", path, err)
	}
	defer func() { _ = rows.Close() }()

	out := map[string][]float32{}
	for rows.Next() {
		var hash string
		var blob []byte
		if err := rows.Scan(&hash, &blob); err != nil {
			return nil, fmt.Errorf("scan chunk vector for %s: %w", path, err)
		}
		if hash != "" {
			out[hash] = decodeVec(blob)
		}
	}
	return out, rows.Err()
}

// Count returns the number of indexed notes and chunks.
func (s *Store) Count(ctx context.Context) (notes, chunks int, err error) {
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM catalog`).Scan(&notes); err != nil {
		return 0, 0, fmt.Errorf("count notes: %w", err)
	}
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chunks`).Scan(&chunks); err != nil {
		return 0, 0, fmt.Errorf("count chunks: %w", err)
	}
	return notes, chunks, nil
}

// --- Writes ---

// UpsertNote replaces every chunk for path with the given chunks and, when
// vectors is non-nil, their embeddings (one per chunk), then records the note in
// the catalog along with its frontmatter serialised as JSON. Passing nil vectors
// indexes FTS-only (graceful Ollama fallback). A nil frontmatter persists "{}".
func (s *Store) UpsertNote(ctx context.Context, path, hash string, chunks []vault.Chunk, vectors [][]float32, frontmatter map[string]any) error {
	if vectors != nil && len(vectors) != len(chunks) {
		return fmt.Errorf("upsert %s: %d vectors for %d chunks", path, len(vectors), len(chunks))
	}

	fmJSON := []byte("{}")
	if len(frontmatter) > 0 {
		b, err := json.Marshal(frontmatter)
		if err != nil {
			return fmt.Errorf("marshal frontmatter for %s: %w", path, err)
		}
		fmJSON = b
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM chunks WHERE path = ?`, path); err != nil {
		return fmt.Errorf("clear chunks for %s: %w", path, err)
	}

	title := ""
	for i, c := range chunks {
		if title == "" {
			title = c.Title
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO chunks (path, title, tags, heading, ord, body, token_est, chunk_hash)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			c.NotePath, c.Title, c.Tags, c.Heading, c.Ord, c.Body, c.TokenEst, vault.ContentHash([]byte(vault.ChunkEmbedText(c))))
		if err != nil {
			return fmt.Errorf("insert chunk for %s: %w", path, err)
		}
		if vectors == nil {
			continue
		}
		id, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("chunk id for %s: %w", path, err)
		}
		vec := vectors[i]
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO vectors (chunk_id, dim, vec) VALUES (?, ?, ?)`,
			id, len(vec), encodeVec(vec)); err != nil {
			return fmt.Errorf("insert vector for %s: %w", path, err)
		}
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO catalog (path, content_hash, title, frontmatter, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(path) DO UPDATE SET
		     content_hash = excluded.content_hash,
		     title        = excluded.title,
		     frontmatter  = excluded.frontmatter,
		     updated_at   = CURRENT_TIMESTAMP`,
		path, hash, title, string(fmJSON)); err != nil {
		return fmt.Errorf("upsert catalog for %s: %w", path, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit upsert %s: %w", path, err)
	}
	return nil
}

// DeleteNote removes all chunks (and their cascaded vectors) and the catalog
// entry for path.
func (s *Store) DeleteNote(ctx context.Context, path string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM chunks WHERE path = ?`, path); err != nil {
		return fmt.Errorf("delete chunks for %s: %w", path, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM catalog WHERE path = ?`, path); err != nil {
		return fmt.Errorf("delete catalog for %s: %w", path, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete %s: %w", path, err)
	}
	return nil
}
