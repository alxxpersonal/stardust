package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/embed"
	"github.com/alxxpersonal/stardust/internal/index"
)

// vaultContext bundles the resolved per-vault layout and config for a command.
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

// openStore opens the sqlite index for this vault.
func (vc vaultContext) openStore(ctx context.Context) (*index.Store, error) {
	return index.Open(ctx, vc.Layout.DB())
}

// embedder returns an Ollama client configured for this vault.
func (vc vaultContext) embedder() *embed.Client {
	return embed.New(vc.Config.OllamaURL, vc.Config.EmbedModel)
}
