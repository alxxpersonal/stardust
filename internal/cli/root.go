// Package cli wires the stardust cobra command tree over the core library.
//
// Each command is a thin frontend: it parses flags, calls into the internal
// packages that do the real work, and renders the result in the active output
// mode (interactive TUI, glamour-rendered markdown, plain markdown, or JSON).
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	fang "charm.land/fang/v2"
	"github.com/spf13/cobra"

	"github.com/alxxpersonal/stardust/internal/tui"
)

// devVersion is the unset placeholder version. When the build version still
// equals it, fang falls back to debug.ReadBuildInfo (so go install builds
// report a real version).
const devVersion = "0.2.0-dev"

// version is the build version, overridable via -ldflags at build time.
var version = devVersion

// commit is the short build commit SHA, overridable via -ldflags at build time.
var commit = ""

// Execute builds the root command and runs it through fang, exiting non-zero on
// error. fang owns help, usage, and error rendering; the cosmic theme and the
// hint error handler are wired here. Data output stays on cmd.OutOrStdout and is
// never routed through fang.
func Execute() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

// run wires the signal-aware context and runs the cobra tree through fang,
// returning any error so the defer that releases the signal handler still runs
// before Execute decides the exit code.
func run() error {
	root := newRootCmd()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return fang.Execute(
		ctx,
		root,
		versionOpt(),
		fang.WithCommit(commit),
		fang.WithColorSchemeFunc(cosmicColorScheme),
		fang.WithErrorHandler(cosmicErrorHandler),
		fang.WithNotifySignal(os.Interrupt, syscall.SIGTERM),
	)
}

// versionOpt sets the fang version from the build-time version, or passes an
// empty string when the version is still the dev placeholder so fang falls back
// to debug.ReadBuildInfo.
func versionOpt() fang.Option {
	if version == devVersion {
		return fang.WithVersion("")
	}
	return fang.WithVersion(version)
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
		newStatusCmd(),
		newIndexCmd(),
		newQueryCmd(),
		newGraphCmd(),
		newCheckCmd(),
		newIndexesCmd(),
		newBundleCmd(),
		newRememberCmd(),
		newRegistryCmd(),
		newSyncCmd(),
		newDigestCmd(),
		newNewCmd(),
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
