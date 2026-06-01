package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/service"
)

// vaultContext bundles the resolved per-vault layout and config for commands
// that touch .stardust files directly (init, hooks) rather than the index.
type vaultContext struct {
	Layout config.Layout
	Config config.Config
}

// resolveVault finds the vault root from the working directory and loads its
// config. It returns config.ErrNoVault when the vault is not initialised.
func resolveVault() (vaultContext, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return vaultContext{}, fmt.Errorf("get working dir: %w", err)
	}
	root, err := config.FindRoot(cwd)
	if err != nil {
		return vaultContext{}, err
	}
	layout := config.Layout{Root: root}
	cfg, err := config.Load(layout.Config())
	if err != nil {
		return vaultContext{}, err
	}
	return vaultContext{Layout: layout, Config: cfg}, nil
}

// openService opens the core Service for the vault containing the working dir.
func openService(ctx context.Context) (*service.Service, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working dir: %w", err)
	}
	return service.Open(ctx, cwd)
}
