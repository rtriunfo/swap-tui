package process

import (
	"bytes"
	"fmt"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// TopN returns the top n processes sorted by RSS, excluding pid 0 and 1.
func TopN(n int) ([]Info, error) {
	out, err := exec.Command("ps", "axo", "pid=,rss=,user=,comm=", "-m").Output()
	if err != nil {
		return nil, fmt.Errorf("ps: %w", err)
	}
	return parsePS(string(out), n, currentUsername()), nil
}

// parsePS is the pure parsing logic for ps output, separated for testability.
func parsePS(output string, n int, currentUser string) []Info {
	var procs []Info
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid <= 1 {
			continue
		}
		rssKB, _ := strconv.ParseInt(fields[1], 10, 64)
		owner := fields[2]
		name := filepath.Base(fields[3])

		category := "system"
		if owner == currentUser {
			category = "user"
		}

		procs = append(procs, Info{
			PID:      pid,
			Name:     name,
			RSSBytes: rssKB * 1024,
			Owner:    category,
		})
		if len(procs) >= n {
			break
		}
	}
	return procs
}

// Scan runs vmmap --summary for each process concurrently and streams
// ScanResults on the returned channel, which is closed when all are done.
func Scan(procs []Info) <-chan ScanResult {
	ch := make(chan ScanResult, len(procs))
	var wg sync.WaitGroup

	for _, p := range procs {
		wg.Add(1)
		go func(p Info) {
			defer wg.Done()
			swapped, err := vmmapSwapped(p.PID)
			p.SwappedBytes = swapped
			ch <- ScanResult{Info: p, Err: err}
		}(p)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	return ch
}

// ReadSwapStats returns system-wide swap usage via sysctl.
func ReadSwapStats() (SwapStats, error) {
	out, err := exec.Command("sysctl", "vm.swapusage").Output()
	if err != nil {
		return SwapStats{}, fmt.Errorf("sysctl: %w", err)
	}
	return ParseSwapStats(string(out))
}

// vmmapSwapped returns swapped bytes for pid.
// Tries without sudo first; falls back to sudo -n (non-interactive) for system processes.
func vmmapSwapped(pid int) (int64, error) {
	out, err := runVmmap(pid, false)
	if err != nil {
		out, err = runVmmap(pid, true)
		if err != nil {
			return 0, err
		}
	}
	return ParseVmmapOutput(string(out))
}

func runVmmap(pid int, useSudo bool) ([]byte, error) {
	pidStr := strconv.Itoa(pid)
	var cmd *exec.Cmd
	if useSudo {
		// -n prevents sudo from prompting; fails cleanly if no cached credential.
		cmd = exec.Command("sudo", "-n", "vmmap", "--summary", pidStr)
	} else {
		cmd = exec.Command("vmmap", "--summary", pidStr)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("vmmap pid=%d: %w", pid, err)
	}
	return stdout.Bytes(), nil
}

func currentUsername() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	return u.Username
}
