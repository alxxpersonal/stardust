package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alxxpersonal/stardust/internal/gitx"
	"github.com/alxxpersonal/stardust/internal/temporal"
	"github.com/alxxpersonal/stardust/internal/vault"
)

// DigestResult is a summary of recent vault activity.
type DigestResult struct {
	Since    string `json:"since"`
	Head     string `json:"head"`
	Changed  int    `json:"changed"`
	Markdown string `json:"markdown"`
}

// Digest summarizes what changed in the vault since a commit cursor: notes
// grouped by area and the open commitments embedded in them. With advance set it
// moves the cursor (meta last_digest_sha) to HEAD so the next digest is
// incremental. An empty since uses the stored cursor, falling back to all notes.
func (s *Service) Digest(ctx context.Context, since string, advance bool) (DigestResult, error) {
	if !gitx.IsRepo(ctx, s.Layout.Root) {
		return DigestResult{}, fmt.Errorf("digest: %s is not a git repository", s.Layout.Root)
	}
	head, _ := gitx.HeadSHA(ctx, s.Layout.Root)
	if since == "" {
		since, _ = s.store.GetMeta(ctx, "last_digest_sha")
	}
	changed, err := gitx.DiffNames(ctx, s.Layout.Root, since)
	if err != nil {
		return DigestResult{}, err
	}
	changed = filterIgnored(changed, s.Config.Ignore)

	type noteInfo struct {
		path, title string
		deleted     bool
	}
	groups := map[string][]noteInfo{}
	var commitments []temporal.Commitment
	for _, rel := range changed {
		rel = filepath.ToSlash(rel)
		area := temporal.TopArea(rel)
		if _, statErr := os.Stat(filepath.Join(s.Layout.Root, rel)); statErr != nil {
			groups[area] = append(groups[area], noteInfo{path: rel, deleted: true})
			continue
		}
		note, err := vault.Parse(s.Layout.Root, rel)
		if err != nil {
			continue
		}
		groups[area] = append(groups[area], noteInfo{path: rel, title: note.Title})
		commitments = append(commitments, temporal.Commitments(rel, note.Body)...)
	}

	var b strings.Builder
	b.WriteString("# Digest\n\n")
	if since == "" {
		fmt.Fprintf(&b, "All %d tracked notes (no prior digest cursor).\n\n", len(changed))
	} else {
		fmt.Fprintf(&b, "%d notes changed since `%s`.\n\n", len(changed), shortSHA(since))
	}

	areas := make([]string, 0, len(groups))
	for a := range groups {
		areas = append(areas, a)
	}
	sort.Strings(areas)
	b.WriteString("## Changed by area\n\n")
	if len(areas) == 0 {
		b.WriteString("_Nothing changed._\n\n")
	}
	for _, a := range areas {
		fmt.Fprintf(&b, "### %s\n\n", a)
		for _, n := range groups[a] {
			if n.deleted {
				fmt.Fprintf(&b, "- _(deleted)_ `%s`\n", n.path)
				continue
			}
			title := n.title
			if title == "" {
				title = n.path
			}
			fmt.Fprintf(&b, "- **%s** `%s`\n", title, n.path)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "## Open commitments (%d)\n\n", len(commitments))
	if len(commitments) == 0 {
		b.WriteString("_None surfaced._\n")
	}
	for _, c := range commitments {
		fmt.Fprintf(&b, "- `%s`: %s\n", c.Path, oneLineN(c.Line, 120))
	}

	if advance && head != "" {
		_ = s.store.SetMeta(ctx, "last_digest_sha", head)
	}
	return DigestResult{Since: since, Head: head, Changed: len(changed), Markdown: b.String()}, nil
}

func shortSHA(s string) string {
	if len(s) > 10 {
		return s[:10]
	}
	return s
}
