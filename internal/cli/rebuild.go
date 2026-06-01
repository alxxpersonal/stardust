package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// newRebuildCmd deletes the derived cache and reindexes from scratch.
func newRebuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rebuild",
		Short: "Delete and regenerate the entire index cache",
		Long:  "Nukes .stardust/cache and reindexes the whole vault from markdown. The cache\nis a derived artifact, so this is always safe.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			vc, err := resolveVault()
			if err != nil {
				return err
			}
			if err := os.RemoveAll(vc.Layout.Cache()); err != nil {
				return fmt.Errorf("clear cache: %w", err)
			}
			fmt.Fprintln(os.Stderr, "cache cleared, rebuilding from markdown...")
			return runIndex(cmd, "")
		},
	}
}
