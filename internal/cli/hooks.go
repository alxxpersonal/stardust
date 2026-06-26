package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/alxxpersonal/stardust/internal/clierr"
	"github.com/alxxpersonal/stardust/internal/gitx"
	"github.com/alxxpersonal/stardust/internal/hooks"
)

// newHooksCmd manages the git commit hooks that keep the index fresh.
func newHooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hooks",
		Short: "Manage git hooks that auto-index on commit",
	}
	var check string
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Wire commit hooks to auto-index the vault (and optionally gate commits on `check`)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			vc, err := resolveVault()
			if err != nil {
				return err
			}
			if !gitx.IsRepo(cmd.Context(), vc.Layout.Root) {
				return clierr.New(vc.Layout.Root+" is not a git repository", "git init")
			}
			res, err := hooks.Install(cmd.Context(), vc.Layout.Root, vc.Layout.Hooks(), check)
			if err != nil {
				return err
			}
			if res.Composed() {
				fmt.Fprintf(cmd.ErrOrStderr(), "installed commit hooks (composed into %s, check: %s)\n", res.Target, check)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "installed commit hooks (owned: %s, check: %s)\n", res.Target, check)
			}
			return nil
		},
	}
	installCmd.Flags().StringVar(&check, "check", "off", "pre-commit vault check: off, warn, strict")
	cmd.AddCommand(
		installCmd,
		&cobra.Command{
			Use:   "uninstall",
			Short: "Remove the commit-hook wiring",
			RunE: func(cmd *cobra.Command, _ []string) error {
				vc, err := resolveVault()
				if err != nil {
					return err
				}
				if err := hooks.Uninstall(cmd.Context(), vc.Layout.Root); err != nil {
					return err
				}
				fmt.Fprintln(cmd.ErrOrStderr(), "removed commit-hook wiring")
				return nil
			},
		},
	)
	return cmd
}
