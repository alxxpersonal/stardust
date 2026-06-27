package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/alxxpersonal/stardust/internal/fusion"
	"github.com/alxxpersonal/stardust/internal/gitx"
	"github.com/alxxpersonal/stardust/internal/graph"
	"github.com/alxxpersonal/stardust/internal/vault"
)

// Provenance values record why a note made a bundle: a keyword (FTS) hit, a
// semantic (vector) neighbor, link-expansion (reached over a wikilink doc edge by
// PageRank), or a frontmatter-ref (reached over a related: edge).
const (
	ProvenanceKeyword       = "keyword"
	ProvenanceSemantic      = "semantic"
	ProvenanceLinkExpansion = "link-expansion"
	ProvenanceFrontmatter   = "frontmatter-ref"
)

// BundleItem is a note chosen for a context bundle. Provenance records the
// channels that selected it, so the agent consumer can see why each note is here;
// Drift records any code the note references that moved since the note's last
// commit, so the agent knows the note may trail its code.
type BundleItem struct {
	Path       string         `json:"path"`
	Title      string         `json:"title"`
	Snippet    string         `json:"snippet"`
	Score      float64        `json:"score"`
	Provenance []string       `json:"provenance"`
	Drift      []DriftBinding `json:"drift,omitempty"`
}

// BundleResult is an assembled, token-budgeted context bundle for a task.
// RetrievalMode and RetrievalReason are inherited from the underlying query so a
// bundle, like a query, announces when its recall degraded to FTS-only.
type BundleResult struct {
	Task            string       `json:"task"`
	Items           []BundleItem `json:"items"`
	Markdown        string       `json:"markdown"`
	Tokens          int          `json:"tokens_estimate"`
	RetrievalMode   string       `json:"retrieval_mode"`
	RetrievalReason string       `json:"retrieval_reason,omitempty"`
	CommitsBehind   int          `json:"commits_behind"`
}

// Bundle assembles the context an agent should boot with for a task: it seeds
// from hybrid recall, expands over the link graph with personalized PageRank,
// fuses the two rankings with RRF, then packs the result to a token budget (the
// most relevant note's body plus summary lines for the rest, with headroom).
func (s *Service) Bundle(ctx context.Context, task string, budgetTokens int) (BundleResult, error) {
	if budgetTokens <= 0 {
		budgetTokens = 4000
	}

	res, err := s.Query(ctx, task, 20)
	if err != nil {
		return BundleResult{}, err
	}

	seeds := make([]string, 0, 5)
	hybridList := make([]string, 0, len(res.Hits))
	snippetOf := map[string]string{}
	for i, h := range res.Hits {
		hybridList = append(hybridList, h.Path)
		snippetOf[h.Path] = h.Snippet
		if i < 5 {
			seeds = append(seeds, h.Path)
		}
	}

	g, err := graph.Build(s.Layout.Root, s.Config.Ignore)
	if err != nil {
		return BundleResult{}, err
	}
	pr := g.PersonalizedPageRank(seeds, 30, 0.85)

	nameToPath := map[string]string{}
	titleOf := map[string]string{}
	for key, node := range g.Nodes {
		nameToPath[key] = node.Path
		titleOf[node.Path] = node.Title
	}
	type ranked struct {
		path  string
		score float64
	}
	prRanked := make([]ranked, 0, len(pr))
	for name, sc := range pr {
		if p, ok := nameToPath[name]; ok {
			prRanked = append(prRanked, ranked{p, sc})
		}
	}
	sort.Slice(prRanked, func(i, j int) bool { return prRanked[i].score > prRanked[j].score })
	prList := make([]string, 0, 20)
	for i, r := range prRanked {
		if i >= 20 {
			break
		}
		prList = append(prList, r.path)
	}

	// Index the incoming doc-edge kinds per node so a PageRank-reached item can be
	// attributed to the edge that connects it (wikilink vs related).
	incomingKinds := map[string]map[string]bool{}
	for _, node := range g.Nodes {
		for _, e := range node.Out {
			if incomingKinds[e.Target] == nil {
				incomingKinds[e.Target] = map[string]bool{}
			}
			incomingKinds[e.Target][e.Kind] = true
		}
	}
	hybridSet := toSet(hybridList)
	prSet := toSet(prList)

	// Index each node's doc-to-code references so a bundled note that trails its
	// referenced code can carry the drift binding (ADR 0018).
	codeRefsByPath := map[string][]string{}
	for _, node := range g.Nodes {
		if len(node.CodeRefs) > 0 {
			codeRefsByPath[node.Path] = codeRefTargets(node.CodeRefs)
		}
	}

	fused := fusion.RRF(fusion.DefaultK, hybridList, prList)
	items := make([]BundleItem, 0, len(fused))
	for path, sc := range fused {
		prov := provenanceFor(path, hybridSet, prSet, res.RetrievalMode, incomingKinds)
		items = append(items, BundleItem{Path: path, Title: titleOf[path], Snippet: snippetOf[path], Score: sc, Provenance: prov})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Score > items[j].Score })

	for i := range items {
		refs := codeRefsByPath[items[i].Path]
		if len(refs) == 0 {
			continue
		}
		bindings, err := s.docDrift(ctx, items[i].Path, refs)
		if err != nil {
			return BundleResult{}, err
		}
		items[i].Drift = bindings
	}

	behind, hasBehind := s.commitsBehindHead(ctx)
	md := s.packBundle(task, items, budgetTokens, behind, hasBehind)
	return BundleResult{
		Task:            task,
		Items:           items,
		Markdown:        md,
		Tokens:          len(md) / 4,
		RetrievalMode:   res.RetrievalMode,
		RetrievalReason: res.RetrievalReason,
		CommitsBehind:   behind,
	}, nil
}

