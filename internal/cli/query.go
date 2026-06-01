package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alxxpersonal/stardust/internal/index"
	"github.com/alxxpersonal/stardust/internal/rerank"
)

// newQueryCmd builds the search command.
func newQueryCmd() *cobra.Command {
	var limit int
	var output string
	cmd := &cobra.Command{
		Use:   "query <text>",
		Short: "Search the vault (hybrid keyword + semantic)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQuery(cmd, strings.Join(args, " "), limit, output)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "maximum number of results")
	cmd.Flags().StringVar(&output, "output", "auto", "output mode: auto, md, json, plain")
	return cmd
}

func runQuery(cmd *cobra.Command, query string, limit int, output string) error {
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

	var queryVec []float32
	embedder := vc.embedder()
	if embedder.Available(ctx) {
		if vecs, err := embedder.Embed(ctx, []string{query}); err == nil && len(vecs) == 1 {
			queryVec = vecs[0]
		}
	}

	hits, err := store.Hybrid(ctx, query, queryVec, limit)
	if err != nil {
		return err
	}

	reranker := rerank.New(vc.Config.RerankerURL, vc.Config.RerankerModel)
	mode := "keyword"
	if queryVec != nil {
		mode = "hybrid"
	}
	if reranker.Enabled() {
		hits = reranker.Rerank(ctx, query, hits)
		mode += " + rerank"
	}

	if output == "json" {
		return emitJSON(cmd.OutOrStdout(), hits)
	}
	emitMarkdown(cmd.OutOrStdout(), renderHits(query, hits, mode), output)
	return nil
}

// renderHits formats search results as markdown.
func renderHits(query string, hits []index.Hit, mode string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %d results for \"%s\" (%s)\n\n", len(hits), query, mode)
	if len(hits) == 0 {
		b.WriteString("_No matches. Try different terms, or run `stardust index` if the vault is unindexed._\n")
		return b.String()
	}
	for i, h := range hits {
		title := h.Title
		if title == "" {
			title = h.Path
		}
		fmt.Fprintf(&b, "## %d. %s\n\n", i+1, title)
		b.WriteString("`" + h.Path + "`")
		if h.Heading != "" {
			b.WriteString(" - " + h.Heading)
		}
		b.WriteString("\n\n")
		b.WriteString(h.Snippet + "\n\n")
	}
	return b.String()
}
