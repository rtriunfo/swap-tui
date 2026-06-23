package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"swap-tui/internal/process"
)

var (
	styleTitle    = lipgloss.NewStyle().Bold(true)
	styleSubtitle = lipgloss.NewStyle().Foreground(lipgloss.Color("#888780"))
	styleFooter   = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
	styleConfirm  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ba7517")).Bold(true)
	styleError    = lipgloss.NewStyle().Foreground(lipgloss.Color("#a32d2d"))
)

func (m Model) View() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(styleTitle.Render("  macOS Swap Audit") + "\n")
	b.WriteString(styleSubtitle.Render("  "+swapStatsLine(m.swapStats)) + "\n\n")

	switch m.state {
	case stateConfirmKill:
		b.WriteString(confirmPrompt(m.selectedProcess()))
		return b.String()
	}

	if m.err != nil {
		b.WriteString(styleError.Render(fmt.Sprintf("  error: %v", m.err)) + "\n")
	}

	if len(m.processes) == 0 {
		if m.scanning {
			b.WriteString(styleSubtitle.Render("  Scanning processes…") + "\n")
		} else {
			b.WriteString(styleSubtitle.Render("  No swap usage found.") + "\n")
		}
		b.WriteString("\n" + styleFooter.Render(keys.help()) + "\n")
		return b.String()
	}

	bw := BarWidth(m.width)
	b.WriteString(styleBold.Render(Header(bw)) + "\n")
	b.WriteString(styleSubtitle.Render(Divider(bw)) + "\n")

	maxSwap := m.processes[0].SwappedBytes // already sorted descending

	for i, p := range m.processes {
		b.WriteString(Row(p, maxSwap, bw, i == m.selected) + "\n")
	}

	if m.scanning {
		b.WriteString("\n" + styleSubtitle.Render("  Refreshing…") + "\n")
	}

	b.WriteString("\n" + styleFooter.Render(keys.help()) + "\n")
	return b.String()
}

func swapStatsLine(s process.SwapStats) string {
	if s.TotalBytes == 0 {
		return "loading…"
	}
	pct := int(float64(s.UsedBytes) / float64(s.TotalBytes) * 100)
	return fmt.Sprintf("system swap  used %s / %s  (%d%%)",
		FormatBytes(s.UsedBytes), FormatBytes(s.TotalBytes), pct)
}

func confirmPrompt(p *process.Info) string {
	if p == nil {
		return ""
	}
	return styleConfirm.Render(
		fmt.Sprintf("  Send SIGTERM to %s (PID %d)?  [y] confirm  [n/esc] cancel\n", p.Name, p.PID),
	)
}
