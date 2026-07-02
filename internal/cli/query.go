package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alxxpersonal/stardust/internal/rerank"
	"github.com/alxxpersonal/stardust/internal/service"
)

// bareMountsSentinel is the value cobra assigns to --mounts when it is passed
// without a value (a null byte never collides with a real mount name). It means
// "search all mounts"; --mounts=a,b instead scopes to the named mounts.
const bareMountsSentinel = "\x00"

// newQueryCmd builds the search command.
func newQueryCmd() *cobra.Command {
	var limit int
	var output string
	var mountsFlag string
	cmd := &cobra.Command{
		Use:   "query <text>",
		Short: "Search the vault (hybrid keyword + semantic)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			useMounts := cmd.Flags().Changed("mounts")
			var scope []string
			if useMounts && mountsFlag != bareMountsSentinel {
				scope = splitScope(mountsFlag)
			}
			return runQuery(cmd, strings.Join(args, " "), limit, output, useMounts, scope)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "maximum number of results")
	cmd.Flags().StringVar(&output, "output", "auto", "output mode: auto, md, json, plain")
	cmd.Flags().StringVar(&mountsFlag, "mounts", "", "also search configured mounts and fuse; optionally scope to a comma list, e.g. --mounts=notion,zotero")
	cmd.Flags().Lookup("mounts").NoOptDefVal = bareMountsSentinel
	return cmd
}

// splitScope parses a comma-separated --mounts value into trimmed, non-empty
// mount names.
func splitScope(s string) []string {
	out := make([]string, 0)
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func runQuery(cmd *cobra.Command, query string, limit int, output string, useMounts bool, scope []string) error {
	ctx := cmd.Context()
	svc, err := openService(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = svc.Close() }()

	if useMounts {
		res, err := svc.QueryMounts(ctx, query, limit, scope)
		if err != nil {
			return err
		}
		if output == "json" {
			return emitJSON(cmd.OutOrStdout(), res)
		}
		emitMarkdown(cmd.OutOrStdout(), renderFused(res), output)
		return nil
	}

	res, err := svc.Query(ctx, query, limit)
	if err != nil {
		return err
	}
	if output == "json" {
		return emitJSON(cmd.OutOrStdout(), res.Hits)
	}
	emitMarkdown(cmd.OutOrStdout(), renderHits(res), output)
	return nil
}

// renderHits formats local search results as markdown, with a reranker line in
// the header announcing the active source when a reranker ran.
func renderHits(res service.QueryResult) string {
	query, hits, mode := res.Query, res.Hits, res.Mode
	var b strings.Builder
	fmt.Fprintf(&b, "# %d results for \"%s\" (%s)\n\n", len(hits), query, mode)
	if line := rerankLine(res); line != "" {
		b.WriteString(line + "\n\n")
	}
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

// rerankLine renders the one-line reranker announcement for the results header.
// It surfaces where a reranker came from when one ran (configured or discovered)
// and stays silent when reranking is off, so a plain no-runtime query renders as
// terse as before discovery existed. The off state is announced by stardust
// status and the machine-readable rerank_source field.
func rerankLine(res service.QueryResult) string {
	switch res.RerankSource {
	case string(rerank.SourceConfigured):
		return "_reranker: configured endpoint_"
	case string(rerank.SourceDiscovered):
		return "_reranker: discovered local runtime_"
	default:
		return ""
	}
}

// renderFused formats fused vault + mount results as markdown, with a routing
// line in the header mirroring how renderHits surfaces the retrieval mode.
func renderFused(res service.MountQueryResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %d results for \"%s\" (fused: vault + mounts)\n\n", len(res.Hits), res.Query)
	if line := routingLine(res); line != "" {
		b.WriteString(line + "\n\n")
	}
	if len(res.Hits) == 0 {
		b.WriteString("_No matches._\n")
		return b.String()
	}
	for i, h := range res.Hits {
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

// routingLine renders the one-line routing note for the fused header. It stays
// silent when routing did not engage (mode all) so single-mount, no-mount, and
// metadata-less workspaces render byte-identically to before routing existed.
func routingLine(res service.MountQueryResult) string {
	if res.RoutingMode == "" || res.RoutingMode == service.RoutingAll {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "_routing: %s", res.RoutingMode)
	if len(res.MountsSearched) > 0 {
		fmt.Fprintf(&b, " - searched %s", strings.Join(res.MountsSearched, ", "))
	}
	if len(res.MountsSkipped) > 0 {
		fmt.Fprintf(&b, "; skipped %s", strings.Join(res.MountsSkipped, ", "))
	}
	if res.RoutingReason != "" {
		fmt.Fprintf(&b, " (%s)", res.RoutingReason)
	}
	b.WriteString("_")
	return b.String()
}
