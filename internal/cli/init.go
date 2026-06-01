package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/gitx"
	"github.com/alxxpersonal/stardust/internal/hooks"
	"github.com/alxxpersonal/stardust/internal/manifest"
)

// newInitCmd scaffolds .stardust/ in the current directory.
func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Scaffold .stardust/ in the current vault",
		Long:  "Creates the .stardust directory, default config, manifest, and an empty index.\nRun it from the vault root.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInit(cmd)
		},
	}
}

func runInit(cmd *cobra.Command) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working dir: %w", err)
	}
	if err := scaffoldVault(cmd.Context(), cwd, "off"); err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Initialised .stardust/ in %s\n", cwd)
	if gitx.IsRepo(cmd.Context(), cwd) {
		fmt.Fprintln(out, "Wired commit hooks (core.hooksPath -> .stardust/hooks).")
	}
	fmt.Fprintln(out, "Next: run `stardust index` to build the search index.")
	return nil
}

// scaffoldVault creates the .stardust layout (dirs, config, manifest, INDEX,
// cache .gitignore) under root and, when root is a git repo, installs the hooks
// with the given check mode. Shared by `init` and `new`.
func scaffoldVault(ctx context.Context, root, check string) error {
	layout := config.Layout{Root: root}

	for _, dir := range []string{layout.Dir(), layout.Cache(), layout.Hooks(), layout.CronJobs()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}

	if _, err := os.Stat(layout.Config()); os.IsNotExist(err) {
		if err := config.Save(layout.Config(), config.Default()); err != nil {
			return err
		}
	}

	// keep the rebuildable cache out of git
	if err := os.WriteFile(filepath.Join(layout.Dir(), ".gitignore"), []byte("cache/\n"), 0o644); err != nil {
		return fmt.Errorf("write .stardust/.gitignore: %w", err)
	}

	if err := manifest.WriteManifest(layout.Manifest(), filepath.Base(root)); err != nil {
		return err
	}
	if err := manifest.WriteIndex(layout.IndexMD(), nil); err != nil {
		return err
	}

	if gitx.IsRepo(ctx, root) {
		if err := hooks.Install(ctx, root, layout.Hooks(), check); err != nil {
			return err
		}
	}
	return nil
}
