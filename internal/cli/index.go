package cli

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
)

// newIndexCmd builds the incremental indexer command.
func newIndexCmd() *cobra.Command {
	var since string
	var background bool
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Index changed notes into the search index",
		Long:  "Incrementally indexes the vault. With git it diffs from the last indexed\ncommit; otherwise it scans the tree. Unchanged notes are skipped by content hash.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if background {
				return spawnBackgroundIndex(since)
			}
			return runIndex(cmd, since)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "only index notes changed since this git SHA")
	cmd.Flags().BoolVar(&background, "background", false, "detach and index in the background")
	return cmd
}

// spawnBackgroundIndex re-execs stardust index detached, for the commit hook.
func spawnBackgroundIndex(since string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	args := []string{"index"}
	if since != "" {
		args = append(args, "--since", since)
	}
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start background index: %w", err)
	}
	return cmd.Process.Release()
}

func runIndex(cmd *cobra.Command, since string) error {
	ctx := cmd.Context()
	svc, err := openService(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = svc.Close() }()

	stats, err := svc.Index(ctx, since)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "indexed %d, skipped %d, deleted %d (vectors: %t)\n",
		stats.Indexed, stats.Skipped, stats.Deleted, stats.Vectors)
	return nil
}
