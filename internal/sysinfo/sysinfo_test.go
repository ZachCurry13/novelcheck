package sysinfo

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSnapshot(t *testing.T) {
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
