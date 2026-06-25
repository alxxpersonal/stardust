package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alxxpersonal/stardust/internal/gitx"
	"github.com/alxxpersonal/stardust/internal/service"
)

// newNewCmd scaffolds a fresh vault.
func newNewCmd() *cobra.Command {
	var template string
	var check string
	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Scaffold a fresh vault (git init, .stardust, hooks, starter files, first commit)",
		Long:  "Bootstraps a new vault directory: starter files (or --template), git init,\n.stardust scaffolding with hooks, and an initial commit. Ready for `stardust index`.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNew(cmd, args[0], template, check)
		},
	}
	cmd.Flags().StringVar(&template, "template", "", "directory of starter files to copy into the new vault")
	cmd.Flags().StringVar(&check, "check", "warn", "pre-commit vault check: off, warn, strict")
	cmd.AddCommand(newDocCmd("spec"), newDocCmd("plan"), newDocCmd("adr"))
	return cmd
}

func newDocCmd(kind string) *cobra.Command {
	var status string
	var related []string
	var governs []string
	cmd := &cobra.Command{
		Use:   kind + " <title>",
		Short: "Create a new " + kind + " doc",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			svc, err := openService(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = svc.Close() }()

			res, err := svc.NewDoc(ctx, service.NewDocOptions{
				Kind:    kind,
				Title:   strings.TrimSpace(args[0]),
				Status:  status,
				Related: related,
				Governs: governs,
			})
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), res.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "doc status")
	cmd.Flags().StringArrayVar(&related, "related", nil, "related doc path")
	if kind == "spec" {
		cmd.Flags().StringArrayVar(&governs, "governs", nil, "repo-relative governed path glob")
	}
	return cmd
}

func runNew(cmd *cobra.Command, name, template, check string) error {
	ctx := cmd.Context()
	root, err := filepath.Abs(name)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", name, err)
	}
	if entries, err := os.ReadDir(root); err == nil && len(entries) > 0 {
		return fmt.Errorf("new: %s already exists and is not empty", root)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", root, err)
	}

	if template != "" {
		if err := copyDir(template, root); err != nil {
			return fmt.Errorf("copy template: %w", err)
		}
	} else if err := writeSkeleton(root); err != nil {
		return err
	}

	if err := gitx.Init(ctx, root); err != nil {
		return err
	}
	if _, err := scaffoldVault(ctx, root, check, false); err != nil {
		return err
	}
	if err := gitx.CommitAll(ctx, root, "initial vault"); err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Created vault %s\n", root)
	fmt.Fprintf(out, "Next: cd %s, add notes, run `stardust index`.\n", name)
	return nil
}

// writeSkeleton writes a minimal, generic starter vault.
func writeSkeleton(root string) error {
	base := filepath.Base(root)
	files := map[string]string{
		"README.md":        "# " + base + "\n\nA Stardust vault. Run `stardust index`, then `stardust query \"...\"`.\n",
		"AGENTS.md":        "# Agent guide\n\nThis is a Stardust vault. To search it, run `stardust query \"<question>\"`.\nSee `.stardust/manifest.md` for the pinned context and `.stardust/INDEX.md` for the table of contents.\n",
		".gitignore":       ".stardust/cache/\n.DS_Store\n",
		"notes/welcome.md": "---\ntitle: Welcome\ntags: [meta]\n---\n# Welcome\n\nThis is a Stardust vault. Notes are plain markdown with [[ideas]] wikilinks.\nRun `stardust query` to search.\n",
		"notes/ideas.md":   "---\ntitle: Ideas\n---\n# Ideas\n\nA place for ideas, linked from [[welcome]].\n",
	}
	for rel, content := range files {
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return fmt.Errorf("create dir for %s: %w", rel, err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", rel, err)
		}
	}
	return nil
}

// copyDir recursively copies the contents of src into dst.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
}
