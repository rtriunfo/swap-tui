package ui

import (
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"swap-tui/internal/process"
)

const refreshInterval = 30 * time.Second

const DefaultTopN = 50

// visibleRows is the number of process rows to display when height is unknown.
const defaultVisibleRows = 20

type appState int

const (
	stateScanning    appState = iota // initial or mid-refresh scan in progress
	stateReady                       // data showing, awaiting keyboard input
	stateConfirmKill                 // awaiting y/n before sending a signal
	stateFilter                      // typing a name filter
)

// sortMode controls the ordering of the process list.
type sortMode int

const (
	sortBySwap sortMode = iota // descending by SwappedBytes (default)
	sortByRSS                  // descending by RSSBytes
)

// Model is the bubbletea application model.
type Model struct {
	processes   []process.Info // full list, sorted by sortMode; only non-zero swap
	swapStats   process.SwapStats
	scanning    bool
	state       appState
	sortMode    sortMode       // active ordering of processes
	filter      string         // case-insensitive name substring; "" = no filter
	pendingSig  syscall.Signal // signal to send once a kill is confirmed
	selected    int            // visible index into visibleProcesses() (from selectedPID)
	selectedPID int            // PID of the currently highlighted row; 0 = none
	scrollOff   int            // index of the first visible row in the viewport
	width       int
	height      int
	topN        int
	err         error
	resultChan  <-chan process.ScanResult
	pending     map[int]process.Info // accumulates vmmap results mid-scan
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
	killDoneMsg  struct{} // sent after SIGTERM so we can trigger a refresh
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
		m.processes = sortProcesses(m.pending, m.sortMode)
		m.pending = make(map[int]process.Info)
		m.scanning = false
		// Don't disturb an open confirm/filter prompt when a refresh lands.
		if m.state == stateScanning {
			m.state = stateReady
		}
		// Re-resolve selected index from the stable selectedPID.
		m.reconcileSelection()
		return m, cmdReadSwapStats()

	case swapStatsMsg:
		m.swapStats = msg.stats

	case tickMsg:
		return m.startRefresh(), tea.Batch(cmdStartScan(m.topN), cmdScheduleTick())

	case killDoneMsg:
		// SIGTERM sent — kick off an immediate refresh so the process drops out.
		return m.startRefresh(), cmdStartScan(m.topN)

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
			return m, cmdKill(m.selectedProcess(), m.pendingSig)
		case key.Matches(msg, keys.Cancel):
			m.state = stateReady
		}

	case stateFilter:
		switch msg.Type {
		case tea.KeyEnter, tea.KeyEsc:
			m.state = stateReady
		case tea.KeyBackspace:
			if n := len(m.filter); n > 0 {
				m.filter = m.filter[:n-1]
				m.reconcileSelection()
			}
		case tea.KeyRunes, tea.KeySpace:
			m.filter += string(msg.Runes)
			m.reconcileSelection()
		}

	case stateReady, stateScanning:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case msg.Type == tea.KeyEsc && m.filter != "":
			// Esc clears an active filter.
			m.filter = ""
			m.reconcileSelection()
		case key.Matches(msg, keys.Up):
			m.moveSelection(-1)
		case key.Matches(msg, keys.Down):
			m.moveSelection(+1)
		case key.Matches(msg, keys.Kill):
			if len(m.visibleProcesses()) > 0 {
				m.pendingSig = syscall.SIGTERM
				m.state = stateConfirmKill
			}
		case key.Matches(msg, keys.ForceKill):
			if len(m.visibleProcesses()) > 0 {
				m.pendingSig = syscall.SIGKILL
				m.state = stateConfirmKill
			}
		case key.Matches(msg, keys.Sort):
			m.toggleSort()
		case key.Matches(msg, keys.Filter):
			m.state = stateFilter
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

func cmdKill(p *process.Info, sig syscall.Signal) tea.Cmd {
	if p == nil {
		return nil
	}
	pid := p.PID
	return func() tea.Msg {
		syscall.Kill(pid, sig) //nolint:errcheck
		return killDoneMsg{}
	}
}

// -- Helpers ------------------------------------------------------------------

func (m *Model) startRefresh() Model {
	m.scanning = true
	m.pending = make(map[int]process.Info)
	return *m
}

func (m *Model) selectedProcess() *process.Info {
	vis := m.visibleProcesses()
	if m.selected < 0 || m.selected >= len(vis) {
		return nil
	}
	p := vis[m.selected]
	return &p
}

