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
	var docs bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold .stardust/ in the current vault",
		Long:  "Creates the .stardust directory, default config, manifest, and an empty index.\nRun it from the vault root.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInit(cmd, docs)
		},
	}
	cmd.Flags().BoolVar(&docs, "docs", false, "scaffold the specs, plans, adr, and research docs collections")
	return cmd
}

func runInit(cmd *cobra.Command, docs bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working dir: %w", err)
	}
	if err := scaffoldVault(cmd.Context(), cwd, "off", docs); err != nil {
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
// with the given check mode. When docs is set it also writes the specs, plans,
// adr, and research docs collection configs. Shared by `init` and `new`.
func scaffoldVault(ctx context.Context, root, check string, docs bool) error {
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

	if docs {
		if err := writeDocsCollections(layout.Collections()); err != nil {
			return err
		}
	}

	if gitx.IsRepo(ctx, root) {
		if err := hooks.Install(ctx, root, layout.Hooks(), check); err != nil {
			return err
		}
	}
	return nil
}

// docsCollections holds the config.toml body for each docs collection the
// registry renders. Each maps a docs subfolder, declares a required title
// string, and a status enum with the convention's closed set. The order matches
// the registry's fixed group order: specs, plans, adr, research.
var docsCollections = []struct {
	name string
	body string
}{
	{
		name: "specs",
		body: "path = \"docs/specs\"\n" +
			"description = \"technical specs\"\n\n" +
			"[[fields]]\n" +
			"name = \"title\"\n" +
			"type = \"string\"\n" +
			"required = true\n\n" +
			"[[fields]]\n" +
			"name = \"status\"\n" +
			"type = \"enum\"\n" +
			"enum = [\"Draft\", \"In Review\", \"Approved\", \"Implemented\", \"Superseded\"]\n",
	},
	{
		name: "plans",
		body: "path = \"docs/plans\"\n" +
			"description = \"implementation plans\"\n\n" +
			"[[fields]]\n" +
			"name = \"title\"\n" +
			"type = \"string\"\n" +
			"required = true\n\n" +
			"[[fields]]\n" +
			"name = \"status\"\n" +
			"type = \"enum\"\n" +
			"enum = [\"Draft\", \"Active\", \"Done\", \"Abandoned\"]\n",
	},
	{
		name: "adr",
		body: "path = \"docs/adr\"\n" +
			"description = \"architecture decision records\"\n\n" +
			"[[fields]]\n" +
			"name = \"title\"\n" +
			"type = \"string\"\n" +
			"required = true\n\n" +
			"[[fields]]\n" +
			"name = \"status\"\n" +
			"type = \"enum\"\n" +
			"enum = [\"Proposed\", \"Accepted\", \"Deferred\", \"Rejected\", \"Superseded\"]\n",
	},
	{
		name: "research",
		body: "path = \"docs/research\"\n" +
			"description = \"research notes\"\n\n" +
			"[[fields]]\n" +
			"name = \"title\"\n" +
			"type = \"string\"\n" +
			"required = true\n\n" +
			"[[fields]]\n" +
			"name = \"status\"\n" +
			"type = \"enum\"\n" +
			"enum = [\"Active\", \"Archived\", \"Superseded\"]\n",
	},
}

// writeDocsCollections writes the four docs collection configs under
// collectionsDir. An existing config is left untouched so re-running init --docs
// never clobbers a customised schema.
func writeDocsCollections(collectionsDir string) error {
	for _, c := range docsCollections {
		dir := filepath.Join(collectionsDir, c.name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create collection %s: %w", c.name, err)
		}
		cfg := filepath.Join(dir, "config.toml")
		if _, err := os.Stat(cfg); err == nil {
			continue
		}
		if err := os.WriteFile(cfg, []byte(c.body), 0o644); err != nil {
			return fmt.Errorf("write collection %s: %w", c.name, err)
		}
	}
	return nil
}
