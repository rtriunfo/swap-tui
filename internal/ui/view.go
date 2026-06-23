package ui

import (
	"fmt"
	"strings"
	"syscall"

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

	vis := m.visibleProcesses()

	b.WriteString("\n")
	b.WriteString(styleTitle.Render("  macOS Swap Audit") + "\n")
	b.WriteString(styleSubtitle.Render("  "+swapStatsLine(m.swapStats)) + "\n")
	b.WriteString(styleSubtitle.Render("  "+trackedLine(vis, m.filter != "")) + "\n\n")

	if m.state == stateConfirmKill {
		b.WriteString(confirmPrompt(m.selectedProcess(), m.pendingSig))
		return b.String()
	}

	if m.err != nil {
		b.WriteString(styleError.Render(fmt.Sprintf("  error: %v", m.err)) + "\n")
	}

	if len(vis) == 0 {
		switch {
		case m.scanning:
			b.WriteString(styleSubtitle.Render("  Scanning processes…") + "\n")
		case m.filter != "":
			b.WriteString(styleSubtitle.Render(fmt.Sprintf("  No processes match %q.", m.filter)) + "\n")
		default:
			b.WriteString(styleSubtitle.Render("  No swap usage found.") + "\n")
		}
		if m.state == stateFilter {
			b.WriteString("\n" + styleConfirm.Render("  filter: "+m.filter+"_") + "\n")
		}
		b.WriteString("\n" + styleFooter.Render(m.footer()) + "\n")
		return b.String()
	}

	bw := BarWidth(m.width)
	b.WriteString(styleBold.Render(Header(bw)) + "\n")
	b.WriteString(styleSubtitle.Render(Divider(bw)) + "\n")

	maxSwap := maxSwapped(vis) // independent of sort order

	// Compute the visible window of rows.
	visible := m.visibleRows()
	total := len(vis)
	start := clamp(m.scrollOff, 0, total-1)
	end := clamp(start+visible, 0, total)

	// "↑ N more" indicator when rows are hidden above.
	if start > 0 {
		b.WriteString(styleFooter.Render(fmt.Sprintf("  ↑ %d more", start)) + "\n")
	}

	for i := start; i < end; i++ {
		b.WriteString(Row(vis[i], maxSwap, bw, i == m.selected) + "\n")
	}

	// "↓ N more" indicator when rows are hidden below.
	if end < total {
		b.WriteString(styleFooter.Render(fmt.Sprintf("  ↓ %d more", total-end)) + "\n")
	}

	if m.state == stateFilter {
		b.WriteString("\n" + styleConfirm.Render("  filter: "+m.filter+"_") + "\n")
	} else if m.scanning {
		b.WriteString("\n" + styleSubtitle.Render("  Refreshing…") + "\n")
	}

	b.WriteString("\n" + styleFooter.Render(m.footer()) + "\n")
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

// trackedLine summarises the displayed processes and their combined swap.
func trackedLine(procs []process.Info, filtered bool) string {
	var total int64
	for _, p := range procs {
		total += p.SwappedBytes
	}
	label := "tracked processes"
	if filtered {
		label = "filtered processes"
	}
	return fmt.Sprintf("%s: %d · total swapped %s", label, len(procs), FormatBytes(total))
}

// maxSwapped returns the largest SwappedBytes in procs, used to normalise bars.
func maxSwapped(procs []process.Info) int64 {
	var max int64
	for _, p := range procs {
		if p.SwappedBytes > max {
			max = p.SwappedBytes
		}
	}
	return max
}

// footer renders the key-binding help plus the active sort and filter state.
func (m Model) footer() string {
	sortLabel := "swap"
	if m.sortMode == sortByRSS {
		sortLabel = "rss"
	}
	s := keys.help() + "   sort: " + sortLabel
	if m.filter != "" {
		s += fmt.Sprintf("   filter: %q (esc clears)", m.filter)
	}
	return s
}

func confirmPrompt(p *process.Info, sig syscall.Signal) string {
	if p == nil {
		return ""
	}
	return styleConfirm.Render(
		fmt.Sprintf("  Send %s to %s (PID %d)?  [y] confirm  [n/esc] cancel\n",
			signalName(sig), p.Name, p.PID),
	)
}

func signalName(sig syscall.Signal) string {
	if sig == syscall.SIGKILL {
		return "SIGKILL"
	}
	return "SIGTERM"
}
