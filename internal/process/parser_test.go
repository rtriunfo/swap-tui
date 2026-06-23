package process

import (
	"testing"
)

// mib / gib force runtime evaluation so Go doesn't reject fractional
// float-constant-to-int64 conversions at compile time.
func mib(n float64) int64 { return int64(n * 1024 * 1024) }
func gib(n float64) int64 { return int64(n * 1024 * 1024 * 1024) }

func TestParseVmmapOutput(t *testing.T) {
	header := "                                VIRTUAL RESIDENT    DIRTY  SWAPPED VOLATILE   NONVOL    EMPTY   REGION \n" +
		"REGION TYPE                        SIZE     SIZE     SIZE     SIZE     SIZE     SIZE     SIZE    COUNT (non-coalesced) \n" +
		"===========                     ======= ========    =====  ======= ========   ======    =====  ======= \n"

	tests := []struct {
		name      string
		input     string
		wantBytes int64
	}{
		{
			name:      "zero swap (0K)",
			input:     header + "TOTAL                            820.1M   154.6M    2720K       0K       0K      16K       0K      291 ",
			wantBytes: 0,
		},
		{
			name:      "megabytes swap",
			input:     header + "TOTAL                            820.1M   154.6M    2720K   305.2M       0K      16K       0K      291 ",
			wantBytes: mib(305.2),
		},
		{
			name:      "gigabytes swap",
			input:     header + "TOTAL                           5820.1M  2354.6M    8512K     1.6G       0K    2048K       0K      891 ",
			wantBytes: gib(1.6),
		},
		{
			name:      "kilobytes swap",
			input:     header + "TOTAL                            820.1M   154.6M    2720K     512K       0K      16K       0K      291 ",
			wantBytes: 512 * 1024,
		},
		{
			name:      "no TOTAL line",
			input:     header,
			wantBytes: 0,
		},
		{
			name:      "no header line",
			input:     "some random\noutput without the column header",
			wantBytes: 0,
		},
		{
			name:      "empty output",
			input:     "",
			wantBytes: 0,
		},
		{
			name: "real-world zero-swap output with preamble",
			input: `ReadOnly portion of Libraries: Total=580.0M resident=82.9M(14%) swapped_out_or_unallocated=497.1M(86%)
Writable regions: Total=101.0M written=240K(0%) resident=2224K(2%) swapped_out=0K(0%) unallocated=98.9M(98%)

` + header + "TOTAL                            820.1M   154.6M    2720K       0K       0K      16K       0K      291 ",
			wantBytes: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVmmapOutput(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// Allow ±1 KB tolerance for floating-point rounding.
			diff := got - tt.wantBytes
			if diff < 0 {
				diff = -diff
			}
			if diff > 1024 {
				t.Errorf("got %d bytes, want %d bytes (diff %d)", got, tt.wantBytes, diff)
			}
		})
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		num  string
		unit string
		want int64
	}{
		{"100", "K", 100 * 1024},
		{"2.5", "M", mib(2.5)},
		{"1.6", "G", gib(1.6)},
		{"512", "", 512},
		{"0", "K", 0},
		{"4096", "M", 4096 * 1024 * 1024},
	}

	for _, tt := range tests {
		got, err := parseSize(tt.num, tt.unit)
		if err != nil {
			t.Errorf("parseSize(%q, %q) unexpected error: %v", tt.num, tt.unit, err)
			continue
		}
		diff := got - tt.want
		if diff < 0 {
			diff = -diff
		}
		if diff > 1024 {
			t.Errorf("parseSize(%q, %q) = %d, want %d", tt.num, tt.unit, got, tt.want)
		}
	}
}

func TestParseSwapStats(t *testing.T) {
	input := "vm.swapusage: total = 4096.00M  used = 2726.31M  free = 1369.69M  (encrypted)\n"

	stats, err := ParseSwapStats(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantTotal := int64(4096 * 1024 * 1024)
	wantUsed := mib(2726.31)
	wantFree := mib(1369.69)

	check := func(name string, got, want int64) {
		diff := got - want
		if diff < 0 {
			diff = -diff
		}
		if diff > 1024*1024 { // 1 MB tolerance
			t.Errorf("%s: got %d, want ~%d", name, got, want)
		}
	}

	check("TotalBytes", stats.TotalBytes, wantTotal)
	check("UsedBytes", stats.UsedBytes, wantUsed)
	check("FreeBytes", stats.FreeBytes, wantFree)
}

func TestParseSwapStatsMalformedInput(t *testing.T) {
	// Empty / garbage input should return zero stats with no error (fields simply absent).
	t.Run("empty input returns zero stats no error", func(t *testing.T) {
		stats, err := ParseSwapStats("")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if stats.TotalBytes != 0 || stats.UsedBytes != 0 || stats.FreeBytes != 0 {
			t.Errorf("expected zero stats, got %+v", stats)
		}
	})

	t.Run("completely unrelated output returns zero stats no error", func(t *testing.T) {
		stats, err := ParseSwapStats("kern.ipc.maxsockbuf: 8388608\nhw.physmem: 17179869184\n")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if stats.TotalBytes != 0 || stats.UsedBytes != 0 || stats.FreeBytes != 0 {
			t.Errorf("expected zero stats, got %+v", stats)
		}
	})

	// Directly call ParseSwapStats with a crafted kv scenario: the outer regex
	// requires \d+ so any captured value starts with digits and sizePattern
	// always matches. Verify the nil guard in the code path by calling
	// parseSize directly with an empty string, which causes an error.
	t.Run("parseSize rejects empty number string", func(t *testing.T) {
		_, err := parseSize("", "M")
		if err == nil {
			t.Error("expected error from parseSize with empty num, got nil")
		}
	})
}
