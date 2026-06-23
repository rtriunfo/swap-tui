package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"swap-tui/internal/process"
)

// makeModel returns a ready Model with test processes pre-loaded.
func makeModel(procs []process.Info) Model {
	m := New(0)
	m.processes = procs
	m.scanning = false
	m.state = stateReady
	m.width = 120
	m.height = 40
	return m
}

func testProcesses() []process.Info {
	return []process.Info{
		{PID: 100, Name: "BigApp", SwappedBytes: 500 * 1024 * 1024, RSSBytes: 1024 * 1024 * 1024, Owner: "system"},
		{PID: 200, Name: "MedApp", SwappedBytes: 200 * 1024 * 1024, RSSBytes: 300 * 1024 * 1024, Owner: "user"},
		{PID: 300, Name: "SmallApp", SwappedBytes: 10 * 1024 * 1024, RSSBytes: 50 * 1024 * 1024, Owner: "user"},
	}
}

// -- sortedBySwap -------------------------------------------------------------

func TestSortedBySwap(t *testing.T) {
	pending := map[int]process.Info{
		1: {PID: 1, SwappedBytes: 100},
		2: {PID: 2, SwappedBytes: 500},
		3: {PID: 3, SwappedBytes: 0},   // zero swap — should be excluded
		4: {PID: 4, SwappedBytes: 200},
	}

	got := sortedBySwap(pending)

	if len(got) != 3 {
		t.Fatalf("got %d processes, want 3 (zero-swap entry excluded)", len(got))
	}
	if got[0].SwappedBytes < got[1].SwappedBytes || got[1].SwappedBytes < got[2].SwappedBytes {
		t.Errorf("processes not sorted descending by swap: %v", got)
	}
	if got[0].PID != 2 {
		t.Errorf("first process PID = %d, want 2 (highest swap)", got[0].PID)
	}
}

func TestSortedBySwapEmptyMap(t *testing.T) {
	got := sortedBySwap(map[int]process.Info{})
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(got))
	}
}

// -- clamp -------------------------------------------------------------------

func TestClamp(t *testing.T) {
	tests := []struct{ v, lo, hi, want int }{
		{5, 0, 10, 5},
		{-1, 0, 10, 0},
		{15, 0, 10, 10},
		{0, 0, 0, 0},
		{3, 5, 2, 5}, // hi < lo → return lo
	}
	for _, tt := range tests {
		got := clamp(tt.v, tt.lo, tt.hi)
		if got != tt.want {
			t.Errorf("clamp(%d,%d,%d) = %d, want %d", tt.v, tt.lo, tt.hi, got, tt.want)
		}
	}
}

// -- Update: navigation -------------------------------------------------------

func TestNavigationUpDown(t *testing.T) {
	m := makeModel(testProcesses())

	// Move down twice.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})

	model := m3.(Model)
	if model.selected != 2 {
		t.Errorf("after 2 downs, selected = %d, want 2", model.selected)
	}

	// Move back up.
	m4, _ := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model = m4.(Model)
	if model.selected != 1 {
		t.Errorf("after up, selected = %d, want 1", model.selected)
	}
}

func TestNavigationClampsAtBounds(t *testing.T) {
	m := makeModel(testProcesses())

	// Try to go above first item.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m2.(Model).selected != 0 {
		t.Errorf("selected should stay at 0 when pressing up at top")
	}

	// Move to last item then try to go further.
	m3 := makeModel(testProcesses())
	m3.selected = 2
	m4, _ := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m4.(Model).selected != 2 {
		t.Errorf("selected should stay at 2 when pressing down at bottom")
	}
}

// -- Update: kill flow -------------------------------------------------------

func TestKillFlowConfirm(t *testing.T) {
	m := makeModel(testProcesses())

	// Press x to enter confirm state.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if m2.(Model).state != stateConfirmKill {
		t.Errorf("state = %v after x, want stateConfirmKill", m2.(Model).state)
	}

	// Press y to confirm — should return to ready and emit a kill cmd.
	m3, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if m3.(Model).state != stateReady {
		t.Errorf("state = %v after y, want stateReady", m3.(Model).state)
	}
	if cmd == nil {
		t.Errorf("expected a kill command after confirming, got nil")
	}
}

func TestKillFlowCancel(t *testing.T) {
	m := makeModel(testProcesses())
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m3, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})

	if m3.(Model).state != stateReady {
		t.Errorf("state = %v after n, want stateReady", m3.(Model).state)
	}
	if cmd != nil {
		t.Errorf("cancel should produce no command, got %v", cmd)
	}
}

func TestKillIgnoredOnEmptyList(t *testing.T) {
	m := makeModel(nil) // no processes
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if m2.(Model).state != stateReady {
		t.Errorf("x on empty list should not enter confirm state")
	}
}

// -- Update: scan results ----------------------------------------------------

func TestScanResultAccumulatesAndSorts(t *testing.T) {
	m := New(0)
	m.pending = make(map[int]process.Info)

	// Simulate receiving scan results out of order.
	r1 := scanResultMsg{result: process.ScanResult{
		Info: process.Info{PID: 1, Name: "small", SwappedBytes: 10, Owner: "user"},
	}}
	r2 := scanResultMsg{result: process.ScanResult{
		Info: process.Info{PID: 2, Name: "big", SwappedBytes: 1000, Owner: "system"},
	}}
	done := scanResultMsg{done: true}

	m2, _ := m.Update(r1)
	m3, _ := m2.Update(r2)
	m4, _ := m3.Update(done)

	final := m4.(Model)
	if final.scanning {
		t.Error("scanning should be false after done message")
	}
	if len(final.processes) != 2 {
		t.Fatalf("got %d processes, want 2", len(final.processes))
	}
	if final.processes[0].PID != 2 {
		t.Errorf("first process should be PID 2 (highest swap), got %d", final.processes[0].PID)
	}
}

// -- Update: window resize ---------------------------------------------------

func TestWindowResize(t *testing.T) {
	m := makeModel(nil)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 150, Height: 50})
	model := m2.(Model)
	if model.width != 150 || model.height != 50 {
		t.Errorf("got (%d, %d), want (150, 50)", model.width, model.height)
	}
}
