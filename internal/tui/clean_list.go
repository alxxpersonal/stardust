package tui

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"

	"github.com/alxxpersonal/stardust/internal/tui/components"
)

// --- Clean Lists ---

type cleanListColumn struct {
	Header    string
	MinWidth  int
	MaxWidth  int
	Align     lipgloss.Position
	Primary   bool
	Muted     bool
	Underline bool
	Numeric   bool
	Count     bool
	Severity  bool
	Warning   bool
}

type cleanListRow struct {
	Cells []string
}

const (
	cleanListGap        = "  "
	cleanListSelected   = "◼ "
	cleanListUnselected = "◻ "
)

var cleanListLabelStyle = lipgloss.NewStyle().
	Foreground(ColorBackground).
	Background(ColorAccent).
	Bold(true).
	Padding(0, 1)

func renderCleanList(title, label string, columns []cleanListColumn, rows []cleanListRow, width int, activeRow int) string {
	return renderCleanListWithDescription(title, label, "", columns, rows, width, activeRow)
}

func renderCleanListWithDescription(
	title string,
	label string,
	description string,
	columns []cleanListColumn,
	rows []cleanListRow,
	width int,
	activeRow int,
) string {
	if width <= 0 {
		return ""
	}
	if len(columns) == 0 {
		return cleanListHeaderBlock(title, label, description, width)
	}

	withMarker := activeRow >= 0 && len(rows) > 0
	fitted := fitCleanListColumns(columns, rows, width, withMarker)

	var body strings.Builder
	for i, row := range rows {
		body.WriteString(renderCleanListRow(fitted, row, i, activeRow, withMarker))
		if i < len(rows)-1 {
			body.WriteString("\n")
		}
	}
	bodyStr := strings.TrimRight(body.String(), "\n")
	if bodyStr == "" {
		bodyStr = MutedStyle.Render("none")
	}

	headerW := width
	if maxW := maxLineWidth(bodyStr); maxW > headerW {
		headerW = maxW
	}
	return cleanListHeaderBlock(title, label, description, headerW) + "\n\n" + bodyStr
}

func cleanListHeaderBlock(title, label, description string, width int) string {
	header := renderCleanListHeader(title, label, width)
	description = components.SanitizeOneLine(description)
	if description == "" {
		return header
	}
	return header + "\n" + MutedStyle.Render(description)
}

func renderCleanListHeader(title, label string, width int) string {
	title = components.SanitizeOneLine(title)
	label = components.SanitizeOneLine(label)
	if title == "" {
		title = "List"
	}
	header := pillStyle.Render(title)
	if label == "" {
		return header
	}
	key := cleanListLabelStyle.Render(label)
	gap := width - lipgloss.Width(header) - lipgloss.Width(key)
	if gap < 2 {
		gap = 2
	}
	return header + strings.Repeat(" ", gap) + key
}

func fitCleanListColumns(columns []cleanListColumn, rows []cleanListRow, width int, withMarker bool) []cleanListColumn {
	fitted := make([]cleanListColumn, len(columns))
	copy(fitted, columns)

	widths := make([]int, len(fitted))
	mins := make([]int, len(fitted))
	for i, col := range fitted {
		w := lipgloss.Width(components.SanitizeOneLine(col.Header))
		for _, row := range rows {
			if i >= len(row.Cells) {
				continue
			}
			if cellW := lipgloss.Width(components.SanitizeOneLine(row.Cells[i])); cellW > w {
				w = cellW
			}
		}
		if col.MaxWidth > 0 && w > col.MaxWidth {
			w = col.MaxWidth
		}
		minW := col.MinWidth
		if minW <= 0 {
			minW = 1
		}
		if w < minW {
			w = minW
		}
		widths[i] = w
		mins[i] = minW
	}

	available := width - cleanListGapWidth(len(fitted))
	if withMarker {
		available -= lipgloss.Width(cleanListSelected)
	}
	if available < len(fitted) {
		available = len(fitted)
	}

	total := sumInts(widths)
	primary := primaryColumnIndex(fitted)
	if total > available {
		shrinkCleanWidths(widths, mins, total-available, primary)
	}

	for i := range fitted {
		if widths[i] < 1 {
			widths[i] = 1
		}
		fitted[i].MinWidth = widths[i]
	}
	return fitted
}

