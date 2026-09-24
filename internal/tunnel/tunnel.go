// Package tunnel runs Cloudflare's connector (cloudflared) inside the
// NovelCheck container so the app can be reached from outside the home
// network without opening router ports. The admin pastes a tunnel token from
// the Cloudflare dashboard; this package keeps the connector running.
package tunnel

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// States reported to the admin panel.
const (
	StateOff        = "off"
	StateStarting   = "starting"
	StateConnected  = "connected"
	StateRetrying   = "retrying"
	StateNotInstall = "not_installed"
)

type Status struct {
	Installed bool     `json:"installed"`
	State     string   `json:"state"`
	LastError string   `json:"last_error,omitempty"`
	Since     string   `json:"since,omitempty"`
	Log       []string `json:"log"`
}

// Manager supervises one cloudflared process.
type Manager struct {
	Binary string // path or name of cloudflared (default "cloudflared")
	// OnChange, if set, is called when the state changes (for notifications).
	OnChange func(state, lastErr string)

	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
	state   string
	lastErr string
	since   time.Time
	log     []string
}

func New() *Manager { return &Manager{Binary: "cloudflared", state: StateOff} }

// Installed reports whether the cloudflared binary is available.
func (m *Manager) Installed() bool {
	_, err := exec.LookPath(m.Binary)
	return err == nil
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := Status{Installed: m.Installed(), State: m.state, LastError: m.lastErr, Log: append([]string{}, m.log...)}
	if !st.Installed {
		st.State = StateNotInstall
	}
	if !m.since.IsZero() {
		st.Since = m.since.UTC().Format(time.RFC3339)
	}
	return st
}

func (m *Manager) set(state, errMsg string) {
	m.mu.Lock()
	changed := m.state != state
	m.state, m.since = state, time.Now()
	if errMsg != "" {
		m.lastErr = errMsg
	}
	lastErr, cb := m.lastErr, m.OnChange
	m.mu.Unlock()
	if changed && cb != nil {
		cb(state, lastErr)
	}
}

func (m *Manager) addLog(line string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.log = append(m.log, line)
	if len(m.log) > 40 {
		m.log = m.log[len(m.log)-40:]
	}
}

// Apply starts, restarts, or stops the connector to match the settings.
func (m *Manager) Apply(enabled bool, token string) error {
	m.Stop()
	if !enabled {
		return nil
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("paste the tunnel token from Cloudflare first")
	}
	if !m.Installed() {
		return errors.New("cloudflared is not installed in this container")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	m.mu.Lock()
	m.cancel, m.done, m.lastErr, m.log = cancel, done, "", nil
	m.mu.Unlock()
	m.set(StateStarting, "")
	go m.supervise(ctx, token, done)
	return nil
}

// Stop shuts the connector down and waits for it to exit.
func (m *Manager) Stop() {
	m.mu.Lock()
	cancel, done := m.cancel, m.done
	m.cancel, m.done = nil, nil
	m.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
	m.set(StateOff, "")
}

// supervise runs cloudflared and restarts it with backoff if it exits.
func (m *Manager) supervise(ctx context.Context, token string, done chan struct{}) {
	defer close(done)
	backoff := 5 * time.Second
	for ctx.Err() == nil {
		started := time.Now()
		err := m.runOnce(ctx, token)
		if ctx.Err() != nil {
			return
		}
		msg := "connector stopped"
		if err != nil {
			msg = err.Error()
		}
		m.set(StateRetrying, msg)
		if time.Since(started) > 2*time.Minute {
			backoff = 5 * time.Second // it was healthy for a while; retry quickly
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 5*time.Minute)
	}
}

func (m *Manager) runOnce(ctx context.Context, token string) error {
	// The token goes in the environment, not argv, so it never shows up in
	// process listings or logs.
	cmd := exec.CommandContext(ctx, m.Binary, "tunnel", "--no-autoupdate", "run")
	cmd.Env = append(os.Environ(), "TUNNEL_TOKEN="+token)
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	cmd.Stdout = cmd.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	m.watch(out)
	return cmd.Wait()
}

// watch reads cloudflared's log, keeps a tail for the admin panel, and
// detects when a connection to Cloudflare is established.
func (m *Manager) watch(r io.Reader) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		m.addLog(line)
		switch {
		case strings.Contains(line, "Registered tunnel connection"):
			m.set(StateConnected, "")
		case strings.Contains(line, "Unauthorized") || strings.Contains(line, "invalid token") ||
			strings.Contains(line, "Provided Tunnel token is not valid"):
			m.set(StateRetrying, "Cloudflare rejected the tunnel token; copy it again from the dashboard")
		}
	}
}
