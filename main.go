package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"swap-tui/internal/ui"
)

// version is overridable at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	topN := flag.Int("n", ui.DefaultTopN, "number of top RSS processes to scan")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("swap-tui %s\n", version)
		return
	}

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