func renderCleanListRow(columns []cleanListColumn, row cleanListRow, rowIndex, activeRow int, withMarker bool) string {
	selected := withMarker && rowIndex == activeRow
	var b strings.Builder
	if withMarker {
		marker := cleanListUnselected
		style := MutedStyle
		if selected {
			marker = cleanListSelected
			style = AccentStyle.Bold(true)
		}
		b.WriteString(style.Render(marker))
	}
	for i, col := range columns {
		if i > 0 {
			b.WriteString(cleanListGap)
		}
		cell := ""
		if i < len(row.Cells) {
			cell = row.Cells[i]
		}
		clamped := clampCleanText(cell, col.MinWidth)
		styled := cleanListCellStyle(col, cell, selected).Render(clamped)
		b.WriteString(padStyledCell(styled, lipgloss.Width(clamped), col.MinWidth, col.Align))
	}
	return b.String()
}

// padStyledCell pads an already-styled cell with plain spaces to width, so that
// styling such as underline covers only the text and not the trailing padding.
func padStyledCell(styled string, visibleW, width int, align lipgloss.Position) string {
	pad := width - visibleW
	if pad < 0 {
		pad = 0
	}
	switch align {
	case lipgloss.Right:
		return strings.Repeat(" ", pad) + styled
	case lipgloss.Center:
		left := pad / 2
		right := pad - left
		return strings.Repeat(" ", left) + styled + strings.Repeat(" ", right)
	default:
		return styled + strings.Repeat(" ", pad)
	}
}

func cleanListCellStyle(col cleanListColumn, value string, selected bool) lipgloss.Style {
	switch {
	case col.Severity:
		return severityStyle(value)
	case col.Numeric:
		return numericValueStyle(value)
	case col.Count:
		return countValueStyle(value)
	case col.Warning:
		return WarningStyle
	case col.Primary && selected:
		return AccentStyle.Bold(true)
	case col.Primary:
		return NormalStyle
	case col.Underline:
		return MutedStyle.Underline(true)
	case col.Muted:
		return MutedStyle
	default:
		return NormalStyle
	}
}

func severityStyle(value string) lipgloss.Style {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "error", "fatal":
		return ErrorStyle
	case "warning", "warn":
		return WarningStyle
	case "info", "notice":
		return AccentStyle
	default:
		return MutedStyle
	}
}

func numericValueStyle(value string) lipgloss.Style {
	raw := strings.TrimSpace(strings.TrimSuffix(value, "%"))
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return AccentStyle
	}
	if n > 1 {
		n /= 100
	}
	switch {
	case n >= 0.75:
		return ScoreHighStyle
	case n >= 0.4:
		return ScoreMedStyle
	default:
		return ScoreLowStyle
	}
}

func countValueStyle(value string) lipgloss.Style {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return AccentStyle
	}
	if n == 0 {
		return MutedStyle
	}
	return AccentStyle.Bold(true)
}

func cleanListGapWidth(columns int) int {
	if columns <= 1 {
		return 0
	}
	return (columns - 1) * lipgloss.Width(cleanListGap)
}

func primaryColumnIndex(columns []cleanListColumn) int {
	for i, col := range columns {
		if col.Primary {
			return i
		}
	}
	if len(columns) == 0 {
		return -1
	}
	return 0
}

func shrinkCleanWidths(widths, mins []int, deficit int, primary int) {
	if deficit <= 0 {
		return
	}
	if primary >= 0 && primary < len(widths) {
		deficit = shrinkOneCleanWidth(widths, mins, primary, deficit)
	}
	for deficit > 0 {
		best := -1
		spare := 0
		for i := range widths {
			if widths[i]-mins[i] > spare {
				spare = widths[i] - mins[i]
				best = i
			}
		}
		if best == -1 || spare <= 0 {
			return
		}
		deficit = shrinkOneCleanWidth(widths, mins, best, deficit)
	}
}

func shrinkOneCleanWidth(widths, mins []int, index, deficit int) int {
	spare := widths[index] - mins[index]
	if spare <= 0 {
		return deficit
	}
	if spare > deficit {
		spare = deficit
	}
	widths[index] -= spare
	return deficit - spare
}

func sumInts(values []int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	return total
}

func maxLineWidth(block string) int {
	maxW := 0
	for _, line := range strings.Split(block, "\n") {
		if w := lipgloss.Width(line); w > maxW {
			maxW = w
		}
	}
	return maxW
}

func clampCleanText(text string, width int) string {
	if width <= 0 {
		return ""
	}
	cleaned := components.SanitizeOneLine(text)
	if lipgloss.Width(cleaned) <= width {
		return cleaned
	}
	if width == 1 {
		return "…"
	}
	return truncateCleanRunes(cleaned, width-1) + "…"
}

func truncateCleanRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	var b strings.Builder
	b.Grow(max)
	n := 0
	for _, r := range s {
		if n >= max {
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

func cleanListCountLabel(count int, noun string) string {
	if count == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", count, noun)
}
