package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/alxxpersonal/stardust/internal/clierr"
)

// newCheckCmd validates vault integrity.
func newCheckCmd() *cobra.Command {
	var strict bool
	var fix bool
	var ci bool
	var updateBaseline bool
	var output string
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Validate vault integrity and docs conventions",
		Long:  "Reports broken wikilinks, malformed frontmatter, convention errors, and agent target issues.\nWith --strict it exits non-zero when there are errors, so a pre-commit hook can gate commits on it.\nWith --fix it autofixes the mechanically-safe doc issues (forbidden dashes, missing or wrong type, missing created/updated) before reporting; it runs before --strict so the gate sees only the remainder.\nWith --ci it reports and exits non-zero only on issues absent from .stardust/baseline.json, so a backlogged repo adopts the gate green. With --update-baseline it snapshots the current issue set into that baseline.",
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

			if updateBaseline {
				base, err := svc.UpdateBaseline(ctx)
				if err != nil {
					return err
				}
				if output == "json" {
					return emitJSON(cmd.OutOrStdout(), base)
				}
				emitMarkdown(cmd.OutOrStdout(), fmt.Sprintf("# Baseline updated\n\n%d issue(s) snapshotted to .stardust/baseline.json.\n", len(base.Issues)), output)
				return nil
			}

			if ci {
				ciRes, err := svc.CheckCI(ctx)
				if err != nil {
					return err
				}
				if output == "json" {
					if err := emitJSON(cmd.OutOrStdout(), ciRes); err != nil {
						return err
					}
				} else {
					emitMarkdown(cmd.OutOrStdout(), ciRes.Markdown, output)
				}
				if ciRes.NewErrors > 0 {
					return clierr.New(fmt.Sprintf("%d new vault error(s) since baseline", ciRes.NewErrors), "stardust check --update-baseline")
				}
				return nil
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
				return clierr.New(fmt.Sprintf("%d vault error(s)", res.Errors), "stardust check --fix")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "exit non-zero if there are errors")
	cmd.Flags().BoolVar(&fix, "fix", false, "autofix mechanically-safe doc issues before reporting")
	cmd.Flags().BoolVar(&ci, "ci", false, "report and exit non-zero only on issues new since .stardust/baseline.json")
	cmd.Flags().BoolVar(&updateBaseline, "update-baseline", false, "snapshot the current issue set into .stardust/baseline.json")
	cmd.Flags().StringVar(&output, "output", "auto", "output mode: auto, md, json, plain")
	return cmd
}
