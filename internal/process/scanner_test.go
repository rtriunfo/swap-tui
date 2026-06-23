package process

import (
	"fmt"
	"os"
	"testing"
)

func TestParsePS(t *testing.T) {
	const currentUser = "alice"

	input := `  501  204800 alice  /Applications/Firefox.app/Contents/MacOS/firefox
  502  102400 alice  /Applications/Slack.app/Contents/MacOS/Slack
    1    8192 root   /sbin/launchd
    0    4096 root   kernel_task
  503   51200 _www   /usr/sbin/httpd
`

	procs := parsePS(input, 10, currentUser)

	if len(procs) != 3 {
		t.Fatalf("got %d processes, want 3 (pid 0 and 1 should be excluded)", len(procs))
	}

	// Verify first process
	p := procs[0]
	if p.PID != 501 {
		t.Errorf("procs[0].PID = %d, want 501", p.PID)
	}
	if p.Name != "firefox" {
		t.Errorf("procs[0].Name = %q, want %q", p.Name, "firefox")
	}
	if p.RSSBytes != 204800*1024 {
		t.Errorf("procs[0].RSSBytes = %d, want %d", p.RSSBytes, 204800*1024)
	}
	if p.Owner != "user" {
		t.Errorf("procs[0].Owner = %q, want %q", p.Owner, "user")
	}

	// System process should be categorised correctly
	sys := procs[2]
	if sys.Owner != "system" {
		t.Errorf("procs[2].Owner = %q, want %q", sys.Owner, "system")
	}
}

func TestParsePSRespectsTopN(t *testing.T) {
	lines := ""
	for i := 2; i <= 10; i++ {
		lines += fmt.Sprintf("  %d  1000 alice /bin/proc%d\n", i, i)
	}

	procs := parsePS(lines, 3, "alice")
	if len(procs) != 3 {
		t.Errorf("got %d processes, want 3", len(procs))
	}
}

func TestParsePSEmptyInput(t *testing.T) {
	procs := parsePS("", 25, "alice")
	if len(procs) != 0 {
		t.Errorf("got %d processes, want 0", len(procs))
	}
}

// TestScanCurrentProcess is an integration test that runs vmmap on the
// current process to verify end-to-end parsing works.
// Run with: go test -run TestScanCurrentProcess -v
func TestScanCurrentProcess(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("set INTEGRATION=1 to run")
	}

	procs := []Info{{PID: os.Getpid(), Name: "test", Owner: "user"}}
	ch := Scan(procs)

	result := <-ch
	if result.Err != nil {
		t.Fatalf("scan error: %v", result.Err)
	}
	// SwappedBytes can legitimately be 0 — just check it doesn't error.
	t.Logf("current process swapped: %d bytes", result.Info.SwappedBytes)
}
