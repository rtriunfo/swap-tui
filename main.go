package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"swap-tui/internal/ui"
)

func main() {
	topN := flag.Int("n", ui.DefaultTopN, "number of top RSS processes to scan")
	flag.Parse()

	p := tea.NewProgram(
		ui.New(*topN),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
