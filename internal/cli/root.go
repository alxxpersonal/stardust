// Package cli wires the stardust cobra command tree over the core library.
//
// Each command is a thin frontend: it parses flags, calls into the internal
// packages that do the real work, and renders the result in the active output
// mode (interactive TUI, glamour-rendered markdown, plain markdown, or JSON).
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/alxxpersonal/stardust/internal/tui"
)

// version is the build version, overridable via -ldflags at build time.
var version = "0.1.0-dev"

// Execute builds the root command and runs it, exiting non-zero on error.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// newRootCmd assembles the stardust command tree.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "stardust",
		Short: "Local-first markdown context engine for AI agents",
		Long: "Stardust indexes a markdown vault into a derived, rebuildable search index\n" +
			"and exposes it to humans (TUI) and agents (CLI). Files stay the source of truth.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// no subcommand: launch the TUI for humans, print help when piped
			if !isTTY() {
				return cmd.Help()
			}
			vc, err := resolveVault()
			if err != nil {
				return err
			}
			return tui.Run(vc.Layout, vc.Config)
		},
	}
	root.AddCommand(
		newInitCmd(),
		newIndexCmd(),
		newQueryCmd(),
		newGraphCmd(),
		newArchiveCmd(),
		newServeCmd(),
		newCronCmd(),
		newHooksCmd(),
		newRebuildCmd(),
		newVersionCmd(),
	)
	return root
}

// newVersionCmd prints the build version.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the stardust version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "stardust", version)
			return nil
		},
	}
}
