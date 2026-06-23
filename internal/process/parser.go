package process

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var sizePattern = regexp.MustCompile(`^(\d+(?:\.\d+)?)([KMG]?)`)

// ParseVmmapOutput extracts swapped bytes from `vmmap --summary` output.
// It finds the character offset of "SWAPPED" in the column header line,
// then reads the value at that offset from the TOTAL summary line.
func ParseVmmapOutput(output string) (int64, error) {
	swappedCol := -1

	for _, line := range strings.Split(output, "\n") {
		if swappedCol == -1 {
			if strings.Contains(line, "VIRTUAL") && strings.Contains(line, "SWAPPED") {
				swappedCol = strings.Index(line, "SWAPPED")
			}
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "TOTAL") && swappedCol < len(line) {
			field := strings.TrimSpace(line[swappedCol:])
			m := sizePattern.FindStringSubmatch(field)
			if m == nil {
				return 0, nil
			}
			return parseSize(m[1], m[2])
		}
	}
	return 0, nil
}

// ParseSwapStats parses the output of `sysctl vm.swapusage`.
// Expected format: vm.swapusage: total = 4096.00M  used = 2726.31M  free = 1369.69M
func ParseSwapStats(output string) (SwapStats, error) {
	re := regexp.MustCompile(`(\w+)\s*=\s*(\d+(?:\.\d+)?[KMG]?)`)
	matches := re.FindAllStringSubmatch(output, -1)

	kv := make(map[string]string, len(matches))
	for _, m := range matches {
		kv[m[1]] = m[2]
	}

	var stats SwapStats
	var err error

	if v, ok := kv["total"]; ok {
		if stats.TotalBytes, err = parseSize(sizePattern.FindStringSubmatch(v)[1], sizePattern.FindStringSubmatch(v)[2]); err != nil {
			return SwapStats{}, fmt.Errorf("total: %w", err)
		}
	}
	if v, ok := kv["used"]; ok {
		m := sizePattern.FindStringSubmatch(v)
		if stats.UsedBytes, err = parseSize(m[1], m[2]); err != nil {
			return SwapStats{}, fmt.Errorf("used: %w", err)
		}
	}
	if v, ok := kv["free"]; ok {
		m := sizePattern.FindStringSubmatch(v)
		if stats.FreeBytes, err = parseSize(m[1], m[2]); err != nil {
			return SwapStats{}, fmt.Errorf("free: %w", err)
		}
	}

	return stats, nil
}

// parseSize converts a vmmap/sysctl size string like "305.2", "M" into bytes.
func parseSize(num, unit string) (int64, error) {
	f, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing %q: %w", num, err)
	}
	switch unit {
	case "K":
		return int64(f * 1024), nil
	case "M":
		return int64(f * 1024 * 1024), nil
	case "G":
		return int64(f * 1024 * 1024 * 1024), nil
	default:
		return int64(f), nil
	}
}
