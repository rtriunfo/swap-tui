package ui

import (
	"sort"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"swap-tui/internal/process"
)

const refreshInterval = 30 * time.Second

const DefaultTopN = 50

type appState int

const (
	stateScanning    appState = iota // initial or mid-refresh scan in progress
	stateReady                       // data showing, awaiting keyboard input
	stateConfirmKill                 // awaiting y/n before sending SIGTERM
)

// Model is the bubbletea application model.
type Model struct {
	processes  []process.Info // sorted by SwappedBytes descending; only non-zero swap
	swapStats  process.SwapStats
	scanning   bool
	state      appState
	selected   int
	width      int
	height     int
	topN       int
	err        error
	resultChan <-chan process.ScanResult
	pending    map[int]process.Info // accumulates vmmap results mid-scan
}

// -- Messages -----------------------------------------------------------------

type (
	scanStartedMsg struct{ ch <-chan process.ScanResult }
	scanResultMsg  struct {
		result process.ScanResult
		done   bool
	}
	swapStatsMsg struct{ stats process.SwapStats }
	tickMsg      struct{ t time.Time }
	errMsg       struct{ err error }
)

// -- Constructor --------------------------------------------------------------

func New(topN int) Model {
	if topN <= 0 {
		topN = DefaultTopN
	}
	return Model{
		topN:     topN,
		scanning: true,
		pending:  make(map[int]process.Info),
	}
}

// -- Bubbletea interface ------------------------------------------------------

func (m Model) Init() tea.Cmd {
	return tea.Batch(cmdStartScan(m.topN), cmdReadSwapStats(), cmdScheduleTick())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		return m.handleKey(msg)

	case scanStartedMsg:
		m.resultChan = msg.ch
		return m, cmdWaitForResult(m.resultChan)

	case scanResultMsg:
		if !msg.done {
			if msg.result.Err == nil {
				m.pending[msg.result.Info.PID] = msg.result.Info
			}
			return m, cmdWaitForResult(m.resultChan)
		}
		// All vmmap calls done — promote pending to the display list.
		m.processes = sortedBySwap(m.pending)
		m.pending = make(map[int]process.Info)
		m.scanning = false
		m.state = stateReady
		m.selected = clamp(m.selected, 0, len(m.processes)-1)
		return m, cmdReadSwapStats()

	case swapStatsMsg:
		m.swapStats = msg.stats

	case tickMsg:
		return m.startRefresh(), tea.Batch(cmdStartScan(m.topN), cmdScheduleTick())

	case errMsg:
		m.err = msg.err
		m.scanning = false
	}

	return m, nil
}

// -- Key handling -------------------------------------------------------------

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {

	case stateConfirmKill:
		switch {
		case key.Matches(msg, keys.Confirm):
			m.state = stateReady
			return m, cmdKill(m.selectedProcess())
		case key.Matches(msg, keys.Cancel):
			m.state = stateReady
		}

	case stateReady, stateScanning:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Up):
			m.selected = clamp(m.selected-1, 0, len(m.processes)-1)
		case key.Matches(msg, keys.Down):
			m.selected = clamp(m.selected+1, 0, len(m.processes)-1)
		case key.Matches(msg, keys.Kill):
			if len(m.processes) > 0 {
				m.state = stateConfirmKill
			}
		case key.Matches(msg, keys.Refresh):
			return m.startRefresh(), cmdStartScan(m.topN)
		}
	}
	return m, nil
}

// -- Commands -----------------------------------------------------------------

func cmdStartScan(topN int) tea.Cmd {
	return func() tea.Msg {
		procs, err := process.TopN(topN)
		if err != nil {
			return errMsg{err}
		}
		return scanStartedMsg{ch: process.Scan(procs)}
	}
}

func cmdWaitForResult(ch <-chan process.ScanResult) tea.Cmd {
	return func() tea.Msg {
		result, ok := <-ch
		return scanResultMsg{result: result, done: !ok}
	}
}

func cmdReadSwapStats() tea.Cmd {
	return func() tea.Msg {
		stats, err := process.ReadSwapStats()
		if err != nil {
			return errMsg{err}
		}
		return swapStatsMsg{stats}
	}
}

func cmdScheduleTick() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
		return tickMsg{t: t}
	})
}

func cmdKill(p *process.Info) tea.Cmd {
	if p == nil {
		return nil
	}
	pid := p.PID
	return func() tea.Msg {
		syscall.Kill(pid, syscall.SIGTERM) //nolint:errcheck
		return nil
	}
}

// -- Helpers ------------------------------------------------------------------

func (m *Model) startRefresh() Model {
	m.scanning = true
	m.pending = make(map[int]process.Info)
	return *m
}

func (m *Model) selectedProcess() *process.Info {
	if m.selected < 0 || m.selected >= len(m.processes) {
		return nil
	}
	p := m.processes[m.selected]
	return &p
}

func sortedBySwap(pending map[int]process.Info) []process.Info {
	procs := make([]process.Info, 0, len(pending))
	for _, p := range pending {
		if p.SwappedBytes > 0 {
			procs = append(procs, p)
		}
	}
	sort.Slice(procs, func(i, j int) bool {
		return procs[i].SwappedBytes > procs[j].SwappedBytes
	})
	return procs
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
