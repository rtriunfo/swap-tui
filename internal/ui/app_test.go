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
		3: {PID: 3, SwappedBytes: 0}, // zero swap — should be excluded
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

// -- PID-keyed selection (Change 1) ------------------------------------------

func TestNavigationKeepsPIDInSync(t *testing.T) {
	m := makeModel(testProcesses())

	// Move down one row; selectedPID should track.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model := m2.(Model)
	if model.selected != 1 {
		t.Fatalf("selected = %d, want 1", model.selected)
	}
	if model.selectedPID != testProcesses()[1].PID {
		t.Errorf("selectedPID = %d, want %d", model.selectedPID, testProcesses()[1].PID)
	}
}

func TestResolveSelectionPIDPresent(t *testing.T) {
	procs := testProcesses()
	idx, pid := resolveSelection(procs, 200) // PID 200 is at index 1
	if idx != 1 || pid != 200 {
		t.Errorf("got (%d, %d), want (1, 200)", idx, pid)
	}
}

func TestResolveSelectionPIDGone(t *testing.T) {
	procs := testProcesses()
	// PID 999 does not exist → should clamp to last row.
	idx, pid := resolveSelection(procs, 999)
	last := len(procs) - 1
	if idx != last {
		t.Errorf("idx = %d, want %d (last row)", idx, last)
	}
	if pid != procs[last].PID {
		t.Errorf("pid = %d, want %d", pid, procs[last].PID)
	}
}

func TestResolveSelectionEmptyList(t *testing.T) {
	idx, pid := resolveSelection(nil, 100)
	if idx != 0 || pid != 0 {
		t.Errorf("got (%d, %d), want (0, 0)", idx, pid)
	}
}

// After a scan refresh the highlighted process should be the same PID even
// if the list order changed.
func TestScanPreservesSelectedPID(t *testing.T) {
	m := makeModel(testProcesses())
	m.selected = 1
	m.selectedPID = testProcesses()[1].PID // PID 200

	// Simulate a scan completing with same processes in a different map order.
	// pending contains PID 200 still present.
	done := scanResultMsg{done: true}
	// Pre-load pending with the same three processes.
	m.pending = map[int]process.Info{
		100: testProcesses()[0],
		200: testProcesses()[1],
		300: testProcesses()[2],
	}
	m2, _ := m.Update(done)
	final := m2.(Model)

	if final.selectedPID != 200 {
		t.Errorf("selectedPID = %d after refresh, want 200", final.selectedPID)
	}
	if final.processes[final.selected].PID != 200 {
		t.Errorf("processes[selected].PID = %d, want 200", final.processes[final.selected].PID)
	}
}

// -- Scrolling helpers (Change 2) --------------------------------------------

func TestClampScroll(t *testing.T) {
	tests := []struct {
		scrollOff, selected, visible, want int
	}{
		{0, 0, 5, 0},  // selection at top, no scroll needed
		{0, 4, 5, 0},  // selection at bottom of viewport, no scroll
		{0, 5, 5, 1},  // selection just off the bottom → scroll down
		{3, 2, 5, 2},  // selection above viewport → scroll up
		{0, 10, 5, 6}, // selection well below viewport
	}
	for _, tt := range tests {
		got := clampScroll(tt.scrollOff, tt.selected, tt.visible)
		if got != tt.want {
			t.Errorf("clampScroll(%d,%d,%d) = %d, want %d",
				tt.scrollOff, tt.selected, tt.visible, got, tt.want)
		}
	}
}

func TestVisibleRowsFallback(t *testing.T) {
	m := makeModel(nil)
	m.height = 0 // not yet received a WindowSizeMsg
	if m.visibleRows() != defaultVisibleRows {
		t.Errorf("visibleRows() = %d with height=0, want %d", m.visibleRows(), defaultVisibleRows)
	}
}

func TestScrollFollowsSelection(t *testing.T) {
	// Build a model with 10 processes and a small viewport.
	procs := make([]process.Info, 10)
	for i := range procs {
		procs[i] = process.Info{PID: i + 1, Name: "p", SwappedBytes: int64(10-i) * 1024 * 1024}
	}
	m := makeModel(procs)
	m.height = 13 // chrome=8 → visible=5

	// Navigate down past the initial viewport.
	cur := m
	for i := 0; i < 6; i++ {
		next, _ := cur.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		cur = next.(Model)
	}

	// selected should be 6, and scrollOff should keep it in view.
	if cur.selected != 6 {
		t.Errorf("selected = %d, want 6", cur.selected)
	}
	visible := cur.visibleRows()
	if cur.selected < cur.scrollOff || cur.selected >= cur.scrollOff+visible {
		t.Errorf("selected %d not in viewport [%d, %d)",
			cur.selected, cur.scrollOff, cur.scrollOff+visible)
	}
}

// -- Kill triggers refresh (Change 5) ----------------------------------------

func TestKillDoneMsgTriggersRefresh(t *testing.T) {
	m := makeModel(testProcesses())
	m2, cmd := m.Update(killDoneMsg{})
	model := m2.(Model)

	if !model.scanning {
		t.Error("killDoneMsg should put the model into scanning state")
	}
	if cmd == nil {
		t.Error("killDoneMsg should return a scan command")
	}
}
