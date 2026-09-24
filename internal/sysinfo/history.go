package sysinfo

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"
)

// Point is one sample in the Usage page's recent history.
type Point struct {
	At         string  `json:"at"`
	CPUPercent float64 `json:"cpu_percent"`
	MemUsed    uint64  `json:"mem_used"`
	RxPerSec   float64 `json:"rx_per_sec"` // bytes/s received
	TxPerSec   float64 `json:"tx_per_sec"` // bytes/s sent
}

const historyLen = 90 // 15 minutes at one sample every 10s

// netBytes sums received/sent bytes over the container's interfaces
// (loopback excluded) from /proc/net/dev.
func netBytes() (rx, tx uint64, ok bool) {
	b, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return 0, 0, false
	}
	for _, line := range strings.Split(string(b), "\n") {
		name, rest, found := strings.Cut(line, ":")
		name = strings.TrimSpace(name)
		if !found || name == "lo" {
			continue
		}
		f := strings.Fields(rest)
		if len(f) < 9 {
			continue
		}
		r, _ := strconv.ParseUint(f[0], 10, 64)
		t, _ := strconv.ParseUint(f[8], 10, 64)
		rx, tx = rx+r, tx+t
	}
	return rx, tx, true
}

// sampleNet updates the network rate from the previous reading.
func (s *Sampler) sampleNet(now time.Time) (float64, float64) {
	rx, tx, ok := netBytes()
	s.mu.Lock()
	defer s.mu.Unlock()
	if ok && !s.netAt.IsZero() {
		if dt := now.Sub(s.netAt).Seconds(); dt >= 0.5 && rx >= s.lastRx && tx >= s.lastTx {
			s.rxRate, s.txRate = float64(rx-s.lastRx)/dt, float64(tx-s.lastTx)/dt
		}
	}
	if ok && now.Sub(s.netAt) >= 500*time.Millisecond {
		s.lastRx, s.lastTx, s.netAt = rx, tx, now
	}
	return s.rxRate, s.txRate
}

// History returns the recent samples, oldest first.
func (s *Sampler) History() []Point {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Point{}, s.history...)
}

// Loop samples usage every 10 seconds for the history charts.
func (s *Sampler) Loop(ctx context.Context) {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for {
		snap := s.Snapshot()
		p := Point{At: time.Now().UTC().Format(time.RFC3339), CPUPercent: snap.CPUPercent,
			MemUsed: snap.MemUsed, RxPerSec: snap.RxPerSec, TxPerSec: snap.TxPerSec}
		s.mu.Lock()
		s.history = append(s.history, p)
		if len(s.history) > historyLen {
			s.history = s.history[len(s.history)-historyLen:]
		}
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
