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
			ctx := cmd.Context()
			svc, err := openService(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = svc.Close() }()

			fmt.Fprintln(os.Stderr, "cache cleared, rebuilding from markdown...")
			stats, err := svc.Rebuild(ctx)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "indexed %d, skipped %d, deleted %d (vectors: %t)\n",
				stats.Indexed, stats.Skipped, stats.Deleted, stats.Vectors)
			return nil
		},
	}
}
