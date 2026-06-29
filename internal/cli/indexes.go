package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/alxxpersonal/stardust/internal/clierr"
)

// newIndexesCmd builds the command that syncs or checks configured directory indexes.
func newIndexesCmd() *cobra.Command {
	var check bool
	var output string
	cmd := &cobra.Command{
		Use:   "indexes",
		Short: "Maintain configured per-directory indexes",
		Long: "Syncs opt-in per-directory INDEX.md files configured under\n" +
			"[conventions.directory_indexes]. With --check it reports missing or\n" +
			"stale directory indexes without writing.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			svc, err := openService(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = svc.Close() }()

			if check {
				res, err := svc.CheckDirectoryIndexes(ctx)
				if err != nil {
					return err
				}
				if output == "json" {
					if err := emitJSON(cmd.OutOrStdout(), res); err != nil {
						return err
					}
				} else {
					emitMarkdown(cmd.OutOrStdout(), res.Markdown, output)
				}
				if len(res.Issues) > 0 {
					return clierr.New(fmt.Sprintf("%d directory index issue(s)", len(res.Issues)), "stardust indexes")
				}
				return nil
			}

			res, err := svc.SyncDirectoryIndexes(ctx)
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
	cmd.Flags().BoolVar(&check, "check", false, "check directory indexes without writing")
	cmd.Flags().StringVar(&output, "output", "auto", "output mode: auto, md, json, plain")
	return cmd
}
