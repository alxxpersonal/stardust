package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alxxpersonal/stardust/internal/index"
	"github.com/alxxpersonal/stardust/internal/service"
)

// newQueryCmd builds the search command.
func newQueryCmd() *cobra.Command {
	var limit int
	var output string
	var useMounts bool
	cmd := &cobra.Command{
		Use:   "query <text>",
		Short: "Search the vault (hybrid keyword + semantic)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQuery(cmd, strings.Join(args, " "), limit, output, useMounts)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "maximum number of results")
	cmd.Flags().StringVar(&output, "output", "auto", "output mode: auto, md, json, plain")
	cmd.Flags().BoolVar(&useMounts, "mounts", false, "also search configured mounts and fuse the results")
	return cmd
}

func runQuery(cmd *cobra.Command, query string, limit int, output string, useMounts bool) error {
	ctx := cmd.Context()
	svc, err := openService(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = svc.Close() }()

	if useMounts {
		fused, err := svc.QueryMounts(ctx, query, limit)
		if err != nil {
			return err
		}
		if output == "json" {
			return emitJSON(cmd.OutOrStdout(), fused)
		}
		emitMarkdown(cmd.OutOrStdout(), renderFused(query, fused), output)
		return nil
	}

	res, err := svc.Query(ctx, query, limit)
	if err != nil {
		return err
	}
	if output == "json" {
		return emitJSON(cmd.OutOrStdout(), res.Hits)
	}
	emitMarkdown(cmd.OutOrStdout(), renderHits(res.Query, res.Hits, res.Mode), output)
	return nil
}

// renderHits formats local search results as markdown.
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

// renderFused formats fused vault + mount results as markdown.
func renderFused(query string, hits []service.FusedHit) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %d results for \"%s\" (fused: vault + mounts)\n\n", len(hits), query)
	if len(hits) == 0 {
		b.WriteString("_No matches._\n")
		return b.String()
	}
	for i, h := range hits {
		title := h.Title
		if title == "" {
			title = h.Ref
		}
		fmt.Fprintf(&b, "## %d. %s\n\n", i+1, title)
		fmt.Fprintf(&b, "`%s` _(source: %s)_\n\n", h.Ref, h.Source)
		if h.Snippet != "" {
			b.WriteString(h.Snippet + "\n\n")
		}
	}
	return b.String()
}
