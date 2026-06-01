package cli

import (
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
	layout := config.Layout{Root: cwd}

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

	vaultName := filepath.Base(cwd)
	if err := manifest.WriteManifest(layout.Manifest(), vaultName); err != nil {
		return err
	}
	if err := manifest.WriteIndex(layout.IndexMD(), nil); err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Initialised .stardust/ in %s\n", cwd)

	if gitx.IsRepo(cmd.Context(), cwd) {
		if err := hooks.Install(cmd.Context(), cwd, layout.Hooks()); err != nil {
			return err
		}
		fmt.Fprintln(out, "Wired commit hooks (core.hooksPath -> .stardust/hooks).")
	}

	fmt.Fprintln(out, "Next: run `stardust index` to build the search index.")
	return nil
}
