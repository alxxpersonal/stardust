package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

// newBundleCmd assembles a task-scoped context bundle.
func newBundleCmd() *cobra.Command {
	var budget int
	var output string
	cmd := &cobra.Command{
		Use:   "bundle <task>",
		Short: "Assemble a task-scoped context bundle for an agent",
		Long:  "Seeds from hybrid recall, expands over the link graph with personalized\nPageRank, fuses with RRF, and packs the most relevant notes to a token budget.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			svc, err := openService(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = svc.Close() }()

			res, err := svc.Bundle(ctx, strings.Join(args, " "), budget)
			if err != nil {
				return err
			}
			if output == "json" {
				return emitJSON(cmd.OutOrStdout(), res)
			}
			emitMarkdown(cmd.OutOrStdout(), res.Markdown, output)
			return nil
		},
	}
	cmd.Flags().IntVar(&budget, "budget", 4000, "approximate token budget for the bundle")
	cmd.Flags().StringVar(&output, "output", "auto", "output mode: auto, md, json, plain")
	return cmd
}
