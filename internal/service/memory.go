package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alxxpersonal/stardust/internal/memory"
	"github.com/alxxpersonal/stardust/internal/vault"
)

// dedupThreshold is the cosine above which a new fact is appended to the nearest
// existing note rather than landing in a fresh note.
const dedupThreshold = 0.6

// MemoryOp is a single memory-tool operation.
type MemoryOp struct {
	Command string // view | create | str_replace | insert | delete | rename
	Path    string
	Content string
	Old     string
	New     string
	Line    int
	Text    string
	Dest    string
}

// Memory executes a memory verb, then re-derives the index for the affected
// markdown file. The vault stays the source of truth; the index follows.
func (s *Service) Memory(ctx context.Context, op MemoryOp) (string, error) {
	mem := memory.New(s.Layout.Root)
	switch op.Command {
	case "view":
		return mem.View(op.Path)
	case "create":
		if err := mem.Create(op.Path, op.Content); err != nil {
			return "", err
		}
		_ = s.reindexPath(ctx, op.Path)
		return "created " + op.Path, nil
	case "str_replace":
		if err := mem.StrReplace(op.Path, op.Old, op.New); err != nil {
			return "", err
		}
		_ = s.reindexPath(ctx, op.Path)
		return "replaced in " + op.Path, nil
	case "insert":
		if err := mem.Insert(op.Path, op.Line, op.Text); err != nil {
			return "", err
		}
		_ = s.reindexPath(ctx, op.Path)
		return "inserted into " + op.Path, nil
	case "delete":
		if err := mem.Delete(op.Path); err != nil {
			return "", err
		}
		_ = s.reindexPath(ctx, op.Path)
		return "deleted " + op.Path, nil
	case "rename":
		if err := mem.Rename(op.Path, op.Dest); err != nil {
			return "", err
		}
		_ = s.reindexPath(ctx, op.Path)
		_ = s.reindexPath(ctx, op.Dest)
		return "renamed " + op.Path + " -> " + op.Dest, nil
	default:
		return "", fmt.Errorf("unknown memory command: %q", op.Command)
	}
}

// RememberResult records where a fact landed.
type RememberResult struct {
	Action string `json:"action"` // appended | created
	Path   string `json:"path"`
}

// Remember stores a fact add-only: it embeds the fact, and if a sufficiently
// similar note exists, appends to it; otherwise it creates a dated note under
// memory/. The index is re-derived for the changed file.
func (s *Service) Remember(ctx context.Context, fact string) (RememberResult, error) {
	fact = strings.TrimSpace(fact)
	if fact == "" {
		return RememberResult{}, fmt.Errorf("remember: empty fact")
	}
	mem := memory.New(s.Layout.Root)
	stamp := time.Now().Format("2006-01-02")
	line := fmt.Sprintf("- %s (added %s)", fact, stamp)

	// dedup-before-write: append to the nearest note when similar enough
	if s.embed.Available(ctx) {
		if vecs, err := s.embed.Embed(ctx, []string{fact}); err == nil && len(vecs) == 1 {
			if near, err := s.store.Nearest(ctx, vecs[0], 1); err == nil && len(near) > 0 && near[0].Score >= dedupThreshold {
				if err := mem.Append(near[0].Path, "\n"+line+"\n"); err != nil {
					return RememberResult{}, err
				}
				_ = s.reindexPath(ctx, near[0].Path)
				return RememberResult{Action: "appended", Path: near[0].Path}, nil
			}
		}
	}

	path := "memory/" + stamp + "-" + slugify(fact) + ".md"
	content := fmt.Sprintf("---\ntitle: %s\ncreated: %s\ntags: [memory]\n---\n\n%s\n", firstWords(fact, 8), stamp, line)
	if err := mem.Create(path, content); err != nil {
		return RememberResult{}, err
	}
	_ = s.reindexPath(ctx, path)
	return RememberResult{Action: "created", Path: path}, nil
}

// reindexPath re-derives the index for a single markdown file (or prunes it when
// the file no longer exists). Non-markdown paths are ignored.
func (s *Service) reindexPath(ctx context.Context, rel string) error {
	rel = filepath.ToSlash(rel)
	if !strings.HasSuffix(strings.ToLower(rel), ".md") {
		return nil
	}
	if _, err := os.Stat(filepath.Join(s.Layout.Root, rel)); err != nil {
		return s.store.DeleteNote(ctx, rel)
	}
	note, err := vault.Parse(s.Layout.Root, rel)
	if err != nil {
		return err
	}
	chunks := vault.Chunks(note)
	var vectors [][]float32
	if s.embed.Available(ctx) {
		vectors, _ = embedChunks(ctx, s.embed, chunks)
	}
	return s.store.UpsertNote(ctx, note.Path, note.Hash, chunks, vectors)
}

// --- Helpers ---

func slugify(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 40 {
		out = strings.Trim(out[:40], "-")
	}
	if out == "" {
		out = "note"
	}
	return out
}

func firstWords(s string, n int) string {
	fields := strings.Fields(s)
	if len(fields) > n {
		fields = fields[:n]
	}
	return strings.Join(fields, " ")
}
