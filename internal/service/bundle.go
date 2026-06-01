package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/alxxpersonal/stardust/internal/fusion"
	"github.com/alxxpersonal/stardust/internal/graph"
	"github.com/alxxpersonal/stardust/internal/vault"
)

// BundleItem is a note chosen for a context bundle.
type BundleItem struct {
	Path    string  `json:"path"`
	Title   string  `json:"title"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

// BundleResult is an assembled, token-budgeted context bundle for a task.
type BundleResult struct {
	Task     string       `json:"task"`
	Items    []BundleItem `json:"items"`
	Markdown string       `json:"markdown"`
	Tokens   int          `json:"tokens_estimate"`
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
	for _, node := range g.Nodes {
		nameToPath[vault.NormalizeLink(node.Path)] = node.Path
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

	fused := fusion.RRF(fusion.DefaultK, hybridList, prList)
	items := make([]BundleItem, 0, len(fused))
	for path, sc := range fused {
		items = append(items, BundleItem{Path: path, Title: titleOf[path], Snippet: snippetOf[path], Score: sc})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Score > items[j].Score })

	md := s.packBundle(task, items, budgetTokens)
	return BundleResult{Task: task, Items: items, Markdown: md, Tokens: len(md) / 4}, nil
}

// packBundle renders the bundle markdown within budget: task header, the top
// note's body (up to ~40% of budget), then summary lines for the rest, stopping
// at ~70% of budget to leave the agent just-in-time headroom.
func (s *Service) packBundle(task string, items []BundleItem, budgetTokens int) string {
	var b strings.Builder
	b.WriteString("# Context bundle\n\n## Task\n\n")
	b.WriteString(strings.TrimSpace(task) + "\n\n")

	if len(items) == 0 {
		b.WriteString("_No relevant notes found._\n")
		return b.String()
	}

	top := items[0]
	b.WriteString("## Most relevant note\n\n")
	fmt.Fprintf(&b, "### %s (`%s`)\n\n", titleOr(top), top.Path)
	if note, err := vault.Parse(s.Layout.Root, top.Path); err == nil {
		b.WriteString(truncateTokens(note.Body, budgetTokens*4/10) + "\n\n")
	} else {
		b.WriteString(top.Snippet + "\n\n")
	}

	b.WriteString("## Related notes\n\n")
	used := len(b.String()) / 4
	limit := budgetTokens * 7 / 10
	for _, it := range items[1:] {
		line := fmt.Sprintf("- **%s** `%s` - %s\n", titleOr(it), it.Path, oneLineN(it.Snippet, 120))
		if used+len(line)/4 > limit {
			break
		}
		b.WriteString(line)
		used += len(line) / 4
	}
	return b.String()
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
