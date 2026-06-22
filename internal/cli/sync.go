package cli

import (
	"fmt"
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
