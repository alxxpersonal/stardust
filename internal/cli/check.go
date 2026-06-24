package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newCheckCmd validates vault integrity.
func newCheckCmd() *cobra.Command {
	var strict bool
	var fix bool
	var output string
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Validate vault integrity and docs conventions",
		Long:  "Reports broken wikilinks, malformed frontmatter, convention errors, and agent target issues.\nWith --strict it exits non-zero when there are errors, so a pre-commit hook can gate commits on it.\nWith --fix it autofixes the mechanically-safe doc issues (forbidden dashes, missing or wrong type, missing created/updated) before reporting; it runs before --strict so the gate sees only the remainder.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			svc, err := openService(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = svc.Close() }()

			if fix {
				fixRes, err := svc.CheckFix(ctx)
				if err != nil {
					return err
				}
				if output == "json" {
					if err := emitJSON(cmd.OutOrStdout(), fixRes); err != nil {
						return err
					}
				} else {
					emitMarkdown(cmd.OutOrStdout(), fixRes.Markdown, output)
				}
			}

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
	cmd.Flags().BoolVar(&fix, "fix", false, "autofix mechanically-safe doc issues before reporting")
	cmd.Flags().StringVar(&output, "output", "auto", "output mode: auto, md, json, plain")
	return cmd
}
