package process

// Info holds the data we care about for a single process.
type Info struct {
	PID          int
	Name         string
	SwappedBytes int64
	RSSBytes     int64
	Owner        string // "user" or "system"
}

// ScanResult is sent on the results channel as each vmmap call completes.
type ScanResult struct {
	Info Info
	Err  error
}

// SwapStats holds system-wide swap usage from sysctl.
type SwapStats struct {
	TotalBytes int64
	UsedBytes  int64
	FreeBytes  int64
}
