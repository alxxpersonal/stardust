package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// Tab indices.
const (
	tabSearch = iota
	tabStatus
	tabGraph
	tabCount
)

var tabNames = []string{"Search", "Status", "Graph"}

// renderTabBar renders the tab strip with the active tab highlighted.
func renderTabBar(active int) string {
	parts := make([]string, len(tabNames))
	for i, name := range tabNames {
		label := fmt.Sprintf(" %d %s ", i+1, name)
		if i == active {
			parts[i] = lipgloss.NewStyle().Foreground(colorBg).Background(colorPrimary).Bold(true).Render(label)
		} else {
			parts[i] = mutedStyle.Render(label)
		}
	}
	return strings.Join(parts, " ")
}
