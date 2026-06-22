package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/alxxpersonal/stardust/internal/manifest"
)

// registryOrder is the fixed collection order for the docs registry. Each name
// renders as one section; a missing or empty collection renders an empty
// section rather than erroring.
var registryOrder = []string{"specs", "plans", "adr", "research"}

// newRegistryCmd builds the command that renders the grouped docs registry.
func newRegistryCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "registry",
		Short: "Render the grouped docs registry from collections",
		Long: "Queries the docs collections (specs, plans, adr, research) and renders a\n" +
			"grouped, status-aware markdown index. The output is regenerated, never\n" +
			"hand-edited. Missing or empty collections render empty sections.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runRegistry(cmd, output)
		},
	}
	cmd.Flags().StringVar(&output, "output", "docs/INDEX.md", "registry output path, relative to the vault root")
	cmd.AddCommand(newRegistryGovernsCmd())
	return cmd
}

func newRegistryGovernsCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "governs <path>",
		Short: "Show docs that govern a code path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			svc, err := openService(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = svc.Close() }()

			res, err := svc.GoverningDocs(ctx, args[0])
			if err != nil {
				return err
			}
			if output == "json" {
				return emitJSON(cmd.OutOrStdout(), res)
			}
			emitMarkdown(cmd.OutOrStdout(), res.Markdown, output)
			return nil
		},
	}
	cmd.Flags().StringVar(&output, "output", "auto", "output mode: auto, md, json, plain")
	return cmd
}

func runRegistry(cmd *cobra.Command, output string) error {
	ctx := cmd.Context()
	svc, err := openService(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = svc.Close() }()

	groups, err := svc.Registry(registryOrder)
	if err != nil {
		return err
	}

	out := output
	if !filepath.IsAbs(out) {
		out = filepath.Join(svc.Layout.Root, out)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("create registry dir: %w", err)
	}
	if err := manifest.WriteRegistry(out, groups); err != nil {
		return err
	}
	if err := svc.RefreshManifest(ctx); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStderr(), "wrote registry", out)
	return nil
}
