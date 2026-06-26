package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alxxpersonal/stardust/internal/gitx"
	"github.com/alxxpersonal/stardust/internal/index"
	"github.com/alxxpersonal/stardust/internal/manifest"
	"github.com/alxxpersonal/stardust/internal/vault"
)

// IndexStats summarizes an indexing run.
type IndexStats struct {
	Indexed int  `json:"indexed"`
	Skipped int  `json:"skipped"`
	Deleted int  `json:"deleted"`
	Vectors bool `json:"vectors"`
}

// Index incrementally indexes the vault. A non-empty since uses the git-diff
// fast path; otherwise it full-scans. Unchanged notes are skipped by content
// hash, deletes are pruned, and INDEX.md is regenerated.
func (s *Service) Index(ctx context.Context, since string) (IndexStats, error) {
	isRepo := gitx.IsRepo(ctx, s.Layout.Root)
	headSHA := ""
	if isRepo {
		headSHA, _ = gitx.HeadSHA(ctx, s.Layout.Root)
	}

	var paths []string
	var currentPaths []string
	var err error
	if since != "" && isRepo {
		paths, err = gitx.DiffNames(ctx, s.Layout.Root, since)
		if err == nil {
			currentPaths, err = vault.Scan(s.Layout.Root, s.Config.Ignore)
		}
	} else {
		paths, err = vault.Scan(s.Layout.Root, s.Config.Ignore)
		currentPaths = append([]string(nil), paths...)
	}
	if err != nil {
		return IndexStats{}, err
	}
	paths = filterIgnored(paths, s.Config.Ignore)
	currentPaths = filterIgnored(currentPaths, s.Config.Ignore)

	catalog, err := s.store.Catalog(ctx)
	if err != nil {
		return IndexStats{}, err
	}

	useVectors := s.embed.Available(ctx)
	var stats IndexStats
	current := pathSet(currentPaths)
	for rel := range catalog {
		if current[rel] {
			continue
		}
		if err := s.store.DeleteNote(ctx, rel); err != nil {
			return stats, err
		}
		delete(catalog, rel)
		stats.Deleted++
	}

	for _, rel := range paths {
		rel = filepath.ToSlash(rel)
		if _, statErr := os.Stat(filepath.Join(s.Layout.Root, rel)); statErr != nil {
			if _, ok := catalog[rel]; ok {
				if err := s.store.DeleteNote(ctx, rel); err != nil {
					return stats, err
				}
				stats.Deleted++
			}
			continue
		}

		note, err := vault.Parse(s.Layout.Root, rel)
		if err != nil {
			return stats, err
		}
		if h, ok := catalog[note.Path]; ok && h == note.Hash {
			stats.Skipped++
			continue
		}

		chunks := vault.Chunks(note)
		var vectors [][]float32
		if useVectors {
			vectors, err = s.embedNoteChunks(ctx, note.Path, chunks)
			if err != nil {
				useVectors = false // degrade to FTS-only for the rest of the run
				vectors = nil
			}
		}
		if err := s.store.UpsertNote(ctx, note.Path, note.Hash, chunks, vectors, note.Frontmatter); err != nil {
			return stats, err
		}
		stats.Indexed++
	}

	if isRepo && headSHA != "" {
		if err := s.store.SetMeta(ctx, "last_indexed_sha", headSHA); err != nil {
			return stats, err
		}
	}
	_ = s.store.SetMeta(ctx, "embed_model", s.embed.Model())

	notes, err := s.store.ListNotes(ctx)
	if err != nil {
		return stats, err
	}
	if err := manifest.WriteIndex(s.Layout.IndexMD(), notes); err != nil {
		return stats, err
	}
	stats.Vectors = useVectors
	return stats, nil
}

// Rebuild deletes the derived cache and reindexes from scratch.
func (s *Service) Rebuild(ctx context.Context) (IndexStats, error) {
	_ = s.store.Close()
	if err := os.RemoveAll(s.Layout.Cache()); err != nil {
		return IndexStats{}, fmt.Errorf("clear cache: %w", err)
	}
	store, err := index.Open(ctx, s.Layout.DB())
	if err != nil {
		return IndexStats{}, err
	}
	s.store = store
	return s.Index(ctx, "")
}

