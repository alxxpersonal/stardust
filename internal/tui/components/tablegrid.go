package components

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/alxxpersonal/stardust/internal/ui"
)

// TableColumn defines a single column for TableGrid.
type TableColumn struct {
	Header string
	Width  int
	Align  lipgloss.Position
}

const (
	tableGridGap        = "  "
	tableGridSelected   = "◼ "
	tableGridUnselected = "◻ "
)

var tableHeaderStyle = lipgloss.NewStyle().
	Foreground(ui.Secondary).
	Bold(true)

var tableMutedStyle = lipgloss.NewStyle().
	Foreground(ui.Muted)

var tablePrimaryStyle = lipgloss.NewStyle().
	Foreground(ui.Text)

var tableActivePrimaryStyle = lipgloss.NewStyle().
	Foreground(ui.Accent).
	Bold(true)

var tableScoreHighStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#34d399")).
	Bold(true)

var tableScoreMedStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#f59e0b"))

var tableScoreLowStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#ff5555"))

var tableGridActiveRowsEnabled = true

// SetTableGridActiveRowsEnabled toggles active-row markers globally.
func SetTableGridActiveRowsEnabled(enabled bool) {
	tableGridActiveRowsEnabled = enabled
}

// TableGrid renders a clean aligned list with two-space gaps.
func TableGrid(columns []TableColumn, rows [][]string, tableWidth int) string {
	return TableGridWithActiveRow(columns, rows, tableWidth, -1)
}

// TableGridWithActiveRow marks one data row by index.
func TableGridWithActiveRow(columns []TableColumn, rows [][]string, tableWidth int, activeRow int) string {
	if !tableGridActiveRowsEnabled {
		activeRow = -1
	}
	if tableWidth <= 0 {
		return ""
	}
	if len(columns) == 0 {
		return padRight("", tableWidth)
	}

	cols := fitGridColumns(columns, tableWidth, activeRow >= 0 && len(rows) > 0)
	var out []string
	out = append(out, renderGridHeader(cols))
	for i, row := range rows {
		out = append(out, renderGridRow(cols, row, i == activeRow, activeRow >= 0))
	}
	return strings.Join(out, "\n")
}

func fitGridColumns(columns []TableColumn, tableWidth int, withMarker bool) []TableColumn {
	fitted := make([]TableColumn, len(columns))
	copy(fitted, columns)

	available := tableWidth - gridGapWidth(len(fitted))
	if withMarker {
		available -= lipgloss.Width(tableGridSelected)
	}
	if available < len(fitted) {
		available = len(fitted)
	}

	widths := make([]int, len(fitted))
	mins := make([]int, len(fitted))
	for i := range fitted {
		if fitted[i].Width < 1 {
			fitted[i].Width = 1
		}
		widths[i] = fitted[i].Width
		mins[i] = minGridWidth(fitted[i])
		if widths[i] < mins[i] {
			widths[i] = mins[i]
		}
	}

	total := sumGridWidths(widths)
	if total > available {
		shrinkGridWidths(widths, mins, total-available)
	}

	for i := range fitted {
		fitted[i].Width = widths[i]
	}
	return fitted
}

func renderGridHeader(columns []TableColumn) string {
	cells := make([]string, 0, len(columns))
	for _, col := range columns {
		text := renderGridCell(col.Header, col.Width, col.Align)
		cells = append(cells, tableHeaderStyle.Render(text))
	}
	return strings.Join(cells, tableGridGap)
}

func renderGridRow(columns []TableColumn, cells []string, active bool, withMarker bool) string {
	parts := make([]string, 0, len(columns)+1)
	if withMarker {
		marker := tableGridUnselected
		style := tableMutedStyle
		if active {
			marker = tableGridSelected
			style = tableActivePrimaryStyle
		}
		parts = append(parts, style.Render(marker))
	}
	for i, col := range columns {
		text := ""
		if i < len(cells) {
			text = cells[i]
		}
		rendered := renderGridCell(text, col.Width, col.Align)
		style := gridCellStyle(i, col, text, active)
		parts = append(parts, style.Render(rendered))
	}
	return strings.Join(parts, tableGridGap)
}

func renderGridCell(text string, width int, align lipgloss.Position) string {
	if width <= 0 {
		return ""
	}

	clamped := ClampTextWidthEllipsis(text, width)
	w := lipgloss.Width(clamped)
	if w >= width {
		return truncateRunes(clamped, width)
	}

	pad := width - w
	switch align {
	case lipgloss.Right:
		return strings.Repeat(" ", pad) + clamped
	case lipgloss.Center:
		left := pad / 2
		right := pad - left
		return strings.Repeat(" ", left) + clamped + strings.Repeat(" ", right)
	default:
		return clamped + strings.Repeat(" ", pad)
	}
}

func gridCellStyle(index int, col TableColumn, value string, active bool) lipgloss.Style {
	if col.Align == lipgloss.Right {
		return gridNumericStyle(value)
	}
	if index == 0 && active {
		return tableActivePrimaryStyle
	}
	if index == 0 {
		return tablePrimaryStyle
	}
	return tableMutedStyle
}

func gridNumericStyle(value string) lipgloss.Style {
	raw := strings.TrimSpace(strings.TrimSuffix(value, "%"))
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return tableMutedStyle
	}
	if n > 1 {
		n /= 100
	}
	switch {
	case n >= 0.75:
		return tableScoreHighStyle
	case n >= 0.4:
		return tableScoreMedStyle
	default:
		return tableScoreLowStyle
	}
}

func gridGapWidth(columns int) int {
	if columns <= 1 {
		return 0
	}
	return (columns - 1) * lipgloss.Width(tableGridGap)
}

func minGridWidth(col TableColumn) int {
	w := lipgloss.Width(SanitizeOneLine(col.Header))
	if w < 1 {
		w = 1
	}
	if w > 12 {
		w = 12
	}
	return w
}

func sumGridWidths(widths []int) int {
	total := 0
	for _, width := range widths {
		total += width
	}
	return total
}

func shrinkGridWidths(widths, mins []int, deficit int) {
	for deficit > 0 {
		best := -1
		bestSpare := 0
		for i := range widths {
			spare := widths[i] - mins[i]
			if spare > bestSpare {
				bestSpare = spare
				best = i
			}
		}
		if best == -1 || bestSpare <= 0 {
			return
		}
		widths[best]--
		deficit--
	}
}
