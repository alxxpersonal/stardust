package tui

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/service"
)

// backend bundles the open service the tabs read from.
type backend struct {
	svc    *service.Service
	hasVec bool
}

// Run opens the vault service and launches the interactive multi-tab TUI.
func Run(layout config.Layout, cfg config.Config) error {
	_ = cfg

	ctx := context.Background()
	svc, err := service.Open(ctx, layout.Root)
	if err != nil {
		return fmt.Errorf("open service: %w", err)
	}
	defer func() { _ = svc.Close() }()

	be := &backend{svc: svc}
	status, err := svc.Status(ctx)
	if err == nil {
		be.hasVec = status.Vectors
	}

	if _, err := tea.NewProgram(newApp(be)).Run(); err != nil {
		return fmt.Errorf("run tui: %w", err)
	}
	return nil
}
