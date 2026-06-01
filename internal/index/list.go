package index

import (
	"context"
	"fmt"
)

// NoteRow is a catalog entry with a representative leading snippet, used to
// generate the INDEX.md table of contents.
type NoteRow struct {
	Path    string
	Title   string
	Snippet string
}

// ListNotes returns every indexed note with its title and a leading snippet,
// ordered by path.
func (s *Store) ListNotes(ctx context.Context) ([]NoteRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT cat.path, cat.title,
		       COALESCE((SELECT body FROM chunks WHERE chunks.path = cat.path ORDER BY ord LIMIT 1), '')
		FROM catalog cat
		ORDER BY cat.path`)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []NoteRow
	for rows.Next() {
		var n NoteRow
		var body string
		if err := rows.Scan(&n.Path, &n.Title, &body); err != nil {
			return nil, fmt.Errorf("scan note row: %w", err)
		}
		n.Snippet = snippet(body)
		out = append(out, n)
	}
	return out, rows.Err()
}