// commitsBehindHead reports how many commits HEAD is ahead of the index cursor
// (last_indexed_sha), so a bundle can declare how stale its index is. It returns
// ok false (and zero) outside a git repo or when the cursor is unset, so the
// stamp is simply omitted rather than guessed.
func (s *Service) commitsBehindHead(ctx context.Context) (int, bool) {
	if !gitx.IsRepo(ctx, s.Layout.Root) {
		return 0, false
	}
	sha, err := s.store.GetMeta(ctx, "last_indexed_sha")
	if err != nil || sha == "" {
		return 0, false
	}
	n, err := gitx.CommitCountSince(ctx, s.Layout.Root, sha)
	if err != nil {
		return 0, false
	}
	return n, true
}

// packBundle renders the bundle markdown within budget: task header, an optional
// freshness stamp, the top note's body (up to ~40% of budget), then summary lines
// for the rest, stopping at ~70% of budget to leave the agent just-in-time
// headroom. When hasBehind is true it stamps how far behind HEAD the index is, so
// the agent knows when to re-verify.
func (s *Service) packBundle(task string, items []BundleItem, budgetTokens, behind int, hasBehind bool) string {
	var b strings.Builder
	b.WriteString("# Context bundle\n\n## Task\n\n")
	b.WriteString(strings.TrimSpace(task) + "\n\n")
	if hasBehind {
		fmt.Fprintf(&b, "_index is %d commits behind HEAD._\n\n", behind)
	}

	if len(items) == 0 {
		b.WriteString("_No relevant notes found._\n")
		return b.String()
	}

	top := items[0]
	b.WriteString("## Most relevant note\n\n")
	fmt.Fprintf(&b, "### %s (`%s`)\n\n", titleOr(top), top.Path)
	if tag := provTag(top.Provenance); tag != "" {
		fmt.Fprintf(&b, "_via:%s_\n\n", tag)
	}
	if d := driftNote(top.Drift); d != "" {
		fmt.Fprintf(&b, "_drift: %s; review_\n\n", d)
	}
	if note, err := vault.Parse(s.Layout.Root, top.Path); err == nil {
		b.WriteString(truncateTokens(note.Body, budgetTokens*4/10) + "\n\n")
	} else {
		b.WriteString(top.Snippet + "\n\n")
	}

	b.WriteString("## Related notes\n\n")
	used := len(b.String()) / 4
	limit := budgetTokens * 7 / 10
	for _, it := range items[1:] {
		line := fmt.Sprintf("- **%s** `%s`%s%s - %s\n", titleOr(it), it.Path, provTag(it.Provenance), driftTag(it.Drift), oneLineN(it.Snippet, 120))
		if used+len(line)/4 > limit {
			break
		}
		b.WriteString(line)
		used += len(line) / 4
	}
	return b.String()
}

// driftNote renders a note's drift bindings as a human-readable clause naming each
// moved file and its commit count, or an empty string when the note has not
// drifted.
func driftNote(bindings []DriftBinding) string {
	if len(bindings) == 0 {
		return ""
	}
	parts := make([]string, 0, len(bindings))
	for _, bind := range bindings {
		parts = append(parts, fmt.Sprintf("`%s` moved %s", bind.File, pluralCommits(bind.ChangedCommits)))
	}
	return strings.Join(parts, ", ")
}

// driftTag renders a related-note's drift bindings as an inline bracketed review
// prompt, or an empty string when the note has not drifted.
func driftTag(bindings []DriftBinding) string {
	if d := driftNote(bindings); d != "" {
		return " [drift: " + d + "; review]"
	}
	return ""
}

// pluralCommits renders a commit count with the right noun, e.g. "1 commit" or
// "4 commits".
func pluralCommits(n int) string {
	if n == 1 {
		return "1 commit"
	}
	return fmt.Sprintf("%d commits", n)
}

// provenanceFor records why a fused item was selected: keyword (and semantic when
// the run was hybrid-semantic) for an FTS hit, plus link-expansion or
// frontmatter-ref for a PageRank-reached node, attributed by its incoming doc-edge
// kind. Order is stable so the rendered tag is deterministic.
func provenanceFor(path string, hybridSet, prSet map[string]bool, retrievalMode string, incomingKinds map[string]map[string]bool) []string {
	var prov []string
	if hybridSet[path] {
		prov = append(prov, ProvenanceKeyword)
		if retrievalMode == RetrievalHybridSemantic {
			prov = append(prov, ProvenanceSemantic)
		}
	}
	if prSet[path] {
		kinds := incomingKinds[vault.GraphKey(path)]
		if kinds[vault.EdgeWikilink] {
			prov = append(prov, ProvenanceLinkExpansion)
		}
		if kinds[vault.EdgeMarkdownLink] {
			prov = append(prov, ProvenanceLinkExpansion)
		}
		if kinds[vault.EdgeRelated] {
			prov = append(prov, ProvenanceFrontmatter)
		}
	}
	return prov
}

// toSet builds a membership set from a path slice.
func toSet(items []string) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, it := range items {
		s[it] = true
	}
	return s
}

// provTag renders a bundle item's provenance as an inline bracketed tag, or an
// empty string when there is none.
func provTag(prov []string) string {
	if len(prov) == 0 {
		return ""
	}
	return " [" + strings.Join(prov, ", ") + "]"
}

func titleOr(it BundleItem) string {
	if it.Title != "" {
		return it.Title
	}
	return it.Path
}

// truncateTokens keeps roughly the first n tokens (estimated as 4 chars each).
func truncateTokens(s string, n int) string {
	max := n * 4
	r := []rune(s)
	if len(r) <= max {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(string(r[:max])) + "\n\n[...truncated]"
}

func oneLineN(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
