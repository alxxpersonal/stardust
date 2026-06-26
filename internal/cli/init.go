package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/convention"
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
	res, err := scaffoldVault(cmd.Context(), cwd, "off", docs)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Initialised .stardust/ in %s\n", cwd)
	if gitx.IsRepo(cmd.Context(), cwd) {
		if res.Composed() {
			fmt.Fprintf(out, "Composed commit hooks into %s.\n", res.Target)
		} else {
			fmt.Fprintf(out, "Wired commit hooks (owned: %s).\n", res.Target)
		}
	}
	fmt.Fprintln(out, "Next: run `stardust index` to build the search index.")
	return nil
}

// scaffoldVault creates the .stardust layout (dirs, config, manifest, INDEX,
// cache .gitignore) under root and, when root is a git repo, installs the hooks
// with the given check mode. When docs is set it also writes the specs, plans,
// adr, and research docs collection configs. Shared by `init` and `new`. The
// returned hooks.Result names which install path was taken; it is the zero value
// when root is not a git repo.
func scaffoldVault(ctx context.Context, root, check string, docs bool) (hooks.Result, error) {
	layout := config.Layout{Root: root}

	for _, dir := range []string{layout.Dir(), layout.Cache(), layout.Hooks(), layout.CronJobs()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return hooks.Result{}, fmt.Errorf("create %s: %w", dir, err)
		}
	}

	if _, err := os.Stat(layout.Config()); os.IsNotExist(err) {
		if err := config.Save(layout.Config(), config.Default()); err != nil {
			return hooks.Result{}, err
		}
	}

	// keep the rebuildable cache out of git
	if err := os.WriteFile(filepath.Join(layout.Dir(), ".gitignore"), []byte("cache/\n"), 0o644); err != nil {
		return hooks.Result{}, fmt.Errorf("write .stardust/.gitignore: %w", err)
	}

	if err := manifest.WriteManifest(layout.Manifest(), filepath.Base(root)); err != nil {
		return hooks.Result{}, err
	}
	if err := manifest.WriteIndex(layout.IndexMD(), nil); err != nil {
		return hooks.Result{}, err
	}

	if docs {
		if err := writeDocsCollections(layout.Collections()); err != nil {
			return hooks.Result{}, err
		}
	}

	if gitx.IsRepo(ctx, root) {
		return hooks.Install(ctx, root, layout.Hooks(), check)
	}
	return hooks.Result{}, nil
}

// writeDocsCollections writes the four docs collection configs under
// collectionsDir. An existing config is left untouched so re-running init --docs
// never clobbers a customised schema.
func writeDocsCollections(collectionsDir string) error {
	for _, c := range convention.DefaultDocCollections() {
		dir := filepath.Join(collectionsDir, c.Name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create collection %s: %w", c.Name, err)
		}
		cfg := filepath.Join(dir, "config.toml")
		if _, err := os.Stat(cfg); err == nil {
			continue
		}
		if err := os.WriteFile(cfg, []byte(docCollectionConfig(c)), 0o644); err != nil {
			return fmt.Errorf("write collection %s: %w", c.Name, err)
		}
	}
	return nil
}

// docCollectionConfig codegens a collection's config.toml from its declarative
// schema (convention.DocCollection.Fields), so the scaffolder and the checker
// read the exact same field set. Each field becomes a [[fields]] table; an enum
// line is emitted only for enum fields.
func docCollectionConfig(c convention.DocCollection) string {
	var b strings.Builder
	fmt.Fprintf(&b, "path = %q\n", c.Path)
	fmt.Fprintf(&b, "description = %q\n", c.Description)
	for _, f := range c.Fields() {
		b.WriteString("\n[[fields]]\n")
		fmt.Fprintf(&b, "name = %q\n", f.Name)
		fmt.Fprintf(&b, "type = %q\n", f.Type)
		fmt.Fprintf(&b, "required = %t\n", f.Required)
		if len(f.Enum) > 0 {
			fmt.Fprintf(&b, "enum = [%s]\n", quoteList(f.Enum))
		}
	}
	return b.String()
}

func quoteList(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = fmt.Sprintf("%q", item)
	}
	return strings.Join(quoted, ", ")
}
