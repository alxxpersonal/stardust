package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/alxxpersonal/stardust/internal/gitx"
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
	vc, err := resolveVault()
	if err != nil {
		return err
	}
	if !gitx.IsRepo(ctx, vc.Layout.Root) {
		return fmt.Errorf("archive: %s is not a git repository", vc.Layout.Root)
	}
	if dest == "" {
		dest = filepath.Join(vc.Layout.Dir(), "archives")
	}
	target, err := gitx.Archive(ctx, vc.Layout.Root, dest)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "archived git history to %s\n", target)
	return nil
}
