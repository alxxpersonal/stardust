package cli

import (
	"github.com/spf13/cobra"
)

// newDigestCmd summarizes recent vault activity.
func newDigestCmd() *cobra.Command {
	var since string
	var output string
	var advance bool
	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Summarize recent vault activity, grouped by area, with open commitments",
		Long:  "Uses git as the change feed: what changed since the last digest cursor (or\n--since), grouped by top-level area, plus surfaced TODO/commitment lines.\n--advance moves the cursor so the next digest is incremental.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			svc, err := openService(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = svc.Close() }()

			res, err := svc.Digest(ctx, since, advance)
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
	cmd.Flags().StringVar(&since, "since", "", "git SHA to diff from (default: the last digest cursor)")
	cmd.Flags().BoolVar(&advance, "advance", false, "advance the digest cursor to HEAD")
	cmd.Flags().StringVar(&output, "output", "auto", "output mode: auto, md, json, plain")
	return cmd
}