// Archive snapshots the vault's git history into dest (default .stardust/archives).
func (s *Service) Archive(ctx context.Context, dest string) (string, error) {
	if !gitx.IsRepo(ctx, s.Layout.Root) {
		return "", fmt.Errorf("archive: %s is not a git repository", s.Layout.Root)
	}
	if dest == "" {
		dest = filepath.Join(s.Layout.Dir(), "archives")
	}
	return gitx.Archive(ctx, s.Layout.Root, dest)
}

// --- Helpers ---

// filterIgnored drops paths under any ignored directory segment.
func filterIgnored(paths, ignore []string) []string {
	ig := make(map[string]bool, len(ignore))
	for _, x := range ignore {
		ig[x] = true
	}
	out := paths[:0]
	for _, p := range paths {
		skip := false
		for _, seg := range strings.Split(p, "/") {
			if ig[seg] {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, p)
		}
	}
	return out
}

// pathSet maps vault-relative paths to true.
func pathSet(paths []string) map[string]bool {
	out := make(map[string]bool, len(paths))
	for _, path := range paths {
		out[filepath.ToSlash(path)] = true
	}
	return out
}

// embedChunks builds one embedding per chunk from its title, heading, and body.
// It is the full-note path used by single-note reindexing.
func embedChunks(ctx context.Context, emb embedder, chunks []vault.Chunk) ([][]float32, error) {
	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = vault.ChunkEmbedText(c)
	}
	return emb.Embed(ctx, texts)
}

// embedNoteChunks returns one vector per chunk, embedding only the chunks whose
// content hash changed since the last index and reusing stored vectors for the
// rest. This makes incremental reindexing cheap enough to keep vectors on by
// default: a one-heading edit re-embeds one chunk, not the whole note.
func (s *Service) embedNoteChunks(ctx context.Context, path string, chunks []vault.Chunk) ([][]float32, error) {
	hashes := chunkHashes(chunks)
	existing, err := s.store.ChunkVectors(ctx, path)
	if err != nil {
		return nil, err
	}
	toEmbed, reuse := embedPlan(hashes, existing)
	vectors := make([][]float32, len(chunks))
	for i, v := range reuse {
		vectors[i] = v
	}
	if len(toEmbed) == 0 {
		return vectors, nil
	}
	texts := make([]string, len(toEmbed))
	for j, idx := range toEmbed {
		texts[j] = vault.ChunkEmbedText(chunks[idx])
	}
	embedded, err := s.embed.Embed(ctx, texts)
	if err != nil {
		return nil, err
	}
	if len(embedded) != len(texts) {
		return nil, fmt.Errorf("embed chunks for %s: got %d vectors for %d inputs", path, len(embedded), len(texts))
	}
	for j, idx := range toEmbed {
		vectors[idx] = embedded[j]
	}
	return vectors, nil
}

// chunkHashes returns the per-chunk content hash of the exact embedded text, the
// same hash UpsertNote persists, so reuse decisions and storage stay aligned.
func chunkHashes(chunks []vault.Chunk) []string {
	out := make([]string, len(chunks))
	for i, c := range chunks {
		out[i] = vault.ContentHash([]byte(vault.ChunkEmbedText(c)))
	}
	return out
}

// embedPlan splits a note's chunk hashes into the indices that must be embedded
// (new or changed) and a map of indices that can reuse an existing vector.
func embedPlan(hashes []string, existing map[string][]float32) (toEmbed []int, reuse map[int][]float32) {
	reuse = map[int][]float32{}
	for i, h := range hashes {
		if vec, ok := existing[h]; ok && vec != nil {
			reuse[i] = vec
		} else {
			toEmbed = append(toEmbed, i)
		}
	}
	return toEmbed, reuse
}
