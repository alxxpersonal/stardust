package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alxxpersonal/stardust/internal/agentsync"
)

// newSyncCmd builds the agent asset sync command.
func newSyncCmd() *cobra.Command {
	var configPath string
	var scope string
	var tools []string
	var output string
	opts := agentsync.Options{}

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync Stardust-managed skills and agents into tool directories",
		RunE: func(cmd *cobra.Command, _ []string) error {
			parsedScope, err := parseSyncScope(scope)
			if err != nil {
				return err
			}
			parsedTools, err := parseSyncTools(tools)
			if err != nil {
				return err
			}
			opts.ConfigPath = configPath
			opts.Scope = parsedScope
			opts.Tools = parsedTools

			ctx := cmd.Context()
			svc, err := openService(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = svc.Close() }()

			res, err := svc.Sync(ctx, opts)
			if err != nil {
				return err
			}
			if output == "json" {
				if err := emitJSON(cmd.OutOrStdout(), res.Plan); err != nil {
					return err
				}
			} else {
				emitMarkdown(cmd.OutOrStdout(), res.Plan.Markdown(), output)
			}
			if opts.Check && (res.Plan.Missing > 0 || res.Plan.Drift > 0 || res.Plan.Conflicts > 0) {
				return fmt.Errorf("sync check failed: %d missing, %d drift, %d conflicts", res.Plan.Missing, res.Plan.Drift, res.Plan.Conflicts)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "sync config path, defaults to .stardust/sync.toml")
	cmd.Flags().StringVar(&scope, "scope", string(agentsync.ScopeAll), "scope filter: repo, global, all")
	cmd.Flags().StringSliceVar(&tools, "tool", nil, "tool filter: claude, codex, gemini")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "print planned actions without writing")
	cmd.Flags().BoolVar(&opts.Check, "check", false, "exit non-zero when sync targets are missing or drifted")
	cmd.Flags().BoolVar(&opts.Repair, "repair", false, "repair drifted stardust-managed targets")
	cmd.Flags().StringVar(&output, "output", "auto", "output mode: auto, md, json, plain")
	cmd.AddCommand(newSyncInitCmd(), newSyncReportCmd())
	return cmd
}

func newSyncInitCmd() *cobra.Command {
	var profile string
	var canonical string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold .stardust/sync.toml",
		RunE: func(cmd *cobra.Command, _ []string) error {
			vc, err := resolveVault()
			if err != nil {
				return err
			}
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("resolve home dir: %w", err)
			}
			cfg := agentsync.DefaultConfig(home, vc.Layout.Root)
			switch profile {
			case "", "default":
			case "alxx":
				cfg = agentsync.AlxxMigrationConfig(home, vc.Layout.Root)
			default:
				return fmt.Errorf("unsupported sync profile %q", profile)
			}
			if canonical != "" {
				applyCanonicalPath(&cfg, expandSyncPath(canonical, home, vc.Layout.Root))
			}
			b, err := agentsync.MarshalConfig(cfg)
			if err != nil {
				return err
			}
			if dryRun {
				fmt.Fprint(cmd.OutOrStdout(), string(b))
				return nil
			}
			if _, err := os.Stat(vc.Layout.SyncConfig()); err == nil {
				return fmt.Errorf("sync config already exists: %s", vc.Layout.SyncConfig())
			} else if !os.IsNotExist(err) {
				return fmt.Errorf("stat sync config %s: %w", vc.Layout.SyncConfig(), err)
			}
			if err := agentsync.SaveConfig(vc.Layout.SyncConfig(), cfg); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "wrote sync config", vc.Layout.SyncConfig())
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "default", "profile: default, alxx")
	cmd.Flags().StringVar(&canonical, "canonical", "", "canonical agent infrastructure root")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print sync.toml without writing")
	return cmd
}

func newSyncReportCmd() *cobra.Command {
	var configPath string
	var output string
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Report adoptable agent assets from migration sources",
		RunE: func(cmd *cobra.Command, _ []string) error {
			vc, err := resolveVault()
			if err != nil {
				return err
			}
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("resolve home dir: %w", err)
			}
			if configPath == "" {
				configPath = vc.Layout.SyncConfig()
			} else if !filepath.IsAbs(configPath) {
				configPath = filepath.Join(vc.Layout.Root, configPath)
			}
			cfg, err := agentsync.LoadConfig(configPath, home, vc.Layout.Root)
			if err != nil {
				return err
			}
			items, err := agentsync.Discover(cfg)
			if err != nil {
				return err
			}
			report := agentsync.BuildMigrationReport(cfg, items)
			if output == "json" {
				return emitJSON(cmd.OutOrStdout(), report)
			}
			emitMarkdown(cmd.OutOrStdout(), report.Markdown(), output)
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "sync config path, defaults to .stardust/sync.toml")
	cmd.Flags().StringVar(&output, "output", "auto", "output mode: auto, md, json, plain")
	return cmd
}

func parseSyncScope(raw string) (agentsync.Scope, error) {
	switch agentsync.Scope(strings.TrimSpace(raw)) {
	case "", agentsync.ScopeAll:
		return agentsync.ScopeAll, nil
	case agentsync.ScopeRepo:
		return agentsync.ScopeRepo, nil
	case agentsync.ScopeGlobal:
		return agentsync.ScopeGlobal, nil
	default:
		return "", fmt.Errorf("unsupported sync scope %q", raw)
	}
}

func parseSyncTools(raw []string) ([]agentsync.Tool, error) {
	var out []agentsync.Tool
	for _, part := range raw {
		for _, item := range strings.Split(part, ",") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			tool := agentsync.Tool(item)
			switch tool {
			case agentsync.ToolClaude, agentsync.ToolCodex, agentsync.ToolGemini:
				out = append(out, tool)
			default:
				return nil, fmt.Errorf("unsupported sync tool %q", item)
			}
		}
	}
	return out, nil
}

func applyCanonicalPath(cfg *agentsync.Config, canonical string) {
	for i := range cfg.Sources {
		switch cfg.Sources[i].Name {
		case "canonical-skills":
			cfg.Sources[i].Path = filepath.Join(canonical, "skills")
		case "canonical-agents":
			cfg.Sources[i].Path = filepath.Join(canonical, "agents")
		}
	}
}

func expandSyncPath(path, home, root string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(root, path)
}
