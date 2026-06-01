package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// newArchiveCmd snapshots the vault's git history into a destination folder.
func newArchiveCmd() *cobra.Command {
	var dest string
	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Snapshot the vault's git history into a destination folder",
		Long:  "Creates a timestamped bare mirror of the vault's full git history (a clone you\ncan restore from) under --dest. The local-first leg of the backup story.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runArchive(cmd, dest)
		},
	}
	cmd.Flags().StringVar(&dest, "dest", "", "destination folder (default: .stardust/archives)")
	return cmd
}

func runArchive(cmd *cobra.Command, dest string) error {
	ctx := cmd.Context()
	svc, err := openService(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = svc.Close() }()

	target, err := svc.Archive(ctx, dest)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "archived git history to %s\n", target)
	return nil
}