// visibleProcesses returns the processes matching the active name filter.
// With no filter it returns the full list unchanged.
func (m Model) visibleProcesses() []process.Info {
	if m.filter == "" {
		return m.processes
	}
	q := strings.ToLower(m.filter)
	out := make([]process.Info, 0, len(m.processes))
	for _, p := range m.processes {
		if strings.Contains(strings.ToLower(p.Name), q) {
			out = append(out, p)
		}
	}
	return out
}

// moveSelection shifts the highlighted row by delta within the visible list,
// keeping selectedPID and the scroll window in sync.
func (m *Model) moveSelection(delta int) {
	vis := m.visibleProcesses()
	m.selected = clamp(m.selected+delta, 0, len(vis)-1)
	m.selectedPID = pidAt(vis, m.selected)
	m.scrollOff = clampScroll(m.scrollOff, m.selected, m.visibleRows())
}

// reconcileSelection re-derives the visible index from selectedPID after the
// visible list changes (scan, sort toggle, or filter edit).
func (m *Model) reconcileSelection() {
	vis := m.visibleProcesses()
	m.selected, m.selectedPID = resolveSelection(vis, m.selectedPID, m.selected)
	m.scrollOff = clampScroll(m.scrollOff, m.selected, m.visibleRows())
}

// toggleSort flips between swap- and RSS-ordered views, re-sorting the current
// list immediately and keeping the same process selected.
func (m *Model) toggleSort() {
	if m.sortMode == sortBySwap {
		m.sortMode = sortByRSS
	} else {
		m.sortMode = sortBySwap
	}
	sortInPlace(m.processes, m.sortMode)
	m.reconcileSelection()
}

// visibleRows returns the number of process rows that fit in the terminal,
// accounting for the fixed chrome lines (title + subtitle + blank + header +
// divider + blank + footer = 7 lines, plus 1 for the "Refreshing…" line when
// scanning). Falls back to defaultVisibleRows when height is not yet known.
func (m *Model) visibleRows() int {
	if m.height <= 0 {
		return defaultVisibleRows
	}
	chrome := 8 // title, subtitle, blank, header, divider, blank, footer, margin
	n := m.height - chrome
	if n < 1 {
		return 1
	}
	return n
}

// resolveSelection finds the visible index for the given PID after a re-sort.
// If the PID is not present (e.g. nothing selected yet, or the process exited)
// it keeps the cursor near fallbackIdx — clamped to the list — rather than
// jumping. With fallbackIdx 0 this leaves a fresh list selected at the top.
// Returns (0, 0) for an empty list.
func resolveSelection(procs []process.Info, pid, fallbackIdx int) (idx int, resolvedPID int) {
	if len(procs) == 0 {
		return 0, 0
	}
	for i, p := range procs {
		if p.PID == pid {
			return i, pid
		}
	}
	// PID not present — keep the cursor near its previous position.
	idx = clamp(fallbackIdx, 0, len(procs)-1)
	return idx, procs[idx].PID
}

// pidAt returns the PID of the process at idx, or 0 if out of range.
func pidAt(procs []process.Info, idx int) int {
	if idx < 0 || idx >= len(procs) {
		return 0
	}
	return procs[idx].PID
}

// clampScroll adjusts scrollOff so that selected stays within the viewport
// [scrollOff, scrollOff+visibleRows). Returns the new scrollOff.
func clampScroll(scrollOff, selected, visibleRows int) int {
	if selected < scrollOff {
		return selected
	}
	if selected >= scrollOff+visibleRows {
		return selected - visibleRows + 1
	}
	return scrollOff
}

// sortProcesses collects the swapped (>0) processes from pending and returns
// them ordered by the given mode.
func sortProcesses(pending map[int]process.Info, mode sortMode) []process.Info {
	procs := make([]process.Info, 0, len(pending))
	for _, p := range pending {
		if p.SwappedBytes > 0 {
			procs = append(procs, p)
		}
	}
	sortInPlace(procs, mode)
	return procs
}

// sortInPlace orders procs descending by the active sort key.
func sortInPlace(procs []process.Info, mode sortMode) {
	sort.Slice(procs, func(i, j int) bool {
		if mode == sortByRSS {
			return procs[i].RSSBytes > procs[j].RSSBytes
		}
		return procs[i].SwappedBytes > procs[j].SwappedBytes
	})
}

// sortedBySwap is a convenience wrapper for the default swap ordering.
func sortedBySwap(pending map[int]process.Info) []process.Info {
	return sortProcesses(pending, sortBySwap)
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
