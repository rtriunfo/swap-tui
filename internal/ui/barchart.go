package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"swap-tui/internal/process"
)

const (
	runeBar   = "█"
	runeEmpty = "░"

	colPID  = 6
	colName = 30
	colSwap = 10
	colRSS  = 10
	// fixed chars: PID + name + swap + rss + 4 gaps of 2 spaces each
	fixedWidth = colPID + colName + colSwap + colRSS + 8
)

var (
	styleSystem   = lipgloss.NewStyle().Foreground(lipgloss.Color("#a32d2d"))
	styleWorth    = lipgloss.NewStyle().Foreground(lipgloss.Color("#ba7517"))
	styleSmall    = lipgloss.NewStyle().Foreground(lipgloss.Color("#888780"))
	styleDim      = lipgloss.NewStyle().Foreground(lipgloss.Color("#444444"))
	styleSelected = lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa")).Bold(true)
	styleBold     = lipgloss.NewStyle().Bold(true)
)

// BarWidth returns the number of characters available for the bar
// given the terminal width.
func BarWidth(termWidth int) int {
	w := termWidth - fixedWidth
	if w < 10 {
		return 10
	}
	if w > 60 {
		return 60
	}
	return w
}

// FormatBytes converts a byte count into a human-readable string.
func FormatBytes(b int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.1f GB", float64(b)/GB)
	case b >= MB:
		return fmt.Sprintf("%.1f MB", float64(b)/MB)
	case b >= KB:
		return fmt.Sprintf("%.1f KB", float64(b)/KB)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// Header returns the column header row.
func Header(barWidth int) string {
	bar := fmt.Sprintf("%-*s", barWidth, "SWAP")
	return fmt.Sprintf("  %-*s  %-*s  %s  %*s  %*s",
		colPID, "PID",
		colName, "PROCESS",
		bar,
		colSwap, "SWAPPED",
		colRSS, "RSS",
	)
}

// Divider returns a separator row matching the header width.
func Divider(barWidth int) string {
	return fmt.Sprintf("  %-*s  %-*s  %s  %*s  %*s",
		colPID, strings.Repeat("─", colPID),
		colName, strings.Repeat("─", colName),
		strings.Repeat("─", barWidth),
		colSwap, strings.Repeat("─", colSwap),
		colRSS, strings.Repeat("─", colRSS),
	)
}

// Row renders a single process as one bar-chart line.
func Row(p process.Info, maxSwap int64, barWidth int, selected bool) string {
	name := truncate(p.Name, colName)
	pid := fmt.Sprintf("%-*d", colPID, p.PID)
	swap := fmt.Sprintf("%*s", colSwap, FormatBytes(p.SwappedBytes))
	rss := fmt.Sprintf("%*s", colRSS, FormatBytes(p.RSSBytes))

	filled := barFilled(p.SwappedBytes, maxSwap, barWidth)
	empty := barWidth - filled

	bar := barColorStyle(p).Render(strings.Repeat(runeBar, filled)) +
		styleDim.Render(strings.Repeat(runeEmpty, empty))

	// The leading gutter doubles as the selection marker. An accent bar in the
	// gutter is reliably visible — unlike a background fill, which the nested
	// ANSI in the coloured bar would reset partway across the row.
	gutter := "  "
	if selected {
		gutter = styleSelected.Render("▌ ")
	}

	return fmt.Sprintf("%s%-*s  %-*s  %s  %s  %s",
		gutter,
		colPID, pid,
		colName, name,
		bar,
		swap,
		rss,
	)
}

// barFilled calculates how many bar characters should be filled.
func barFilled(swapped, maxSwap int64, barWidth int) int {
	if maxSwap == 0 {
		return 0
	}
	n := int(float64(swapped) / float64(maxSwap) * float64(barWidth))
	if n > barWidth {
		return barWidth
	}
	return n
}

// SwapCategory classifies a process for colour-coding purposes.
// Exported so tests can verify the logic without depending on lipgloss internals.
func SwapCategory(p process.Info) string {
	if p.Owner == "system" {
		return "system"
	}
	if p.SwappedBytes >= 100*1024*1024 {
		return "worth"
	}
	return "small"
}

func barColorStyle(p process.Info) lipgloss.Style {
	switch SwapCategory(p) {
	case "system":
		return styleSystem
	case "worth":
		return styleWorth
	default:
		return styleSmall
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
