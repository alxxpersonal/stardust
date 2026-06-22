package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newCheckCmd validates vault integrity.
func newCheckCmd() *cobra.Command {
	var strict bool
	var output string
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Validate vault integrity and docs conventions",
		Long:  "Reports broken wikilinks, malformed frontmatter, convention errors, and agent target issues.\nWith --strict it exits non-zero when there are errors, so a pre-commit hook can gate commits on it.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			svc, err := openService(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = svc.Close() }()

			res, err := svc.Check(ctx)
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
			if strict && res.Errors > 0 {
				return fmt.Errorf("%d vault error(s)", res.Errors)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "exit non-zero if there are errors")
	cmd.Flags().StringVar(&output, "output", "auto", "output mode: auto, md, json, plain")
	return cmd
}
