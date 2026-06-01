package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alxxpersonal/stardust/internal/service"
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
	ctx := cmd.Context()
	svc, err := openService(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = svc.Close() }()

	rep, err := svc.Graph(ctx)
	if err != nil {
		return err
	}
	if output == "json" {
		return emitJSON(cmd.OutOrStdout(), rep)
	}
	emitMarkdown(cmd.OutOrStdout(), renderGraph(rep), output)
	return nil
}

func renderGraph(rep service.GraphReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Link graph\n\n%d notes, %d links.\n\n", rep.Notes, rep.Links)

	fmt.Fprintf(&b, "## Orphans (%d)\n\n", len(rep.Orphans))
	if len(rep.Orphans) == 0 {
		b.WriteString("_None. Every note is linked._\n\n")
	} else {
		for _, p := range rep.Orphans {
			b.WriteString("- `" + p + "`\n")
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "## Broken links (%d)\n\n", len(rep.Broken))
	if len(rep.Broken) == 0 {
		b.WriteString("_None. Every wikilink resolves._\n")
	} else {
		for _, bl := range rep.Broken {
			fmt.Fprintf(&b, "- `%s` -> [[%s]]\n", bl.From, bl.Target)
		}
	}
	return b.String()
}
