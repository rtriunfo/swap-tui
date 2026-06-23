# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Building and Running

**Build the binary:**
```bash
go build -o swap-tui .
```

**Run directly (without building):**
```bash
go run .
```

**Run with custom flags:**
```bash
go run . -n 75                    # scan top 75 processes instead of default 50
go run . -version                 # print version and exit
sudo -v && go run .              # enable sudo for system process scanning
```

**Build with an embedded version string** (defaults to `dev`):
```bash
go build -ldflags "-X main.version=$(git describe --tags --always)" -o swap-tui .
```

## Continuous Integration

`.github/workflows/ci.yml` runs `go build`, `go vet`, and `go test ./...` on every push and pull request. It uses `runs-on: ubuntu-latest` — the unit tests are pure (no `vmmap`/`ps`/`sysctl` dependency) so they run on any platform, even though the app itself only functions on macOS.

## Testing

**Run all unit tests (no external tools required):**
```bash
go test ./...
```

**Run a single test:**
```bash
go test ./internal/process -run TestParsePS -v
```

**Run integration tests (includes vmmap on current process):**
```bash
INTEGRATION=1 go test ./... -run TestScanCurrentProcess -v
```

## Project Architecture

### Package Structure

The project is split into two independent packages:

- **`internal/process/`** — I/O and data parsing (no UI dependencies)
  - `types.go`: Core data structures (Info, ScanResult, SwapStats)
  - `scanner.go`: Runs external commands (ps, vmmap, sysctl) with concurrent I/O
  - `parser.go`: Pure parsing functions that take strings and return values
  - Tests: `scanner_test.go`, `parser_test.go`

- **`internal/ui/`** — TUI rendering and state machine (depends on process package)
  - `app.go`: Bubbletea Model implementation with state machine (scanning → ready → confirmKill / filter), plus sort, filter, and selection helpers
  - `view.go`: Renders the display (read-only, no state mutation)
  - `barchart.go`: Rendering helpers (Row, Header, FormatBytes)
  - `keys.go`: Key binding definitions
  - Tests: `app_test.go`, `features_test.go`, `barchart_test.go`

### Data Flow

1. **Initial scan**: `cmdStartScan` calls `process.TopN(n)` to get top N processes by RSS via `ps`
2. **Parallel vmmap calls**: `process.Scan()` spawns concurrent `vmmap --summary` for each process, streaming results on a channel
3. **UI accumulation**: Results trickle in via `scanResultMsg`; accumulated in `pending` map until all complete
4. **Display update**: When scan done, `pending` is filtered to non-zero swap and sorted by the active `sortMode`, then promoted to the `processes` slice (the full list)
5. **Auto-refresh**: Timer-based refresh every 30 seconds (configurable via `refreshInterval`)

### State Machine

The UI operates in four states (`stateScanning`, `stateReady`, `stateConfirmKill`, `stateFilter`):

- **stateScanning**: Results streaming in; display updates as they arrive
- **stateReady**: All results in; keyboard input (navigation, kill, sort, filter, refresh, quit)
- **stateConfirmKill**: Awaiting y/n confirmation before sending a signal (`Model.pendingSig` holds SIGTERM for `x` or SIGKILL for `X`)
- **stateFilter**: Typing a case-insensitive name filter; characters/backspace edit `Model.filter`, Enter/Esc returns to ready

State transitions are handled in `Model.handleKey()` and messages update state in `Model.Update()`. A refresh landing mid-prompt does not disturb an open confirm or filter (only `stateScanning` is promoted to `stateReady`).

### Selection, Sorting, and Filtering

- **PID-keyed selection**: `Model.selected` is a visible-list index, but the stable identity is `Model.selectedPID`. After any change to the visible list (scan, sort toggle, filter edit) `reconcileSelection()` re-derives the index from the PID so the cursor stays on the same process.
- **Sort modes**: `s` toggles `Model.sortMode` between `sortBySwap` and `sortByRSS`; the current list is re-sorted in place immediately via `sortInPlace`.
- **Filtering is display-time**: `processes` always holds the full list; `visibleProcesses()` applies the `filter` substring at render/navigation time, so a background refresh never drops the filter. Selection, scrolling, and kill all operate on the filtered slice.
- **Scrolling viewport**: `visibleRows()` sizes the window from terminal height; `clampScroll()` keeps the selected row on screen, with `↑ N more` / `↓ N more` indicators.
- **Bar normalization**: bars are normalized against the maximum `SwappedBytes` in the visible set (`maxSwapped`), independent of the active sort key.

### Pure Functions in Parser

`parser.go` functions (`ParseVmmapOutput`, `ParseSwapStats`) take strings and return values—no I/O, no side effects. This design makes them fast and easy to test. `scanner.go` tests include integration tests that verify the full end-to-end pipeline with real `vmmap` calls.

## Key Implementation Notes

- **Concurrency model**: Each process gets a goroutine for its `vmmap --summary` call; results stream on a channel
- **Column-based parsing**: `vmmap` output is parsed by finding the "SWAPPED" column header and extracting the value at that offset from the TOTAL line
- **User vs. system filtering**: Processes owned by the current user are marked "user"; others are "system" (used for color coding)
- **Swap filtering**: Only processes with SwappedBytes > 0 appear in the display
- **Kill signals**: `x` sends SIGTERM, `X` sends SIGKILL; both go through the y/n confirm flow and trigger an immediate refresh (`killDoneMsg`) so the dying process drops out promptly
- **Error handling**: `vmmap` failures per process are silently skipped (some system processes reject it even with sudo); system errors (ps, sysctl) are surfaced in the UI
