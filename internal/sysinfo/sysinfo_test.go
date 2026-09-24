package sysinfo

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestSnapshot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("reads /proc and cgroups, Linux only")
	}
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "novelcheck.db"), make([]byte, 4096), 0o600)
	s := New(dir)
	s.Snapshot()
	busy := time.Now().Add(600 * time.Millisecond)
	for time.Now().Before(busy) { // burn some CPU so the rate is measurable
	}
	snap := s.Snapshot()
	if snap.CPUCores < 1 || snap.MemUsed == 0 || snap.MemLimit == 0 || snap.DBSize != 4096 || snap.DiskTotal == 0 {
		t.Fatalf("unexpected snapshot %+v", snap)
	}
	if snap.CPUPercent <= 0 || snap.CPUPercent > 100 {
		t.Fatalf("cpu percent out of range: %v", snap.CPUPercent)
	}
}

func TestNetworkAndHistory(t *testing.T) {
	s := New(t.TempDir())
	if _, _, ok := netBytes(); !ok {
		t.Skip("/proc/net/dev not available")
	}
	s.Snapshot()
	time.Sleep(600 * time.Millisecond)
	snap := s.Snapshot()
	if snap.RxPerSec < 0 || snap.TxPerSec < 0 {
		t.Fatalf("negative network rate: %+v", snap)
	}
	s.mu.Lock()
	for i := 0; i < historyLen+5; i++ {
		s.history = append(s.history, Point{})
	}
	s.mu.Unlock()
	if h := s.History(); len(h) != historyLen+5 {
		t.Fatalf("history copy: %d", len(h))
	}
}
