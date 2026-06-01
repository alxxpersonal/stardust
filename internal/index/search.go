package index

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
)

// rrfK is the Reciprocal Rank Fusion constant. Rank-based fusion needs no score
// calibration, so FTS (BM25) and vector (cosine) rankings combine cleanly.
const rrfK = 60

// Hit is a single retrieval result, collapsed to one row per note (the best
// matching chunk stands in for its parent note).
type Hit struct {
	Path    string  `json:"path"`
	Title   string  `json:"title"`
	Heading string  `json:"heading"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

// rankedChunk is an internal candidate carrying enough to render a Hit.
type rankedChunk struct {
	id      int64
	path    string
	title   string
	heading string
	body    string
}

// Hybrid runs FTS5 BM25 plus, when queryVec is non-nil, brute-force vector
// search, fuses the two rankings with RRF, collapses to the best chunk per note,
// and returns up to limit hits. With a nil queryVec it degrades to FTS-only.
func (s *Store) Hybrid(ctx context.Context, query string, queryVec []float32, limit int) ([]Hit, error) {
	if limit <= 0 {
		limit = 10
	}
	pool := limit * 5

	fts, err := s.searchFTS(ctx, query, pool)
	if err != nil {
		return nil, err
	}

	var vec []rankedChunk
	if queryVec != nil {
		vec, err = s.searchVec(ctx, queryVec, pool)
		if err != nil {
			return nil, err
		}
	}

	scores := map[int64]float64{}
	meta := map[int64]rankedChunk{}
	accumulate := func(list []rankedChunk) {
		for rank, rc := range list {
			scores[rc.id] += 1.0 / float64(rrfK+rank+1)
			meta[rc.id] = rc
		}
	}
	accumulate(fts)
	accumulate(vec)

	// collapse to the best-scoring chunk per note (parent-document)
	type best struct {
		score float64
		rc    rankedChunk
	}
	byPath := map[string]best{}
	for id, sc := range scores {
		rc := meta[id]
		if cur, ok := byPath[rc.path]; !ok || sc > cur.score {
			byPath[rc.path] = best{score: sc, rc: rc}
		}
	}

	hits := make([]Hit, 0, len(byPath))
	for _, b := range byPath {
		hits = append(hits, Hit{
			Path:    b.rc.path,
			Title:   b.rc.title,
			Heading: b.rc.heading,
			Snippet: snippet(b.rc.body),
			Score:   b.score,
		})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

// searchFTS runs an FTS5 MATCH ranked by weighted BM25 (title 2.0, tags 1.0,
// heading 1.5, body 1.0). Lower BM25 is better, so the slice is best-first.
func (s *Store) searchFTS(ctx context.Context, query string, limit int) ([]rankedChunk, error) {
	match := ftsQuery(query)
	if match == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.path, c.title, c.heading, c.body
		FROM chunks_fts
		JOIN chunks c ON c.id = chunks_fts.rowid
		WHERE chunks_fts MATCH ?
		ORDER BY bm25(chunks_fts, 2.0, 1.0, 1.5, 1.0)
		LIMIT ?`, match, limit)
	if err != nil {
		return nil, fmt.Errorf("fts search: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanRanked(rows)
}

// searchVec brute-forces cosine similarity over every stored vector. At personal
// scale a flat scan is instant and keeps the single static binary (modernc
// cannot load the sqlite-vec C extension).
func (s *Store) searchVec(ctx context.Context, query []float32, limit int) ([]rankedChunk, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.path, c.title, c.heading, c.body, v.vec
		FROM vectors v
		JOIN chunks c ON c.id = v.chunk_id`)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type scored struct {
		rc  rankedChunk
		sim float64
	}
	var all []scored
	for rows.Next() {
		var rc rankedChunk
		var blob []byte
		if err := rows.Scan(&rc.id, &rc.path, &rc.title, &rc.heading, &rc.body, &blob); err != nil {
			return nil, fmt.Errorf("scan vector row: %w", err)
		}
		all = append(all, scored{rc: rc, sim: cosine(query, decodeVec(blob))})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate vectors: %w", err)
	}

	sort.Slice(all, func(i, j int) bool { return all[i].sim > all[j].sim })
	if len(all) > limit {
		all = all[:limit]
	}
	out := make([]rankedChunk, len(all))
	for i, s := range all {
		out[i] = s.rc
	}
	return out, nil
}

// Nearest returns the notes whose chunks are most cosine-similar to query,
// collapsed to the best chunk per note, with Score set to that cosine. It is the
// dedup-before-write primitive. Returns nil when no vectors are indexed.
func (s *Store) Nearest(ctx context.Context, query []float32, limit int) ([]Hit, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.path, c.title, c.heading, c.body, v.vec
		FROM vectors v
		JOIN chunks c ON c.id = v.chunk_id`)
	if err != nil {
		return nil, fmt.Errorf("nearest: %w", err)
	}
	defer func() { _ = rows.Close() }()

	best := map[string]Hit{}
	for rows.Next() {
		var path, title, heading, body string
		var blob []byte
		if err := rows.Scan(&path, &title, &heading, &body, &blob); err != nil {
			return nil, fmt.Errorf("scan nearest: %w", err)
		}
		sim := cosine(query, decodeVec(blob))
		if cur, ok := best[path]; !ok || sim > cur.Score {
			best[path] = Hit{Path: path, Title: title, Heading: heading, Snippet: snippet(body), Score: sim}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate nearest: %w", err)
	}

	out := make([]Hit, 0, len(best))
	for _, h := range best {
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// --- Helpers ---

func scanRanked(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]rankedChunk, error) {
	var out []rankedChunk
	for rows.Next() {
		var rc rankedChunk
		if err := rows.Scan(&rc.id, &rc.path, &rc.title, &rc.heading, &rc.body); err != nil {
			return nil, fmt.Errorf("scan ranked chunk: %w", err)
		}
		out = append(out, rc)
	}
	return out, rows.Err()
}

// ftsQuery turns free text into a safe FTS5 MATCH expression: alphanumeric terms
// quoted and OR-joined, so punctuation never breaks the parser.
func ftsQuery(raw string) string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	var terms []string
	for _, f := range fields {
		terms = append(terms, `"`+f+`"`)
	}
	return strings.Join(terms, " OR ")
}

func snippet(body string) string {
	const max = 280
	s := strings.Join(strings.Fields(body), " ")
	if len(s) > max {
		s = strings.TrimSpace(s[:max]) + "..."
	}
	return s
}

// --- Vector codec + similarity ---

// encodeVec serialises a float32 vector as little-endian bytes.
func encodeVec(v []float32) []byte {
	b := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(f))
	}
	return b
}

// decodeVec parses little-endian bytes back into a float32 vector.
func decodeVec(b []byte) []float32 {
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}

// cosine returns the cosine similarity of two equal-length vectors, or 0 when
// lengths differ or either is zero.
func cosine(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
