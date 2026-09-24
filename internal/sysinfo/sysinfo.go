// Package sysinfo reports NovelCheck's own resource use for the Usage page:
// CPU and memory of its container (Linux cgroup v2, falling back to the
// process), plus disk space and database size for the data folder.
package sysinfo

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Snapshot struct {
	UptimeSeconds int64   `json:"uptime_seconds"`
	CPUPercent    float64 `json:"cpu_percent"` // share of all cores, 0-100
	CPUCores      int     `json:"cpu_cores"`
	MemUsed       uint64  `json:"mem_used"`
	MemLimit      uint64  `json:"mem_limit"` // container limit, or host RAM if unlimited
	MemIsLimit    bool    `json:"mem_is_container_limit"`
	DiskFree      uint64  `json:"disk_free"`
	DiskTotal     uint64  `json:"disk_total"`
	DBSize        uint64  `json:"db_size"`
	Goroutines    int     `json:"goroutines"`
	RxPerSec      float64 `json:"rx_per_sec"` // network download, bytes/s
	TxPerSec      float64 `json:"tx_per_sec"` // network upload, bytes/s
}

// Sampler remembers the previous CPU reading so it can report a rate.
type Sampler struct {
	DataDir string
	start   time.Time

	mu       sync.Mutex
	lastCPU  float64 // CPU seconds used
	lastWall time.Time
	lastPct  float64

	lastRx, lastTx uint64
	netAt          time.Time
	rxRate, txRate float64
	history        []Point
}

func New(dataDir string) *Sampler { return &Sampler{DataDir: dataDir, start: time.Now()} }

func (s *Sampler) Snapshot() Snapshot {
	snap := Snapshot{
		UptimeSeconds: int64(time.Since(s.start).Seconds()),
		CPUCores:      runtime.NumCPU(),
		Goroutines:    runtime.NumGoroutine(),
	}
	snap.CPUPercent = s.cpuPercent(snap.CPUCores)
	snap.RxPerSec, snap.TxPerSec = s.sampleNet(time.Now())
	snap.MemUsed, snap.MemLimit, snap.MemIsLimit = memory()
	snap.DiskFree, snap.DiskTotal = diskSpace(s.DataDir)
	for _, f := range []string{"novelcheck.db", "novelcheck.db-wal", "novelcheck.db-shm"} {
		if fi, err := os.Stat(filepath.Join(s.DataDir, f)); err == nil {
			snap.DBSize += uint64(fi.Size())
		}
	}
	return snap
}

// cpuPercent is the CPU used since the previous call, as a share of all cores.
func (s *Sampler) cpuPercent(cores int) float64 {
	used, ok := cpuSeconds()
	if !ok {
		return 0
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.lastWall.IsZero() {
		if wall := now.Sub(s.lastWall).Seconds(); wall >= 0.5 && cores > 0 {
			s.lastPct = min(100, max(0, (used-s.lastCPU)/wall/float64(cores)*100))
		}
	}
	if now.Sub(s.lastWall) >= 500*time.Millisecond {
		s.lastCPU, s.lastWall = used, now
	}
	return s.lastPct
}

// cpuSeconds reads total CPU time of the container (cgroup v2), or of this
// process if cgroup data isn't available.
func cpuSeconds() (float64, bool) {
	if b, err := os.ReadFile("/sys/fs/cgroup/cpu.stat"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if f := strings.Fields(line); len(f) == 2 && f[0] == "usage_usec" {
				if v, err := strconv.ParseFloat(f[1], 64); err == nil {
					return v / 1e6, true
				}
			}
		}
	}
	b, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0, false
	}
	// Fields after the ")" that closes the command name; utime/stime are 14 and 15.
	rest := string(b)
	if i := strings.LastIndexByte(rest, ')'); i >= 0 {
		rest = rest[i+1:]
	}
	f := strings.Fields(rest)
	if len(f) < 13 {
		return 0, false
	}
	ut, _ := strconv.ParseFloat(f[11], 64)
	st, _ := strconv.ParseFloat(f[12], 64)
	return (ut + st) / 100, true // USER_HZ is 100 on Linux
}

// memory returns used bytes and the limit: the container's memory.max when
// set, otherwise the host's total RAM.
func memory() (used, limit uint64, isContainer bool) {
	if v, ok := readUint("/sys/fs/cgroup/memory.current"); ok {
		used = v
		// Page cache is reclaimable; report what's actually held.
		if b, err := os.ReadFile("/sys/fs/cgroup/memory.stat"); err == nil {
			for _, line := range strings.Split(string(b), "\n") {
				if f := strings.Fields(line); len(f) == 2 && f[0] == "inactive_file" {
					if c, err := strconv.ParseUint(f[1], 10, 64); err == nil && c < used {
						used -= c
					}
				}
			}
		}
		if l, ok := readUint("/sys/fs/cgroup/memory.max"); ok {
			return used, l, true
		}
	}
	if used == 0 {
		used = procStatusKB("VmRSS") * 1024
	}
	if b, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if f := strings.Fields(line); len(f) >= 2 && f[0] == "MemTotal:" {
				kb, _ := strconv.ParseUint(f[1], 10, 64)
				limit = kb * 1024
			}
		}
	}
	return used, limit, false
}

func readUint(path string) (uint64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
	return v, err == nil // "max" (no limit) fails to parse
}

func procStatusKB(key string) uint64 {
	b, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		if f := strings.Fields(line); len(f) >= 2 && f[0] == key+":" {
			v, _ := strconv.ParseUint(f[1], 10, 64)
			return v
		}
	}
	return 0
}
