package tunnel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeCloudflared writes a script that behaves like cloudflared: it checks
// the token arrives via TUNNEL_TOKEN (never argv), logs a connection, and
// runs until stopped.
func fakeCloudflared(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "cloudflared")
	script := `#!/bin/sh
case "$*" in *eyJ*) echo "token leaked into argv" >&2; exit 3;; esac
if [ "$TUNNEL_TOKEN" != "eyJgood" ]; then echo "Provided Tunnel token is not valid" >&2; exit 1; fi
echo "INF Registered tunnel connection connIndex=0" >&2
exec sleep 30
`
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func waitFor(t *testing.T, m *Manager, state string) Status {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if st := m.Status(); st.State == state {
			return st
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("state never became %q: %+v", state, m.Status())
	return Status{}
}

func TestConnectAndStop(t *testing.T) {
	m := &Manager{Binary: fakeCloudflared(t), state: StateOff}
	if err := m.Apply(true, "eyJgood"); err != nil {
		t.Fatal(err)
	}
	st := waitFor(t, m, StateConnected)
	if len(st.Log) == 0 || !strings.Contains(st.Log[0], "Registered") {
		t.Fatalf("log not captured: %+v", st.Log)
	}
	start := time.Now()
	if err := m.Apply(false, ""); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("stop took too long")
	}
	if m.Status().State != StateOff {
		t.Fatalf("expected off, got %+v", m.Status())
	}
}

func TestBadTokenRetries(t *testing.T) {
	m := &Manager{Binary: fakeCloudflared(t), state: StateOff}
	if err := m.Apply(true, "eyJbad"); err != nil {
		t.Fatal(err)
	}
	st := waitFor(t, m, StateRetrying)
	if !strings.Contains(st.LastError, "rejected the tunnel token") && !strings.Contains(st.LastError, "exit status") {
		t.Fatalf("unexpected error %q", st.LastError)
	}
	m.Stop()
}

func TestNotInstalledAndEmptyToken(t *testing.T) {
	m := &Manager{Binary: "definitely-not-cloudflared", state: StateOff}
	if st := m.Status(); st.Installed || st.State != StateNotInstall {
		t.Fatalf("unexpected %+v", st)
	}
	if err := m.Apply(true, "eyJgood"); err == nil {
		t.Fatal("expected not-installed error")
	}
	if err := m.Apply(true, " "); err == nil {
		t.Fatal("expected empty-token error")
	}
}
