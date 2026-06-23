package ui

import (
	"strings"
	"testing"

	"swap-tui/internal/process"
)

func mib(n float64) int64 { return int64(n * 1024 * 1024) }
func gib(n float64) int64 { return int64(n * 1024 * 1024 * 1024) }

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{mib(305.2), "305.2 MB"},
		{gib(1.6), "1.6 GB"},
	}

	for _, tt := range tests {
		got := FormatBytes(tt.bytes)
		if got != tt.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestBarWidth(t *testing.T) {
	tests := []struct {
		termWidth int
		want      int
	}{
		{40, 10},                // minimum clamp
		{80, 80 - fixedWidth},   // exact calculation
		{120, 120 - fixedWidth}, // exact calculation
		{200, 60},               // maximum clamp
	}

	for _, tt := range tests {
		got := BarWidth(tt.termWidth)
		if got != tt.want {
			t.Errorf("BarWidth(%d) = %d, want %d", tt.termWidth, got, tt.want)
		}
	}
}

func TestBarFilled(t *testing.T) {
	tests := []struct {
		swapped  int64
		maxSwap  int64
		barWidth int
		want     int
	}{
		{0, 100, 20, 0},
		{100, 100, 20, 20}, // 100% → full bar
		{50, 100, 20, 10},  // 50% → half bar
		{305, 1638, 40, 7}, // ~18.6% of 40 = 7
		{0, 0, 20, 0},      // zero maxSwap
		{100, 100, 0, 0},   // zero barWidth
	}

	for _, tt := range tests {
		got := barFilled(tt.swapped, tt.maxSwap, tt.barWidth)
		if got != tt.want {
			t.Errorf("barFilled(%d, %d, %d) = %d, want %d",
				tt.swapped, tt.maxSwap, tt.barWidth, got, tt.want)
		}
	}
}

func TestSwapCategory(t *testing.T) {
	tests := []struct {
		name string
		p    process.Info
		want string
	}{
		{"system process", process.Info{Owner: "system", SwappedBytes: 500 * 1024 * 1024}, "system"},
		{"user >100MB", process.Info{Owner: "user", SwappedBytes: 200 * 1024 * 1024}, "worth"},
		{"user =100MB boundary", process.Info{Owner: "user", SwappedBytes: 100 * 1024 * 1024}, "worth"},
		{"user <100MB", process.Info{Owner: "user", SwappedBytes: 10 * 1024 * 1024}, "small"},
		{"user 0 bytes", process.Info{Owner: "user", SwappedBytes: 0}, "small"},
		{"system large swap still system", process.Info{Owner: "system", SwappedBytes: 2 * 1024 * 1024 * 1024}, "system"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SwapCategory(tt.p)
			if got != tt.want {
				t.Errorf("SwapCategory() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 8, "hello..."},
		{"ab", 2, "ab"},
		{"abc", 2, "ab"}, // max ≤ 3: just truncate, no ellipsis
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.max)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
		}
		if len(got) > tt.max {
			t.Errorf("truncate(%q, %d) result %q exceeds max %d", tt.input, tt.max, got, tt.max)
		}
	}
}

func TestRowContainsExpectedFields(t *testing.T) {
	p := process.Info{
		PID:          1234,
		Name:         "MyApp",
		SwappedBytes: 100 * 1024 * 1024,
		RSSBytes:     200 * 1024 * 1024,
		Owner:        "user",
	}
	row := Row(p, 200*1024*1024, 20, false)

	checks := []string{"1234", "MyApp", "100.0 MB", "200.0 MB"}
	for _, s := range checks {
		if !strings.Contains(row, s) {
			t.Errorf("Row() output missing %q\nGot: %q", s, row)
		}
	}
}

func TestHeaderAndDividerSameWidth(t *testing.T) {
	for _, bw := range []int{10, 20, 40, 60} {
		h := Header(bw)
		d := Divider(bw)
		// Strip ANSI codes by comparing visible length via rune count (no ANSI here)
		if len([]rune(h)) != len([]rune(d)) {
			t.Errorf("barWidth=%d: Header len=%d, Divider len=%d — they should match",
				bw, len([]rune(h)), len([]rune(d)))
		}
	}
}
