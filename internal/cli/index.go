package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/alxxpersonal/stardust/internal/gitx"
	"github.com/alxxpersonal/stardust/internal/manifest"
	"github.com/alxxpersonal/stardust/internal/vault"
)

// newIndexCmd builds the incremental indexer command.
func newIndexCmd() *cobra.Command {
	var since string
	var background bool
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Index changed notes into the search index",
		Long:  "Incrementally indexes the vault. With git it diffs from the last indexed\ncommit; otherwise it scans the tree. Unchanged notes are skipped by content hash.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if background {
				return spawnBackgroundIndex(since)
			}
			return runIndex(cmd, since)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "only index notes changed since this git SHA")
	cmd.Flags().BoolVar(&background, "background", false, "detach and index in the background")
	return cmd
}

// spawnBackgroundIndex re-execs stardust index detached, for the commit hook.
func spawnBackgroundIndex(since string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	args := []string{"index"}
	if since != "" {
		args = append(args, "--since", since)
	}
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start background index: %w", err)
	}
	return cmd.Process.Release()
}

func runIndex(cmd *cobra.Command, since string) error {
	ctx := cmd.Context()
	vc, err := resolveVault()
	if err != nil {
		return err
	}
	store, err := vc.openStore(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	isRepo := gitx.IsRepo(ctx, vc.Layout.Root)
	headSHA := ""
	if isRepo {
		headSHA, _ = gitx.HeadSHA(ctx, vc.Layout.Root)
	}

	// --since is the git-diff fast path (the commit hook). The default is a full
	// scan with content-hash skip, which also catches uncommitted working-tree
	// edits; content hash, not mtime, is the authority.
	var paths []string
	if since != "" && isRepo {
		paths, err = gitx.DiffNames(ctx, vc.Layout.Root, since)
	} else {
		paths, err = vault.Scan(vc.Layout.Root, vc.Config.Ignore)
	}
	if err != nil {
		return err
	}
	// the git-diff path returns raw tracked paths, so filter ignored directories
	// (notably .stardust itself) on both sources to avoid an index feedback loop
	paths = filterIgnored(paths, vc.Config.Ignore)

	catalog, err := store.Catalog(ctx)
	if err != nil {
		return err
	}

	embedder := vc.embedder()
	useVectors := embedder.Available(ctx)

	var indexed, skipped, deleted int
	for _, rel := range paths {
		rel = filepath.ToSlash(rel)
		if _, statErr := os.Stat(filepath.Join(vc.Layout.Root, rel)); statErr != nil {
			if _, ok := catalog[rel]; ok {
				if err := store.DeleteNote(ctx, rel); err != nil {
					return err
				}
				deleted++
			}
			continue
		}

		note, err := vault.Parse(vc.Layout.Root, rel)
		if err != nil {
			return err
		}
		if h, ok := catalog[note.Path]; ok && h == note.Hash {
			skipped++
			continue
		}

		chunks := vault.Chunks(note)
		var vectors [][]float32
		if useVectors {
			vectors, err = embedChunks(ctx, embedder, chunks)
			if err != nil {
				// degrade to FTS-only for the rest of the run
				fmt.Fprintf(os.Stderr, "embedding unavailable, continuing FTS-only: %v\n", err)
				useVectors = false
				vectors = nil
			}
		}
		if err := store.UpsertNote(ctx, note.Path, note.Hash, chunks, vectors); err != nil {
			return err
		}
		indexed++
	}

	if isRepo && headSHA != "" {
		if err := store.SetMeta(ctx, "last_indexed_sha", headSHA); err != nil {
			return err
		}
	}
	_ = store.SetMeta(ctx, "embed_model", embedder.Model())

	notes, err := store.ListNotes(ctx)
	if err != nil {
		return err
	}
	if err := manifest.WriteIndex(vc.Layout.IndexMD(), notes); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "indexed %d, skipped %d, deleted %d (vectors: %t)\n", indexed, skipped, deleted, useVectors)
	return nil
}

// filterIgnored drops paths that fall under any ignored directory segment.
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

// embedChunks builds one embedding per chunk from its title, heading, and body.
func embedChunks(ctx context.Context, embedder interface {
	Embed(context.Context, []string) ([][]float32, error)
}, chunks []vault.Chunk) ([][]float32, error) {
	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = strings.TrimSpace(c.Title + "\n" + c.Heading + "\n" + c.Body)
	}
	return embedder.Embed(ctx, texts)
}
