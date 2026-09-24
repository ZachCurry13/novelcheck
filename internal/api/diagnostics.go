package api

import (
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
	"github.com/zachcurry13/novelcheck/internal/version"
)

// handleDiagnostics returns a plain-text report for troubleshooting: version,
// settings (secrets never included), worker, recent problems and the
// remote-access log. Email addresses are partly hidden so it's safe to paste
// into a GitHub issue.
func (s *Server) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(s.diagnosticsText()))
}

func (s *Server) diagnosticsText() string {
	var b strings.Builder
	line := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }
	line("NovelCheck diagnostics · %s", time.Now().UTC().Format(time.RFC3339))
	line("Version %s · %s/%s · Go %s", version.Version, runtime.GOOS, runtime.GOARCH, runtime.Version())

	line("\n== Settings ==")
	all, _ := s.Store.AllSettings()
	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := all[k]
		switch {
		case store.SecretKeys[k]:
			v = map[bool]string{true: "(set)", false: "(not set)"}[v != ""]
		case strings.Contains(v, "@"):
			v = maskEmail(v)
		}
		line("%s = %s", k, v)
	}
	line("AI models in order: %s", strings.Join(s.Store.LLMModels(), " → "))

	line("\n== Library ==")
	if counts, err := s.Store.StatusCounts(); err == nil {
		line("pending %d · queued %d · processing %d · analyzed %d · errors %d",
			counts["pending"], counts["queued"], counts["processing"], counts["analyzed"], counts["error"])
	}
	ws := s.Worker.Status()
	line("worker: %s · %d waiting · last error: %s", ws.State, ws.QueueLength, orNone(ws.LastError))
	line("calibre library found: %v (%s)", s.Syncer.Available(), s.Syncer.LibraryDir())
	line("last calibre sync: %s", orNone(s.Store.Setting(store.KeyCalibreLastResult)))

	if s.SysInfo != nil {
		n := s.SysInfo.Snapshot()
		line("\n== Resources ==")
		line("cpu %.1f%% of %d cores · memory %d MB of %d MB · disk free %d GB · db %.1f MB · up %s",
			n.CPUPercent, n.CPUCores, n.MemUsed>>20, n.MemLimit>>20, n.DiskFree>>30, float64(n.DBSize)/(1<<20),
			(time.Duration(n.UptimeSeconds) * time.Second).String())
	}

	line("\n== Recent problems ==")
	items, _, _ := s.Store.Notifications(30)
	if len(items) == 0 {
		line("(none)")
	}
	for _, n := range items {
		line("[%s] %s %s: %s (x%d)", n.UpdatedAt, n.Level, n.Source, n.Message, n.Count)
	}

	if s.Tunnel != nil {
		st := s.Tunnel.Status()
		line("\n== Remote access (cloudflared) ==")
		line("installed %v · state %s · since %s · last problem: %s", st.Installed, st.State, orNone(st.Since), orNone(st.LastError))
		for _, l := range st.Log {
			line("%s", l)
		}
	}
	return b.String()
}

func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(none)"
	}
	return s
}

// maskEmail turns "reader@gmail.com" into "r***@gmail.com".
func maskEmail(v string) string {
	at := strings.LastIndex(v, "@")
	if at <= 0 {
		return v
	}
	return v[:1] + "***" + v[at:]
}
