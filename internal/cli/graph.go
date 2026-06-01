package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alxxpersonal/stardust/internal/graph"
)

// newGraphCmd builds the link-graph command.
func newGraphCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Derive the link graph and report orphans + broken links",
		Long:  "Builds the wikilink graph from markdown, writes it to .stardust/cache/graph.json,\nand reports orphan notes and broken links.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runGraph(cmd, output)
		},
	}
	cmd.Flags().StringVar(&output, "output", "auto", "output mode: auto, md, json, plain")
	return cmd
}

func runGraph(cmd *cobra.Command, output string) error {
	vc, err := resolveVault()
	if err != nil {
		return err
	}
	g, err := graph.Build(vc.Layout.Root, vc.Config.Ignore)
	if err != nil {
		return err
	}
	if err := g.Save(vc.Layout.GraphJSON()); err != nil {
		return err
	}

	orphans := g.Orphans()
	broken := g.BrokenLinks()

	if output == "json" {
		return emitJSON(cmd.OutOrStdout(), map[string]any{
			"notes":   len(g.Nodes),
			"links":   g.EdgeCount(),
			"orphans": orphans,
			"broken":  broken,
		})
	}
	emitMarkdown(cmd.OutOrStdout(), renderGraph(g, orphans, broken), output)
	return nil
}

func renderGraph(g *graph.Graph, orphans []string, broken []graph.BrokenLink) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Link graph\n\n%d notes, %d links.\n\n", len(g.Nodes), g.EdgeCount())

	fmt.Fprintf(&b, "## Orphans (%d)\n\n", len(orphans))
	if len(orphans) == 0 {
		b.WriteString("_None. Every note is linked._\n\n")
	} else {
		for _, p := range orphans {
			b.WriteString("- `" + p + "`\n")
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "## Broken links (%d)\n\n", len(broken))
	if len(broken) == 0 {
		b.WriteString("_None. Every wikilink resolves._\n")
	} else {
		for _, bl := range broken {
			fmt.Fprintf(&b, "- `%s` -> [[%s]]\n", bl.From, bl.Target)
		}
	}
	return b.String()
}
