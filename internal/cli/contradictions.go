package cli

import (
	"github.com/spf13/cobra"

	"github.com/alxxpersonal/stardust/internal/service"
)

// newContradictionsCmd surfaces deterministic cross-note contradiction
// candidates as review prompts. It mirrors the digest command surface and is
// deliberately not a check kind: candidates are advisory and never gate CI.
func newContradictionsCmd() *cobra.Command {
	var since string
	var output string
	var advance bool
	var all bool
	var limit int
	cmd := &cobra.Command{
		Use:   "contradictions",
		Short: "Surface cross-note contradiction candidates as review prompts (never verdicts)",
		Long: "Prepares a short, high-precision list of candidate contradiction pairs from\n" +
			"recently changed notes: an assertion on one side and an opposing same-subject\n" +
			"line on another. The binary never judges whether a pair truly conflicts; each\n" +
			"candidate is a review prompt for an agent or a human to confirm. Uses git as\n" +
			"the change feed (since the last contradiction cursor, or --since / --all) and\n" +
			"hybrid retrieval for same-subject recall, announcing fts-only degradation when\n" +
			"the embedder is down. --advance moves the cursor so the next scan is incremental.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			svc, err := openService(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = svc.Close() }()

			res, err := svc.Contradictions(ctx, service.ContradictionOptions{
				Since:   since,
				All:     all,
				Advance: advance,
				Limit:   limit,
			})
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
	cmd.Flags().StringVar(&since, "since", "", "git SHA to diff the A-side from (default: the last contradiction cursor)")
	cmd.Flags().BoolVar(&advance, "advance", false, "advance the contradiction cursor to HEAD")
	cmd.Flags().BoolVar(&all, "all", false, "sweep every tracked note as the A-side (a full audit; costs O(notes) recalls)")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum number of candidates (default: a small top-N)")
	cmd.Flags().StringVar(&output, "output", "auto", "output mode: auto, md, json, plain")
	return cmd
}
