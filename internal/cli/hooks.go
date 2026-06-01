package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

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
				return fmt.Errorf("hooks: %s is not a git repository", vc.Layout.Root)
			}
			if err := hooks.Install(cmd.Context(), vc.Layout.Root, vc.Layout.Hooks(), check); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "installed commit hooks (core.hooksPath -> .stardust/hooks, check: %s)\n", check)
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
				fmt.Fprintln(os.Stderr, "removed commit-hook wiring")
				return nil
			},
		},
	)
	return cmd
}
