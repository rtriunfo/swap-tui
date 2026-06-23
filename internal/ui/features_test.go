package ui

import (
	"syscall"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"swap-tui/internal/process"
)

// -- Sort toggle -------------------------------------------------------------

func TestToggleSortByRSS(t *testing.T) {
	// testProcesses() is swap-descending; RSS order differs (BigApp has the most
	// RSS too here, so craft a case where swap and RSS orders diverge).
	procs := []process.Info{
		{PID: 1, Name: "A", SwappedBytes: 500, RSSBytes: 100},
		{PID: 2, Name: "B", SwappedBytes: 200, RSSBytes: 900},
		{PID: 3, Name: "C", SwappedBytes: 100, RSSBytes: 400},
	}
	m := makeModel(procs)

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	model := m2.(Model)

	if model.sortMode != sortByRSS {
		t.Fatalf("sortMode = %v, want sortByRSS", model.sortMode)
	}
	if model.processes[0].PID != 2 {
		t.Errorf("by RSS, first PID = %d, want 2 (highest RSS)", model.processes[0].PID)
	}

	// Toggle back to swap.
	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	model = m3.(Model)
	if model.sortMode != sortBySwap {
		t.Errorf("sortMode = %v, want sortBySwap after second toggle", model.sortMode)
	}
	if model.processes[0].PID != 1 {
		t.Errorf("by swap, first PID = %d, want 1 (highest swap)", model.processes[0].PID)
	}
}

func TestToggleSortKeepsSelection(t *testing.T) {
	procs := []process.Info{
		{PID: 1, Name: "A", SwappedBytes: 500, RSSBytes: 100},
		{PID: 2, Name: "B", SwappedBytes: 200, RSSBytes: 900},
		{PID: 3, Name: "C", SwappedBytes: 100, RSSBytes: 400},
	}
	m := makeModel(procs)
	m.selected = 1
	m.selectedPID = 2 // select "B"

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	model := m2.(Model)

	if model.selectedPID != 2 {
		t.Errorf("selectedPID = %d, want 2 (B stays selected)", model.selectedPID)
	}
	if model.processes[model.selected].PID != 2 {
		t.Errorf("selected row PID = %d, want 2", model.processes[model.selected].PID)
	}
}

// -- Filter ------------------------------------------------------------------

func TestFilterFlow(t *testing.T) {
	procs := []process.Info{
		{PID: 1, Name: "Chrome", SwappedBytes: 500},
		{PID: 2, Name: "chrome_helper", SwappedBytes: 200},
		{PID: 3, Name: "Safari", SwappedBytes: 100},
	}
	m := makeModel(procs)

	// Enter filter mode.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if m2.(Model).state != stateFilter {
		t.Fatalf("state = %v after /, want stateFilter", m2.(Model).state)
	}

	// Type "chr".
	model := m2.(Model)
	for _, r := range "chr" {
		next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		model = next.(Model)
	}
	if model.filter != "chr" {
		t.Errorf("filter = %q, want %q", model.filter, "chr")
	}

	vis := model.visibleProcesses()
	if len(vis) != 2 {
		t.Fatalf("visible = %d, want 2 (case-insensitive match)", len(vis))
	}

	// Enter applies and exits to ready.
	m3, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m3.(Model).state != stateReady {
		t.Errorf("state = %v after enter, want stateReady", m3.(Model).state)
	}
	if m3.(Model).filter != "chr" {
		t.Errorf("filter should persist after enter, got %q", m3.(Model).filter)
	}

	// Esc in ready clears the filter.
	m4, _ := m3.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m4.(Model).filter != "" {
		t.Errorf("esc should clear filter, got %q", m4.(Model).filter)
	}
}

func TestFilterBackspace(t *testing.T) {
	m := makeModel(testProcesses())
	m.state = stateFilter
	m.filter = "abc"

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m2.(Model).filter != "ab" {
		t.Errorf("filter = %q after backspace, want %q", m2.(Model).filter, "ab")
	}
}

func TestKillOperatesOnFilteredList(t *testing.T) {
	procs := []process.Info{
		{PID: 1, Name: "Chrome", SwappedBytes: 500},
		{PID: 2, Name: "Safari", SwappedBytes: 200},
	}
	m := makeModel(procs)
	m.filter = "safari"
	m.reconcileSelection()

	sel := m.selectedProcess()
	if sel == nil || sel.PID != 2 {
		t.Fatalf("selectedProcess on filtered list = %v, want PID 2", sel)
	}
}

// -- Force kill --------------------------------------------------------------

func TestForceKillUsesSIGKILL(t *testing.T) {
	m := makeModel(testProcesses())

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	model := m2.(Model)
	if model.state != stateConfirmKill {
		t.Fatalf("state = %v after X, want stateConfirmKill", model.state)
	}
	if model.pendingSig != syscall.SIGKILL {
		t.Errorf("pendingSig = %v, want SIGKILL", model.pendingSig)
	}
}

func TestNormalKillUsesSIGTERM(t *testing.T) {
	m := makeModel(testProcesses())
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if m2.(Model).pendingSig != syscall.SIGTERM {
		t.Errorf("pendingSig = %v, want SIGTERM", m2.(Model).pendingSig)
	}
}

func TestSignalName(t *testing.T) {
	if got := signalName(syscall.SIGKILL); got != "SIGKILL" {
		t.Errorf("signalName(SIGKILL) = %q", got)
	}
	if got := signalName(syscall.SIGTERM); got != "SIGTERM" {
		t.Errorf("signalName(SIGTERM) = %q", got)
	}
}

// -- View helpers ------------------------------------------------------------

func TestMaxSwapped(t *testing.T) {
	procs := []process.Info{
		{SwappedBytes: 100},
		{SwappedBytes: 900},
		{SwappedBytes: 400},
	}
	if got := maxSwapped(procs); got != 900 {
		t.Errorf("maxSwapped = %d, want 900", got)
	}
	if got := maxSwapped(nil); got != 0 {
		t.Errorf("maxSwapped(nil) = %d, want 0", got)
	}
}

func TestTrackedLine(t *testing.T) {
	procs := []process.Info{
		{SwappedBytes: 1024 * 1024},
		{SwappedBytes: 1024 * 1024},
	}
	got := trackedLine(procs, false)
	if got == "" {
		t.Fatal("trackedLine returned empty")
	}
	// Filtered variant uses a different label.
	if trackedLine(procs, true) == got {
		t.Error("filtered trackedLine should differ from unfiltered label")
	}
}
