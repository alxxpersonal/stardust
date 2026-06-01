package tui

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/embed"
	"github.com/alxxpersonal/stardust/internal/index"
)

// backend bundles the open index and config the tabs read from.
type backend struct {
	layout config.Layout
	cfg    config.Config
	store  *index.Store
	embed  *embed.Client
	hasVec bool
}

// Run opens the vault index and launches the interactive multi-tab TUI.
func Run(layout config.Layout, cfg config.Config) error {
	ctx := context.Background()
	store, err := index.Open(ctx, layout.DB())
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	emb := embed.New(cfg.OllamaURL, cfg.EmbedModel)
	be := &backend{
		layout: layout,
		cfg:    cfg,
		store:  store,
		embed:  emb,
		hasVec: emb.Available(ctx),
	}

	if _, err := tea.NewProgram(newApp(be)).Run(); err != nil {
		return fmt.Errorf("run tui: %w", err)
	}
	return nil
}
