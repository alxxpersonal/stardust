package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alxxpersonal/stardust/internal/agentsync"
)

// SyncResult is the planned or applied agent asset sync.
type SyncResult struct {
	Plan agentsync.Plan `json:"plan"`
}

// Sync discovers agent assets, plans target changes, and applies them unless
// the options request dry-run or check mode.
func (s *Service) Sync(_ context.Context, opts agentsync.Options) (SyncResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return SyncResult{}, fmt.Errorf("resolve home dir: %w", err)
	}
	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = s.Layout.SyncConfig()
	} else if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(s.Layout.Root, configPath)
	}

	cfg, err := agentsync.LoadConfig(configPath, home, s.Layout.Root)
	if err != nil {
		return SyncResult{}, err
	}
	items, err := agentsync.Discover(cfg)
	if err != nil {
		return SyncResult{}, err
	}
	plan, err := agentsync.BuildPlan(cfg, items, opts)
	if err != nil {
		return SyncResult{}, err
	}
	if opts.DryRun || opts.Check {
		return SyncResult{Plan: plan}, nil
	}
	plan, err = agentsync.Apply(plan)
	if err != nil {
		return SyncResult{}, err
	}
	return SyncResult{Plan: plan}, nil
}
