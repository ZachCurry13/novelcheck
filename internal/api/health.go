package api

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/calibre"
	"github.com/zachcurry13/novelcheck/internal/delivery"
	"github.com/zachcurry13/novelcheck/internal/enrich"
	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/ollama"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// CheckResult is one row of the System page's "Check everything".
type CheckResult struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"` // ok | warn | error | skip
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`  // what to do about it
	Link    string `json:"link,omitempty"` // in-app page for the fix
	Millis  int64  `json:"ms"`
}

type check struct {
	id, name, link string
	run            func(ctx context.Context) (status, msg, fix string)
}

// runChecks runs every connection check in parallel (10s each) and records
// failures as notifications (passing checks resolve earlier ones).
func (s *Server) runChecks(ctx context.Context) []CheckResult {
	checks := s.checks()
	out := make([]CheckResult, len(checks))
	var wg sync.WaitGroup
	for i, c := range checks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			limit := 10 * time.Second
			if strings.HasPrefix(c.id, "llm") && llm.IsLocal(s.Store.Setting(store.KeyLLMBaseURL)) {
				limit = 90 * time.Second // a local model may need to load first
			} else if strings.HasPrefix(c.id, "llm") {
				limit = 30 * time.Second
			}
			cctx, cancel := context.WithTimeout(ctx, limit)
			defer cancel()
			start := time.Now()
			status, msg, fix := c.run(cctx)
			out[i] = CheckResult{ID: c.id, Name: c.name, Status: status, Message: msg, Fix: fix, Link: c.link,
				Millis: time.Since(start).Milliseconds()}
		}()
	}
	wg.Wait()
	for _, r := range out {
		switch r.Status {
		case "error", "warn":
			level := map[string]string{"error": "error", "warn": "warning"}[r.Status]
			s.Store.Notify(level, "check:"+r.ID, r.Name+": "+r.Message, r.Link)
		case "ok":
			s.Store.Resolve("check:" + r.ID)
		}
	}
	return out
}

func errResult(err error, fix string) (string, string, string) { return "error", err.Error(), fix }

func (s *Server) checks() []check {
	set := s.Store.Setting
	cs := []check{
		{"llm", "AI provider (" + set(store.KeyLLMModel) + ")", "#/admin", func(ctx context.Context) (string, string, string) {
			return s.checkModel(ctx, set(store.KeyLLMModel))
		}},
	}
	for i, fb := range s.Store.LLMModels() {
		if i == 0 {
			continue // the main model is checked above
		}
		id := "llm-fallback"
		if i > 1 {
			id = fmt.Sprintf("llm-fallback-%d", i)
		}
		cs = append(cs, check{id, fmt.Sprintf("AI fallback #%d (%s)", i, fb), "#/admin", func(ctx context.Context) (string, string, string) {
			return s.checkModel(ctx, fb)
		}})
	}
	cs = append(cs,
		check{"openlibrary", "Open Library (book blurbs)", "", func(ctx context.Context) (string, string, string) {
			if err := enrich.New("").CheckOpenLibrary(ctx); err != nil {
				return errResult(err, "Open Library may be down; NovelCheck falls back to Google Books.")
			}
			return "ok", "Working", ""
		}},
		check{"googlebooks", "Google Books (book blurbs)", "#/admin", func(ctx context.Context) (string, string, string) {
			key := set(store.KeyGoogleBooksAPIKey)
			if err := enrich.New(key).CheckGoogleBooks(ctx); err != nil {
				if key == "" {
					// Without a key everyone shares Google's anonymous quota.
					return "warn", err.Error(), "Add a free Google Books API key: console.cloud.google.com → APIs & Services → enable \"Books API\" → Credentials → Create API key. Paste it in Admin → LLM Analysis Engine → Google Books API key. Until then NovelCheck uses Open Library and Calibre's own descriptions."
				}
				return errResult(err, "Check the Google Books API key in Admin → LLM Analysis Engine (is the Books API enabled for it?), or clear it to use the shared free quota.")
			}
			if key == "" {
				return "ok", "Working without an API key (shared daily limit)", "Optional: add a Google Books API key in Admin to avoid hitting the shared limit on big libraries."
			}
			return "ok", "Working with your API key", ""
		}},
		check{"calibre", "Calibre library", "#/admin", func(ctx context.Context) (string, string, string) {
			if !s.Syncer.Available() {
				return "error", "No Calibre library selected or found", "Pick your library in Admin → Calibre Library."
			}
			n, err := calibre.CountBooks(s.Syncer.LibraryDir())
			if err != nil {
				return errResult(err, "Give the apps user read access to the Calibre folder in TrueNAS.")
			}
			return "ok", fmt.Sprintf("Readable: %d books", n), ""
		}},
		check{"disk", "Data disk space", "", func(ctx context.Context) (string, string, string) {
			snap := s.SysInfo.Snapshot()
			free := float64(snap.DiskFree) / (1 << 30)
			if snap.DiskTotal == 0 {
				return "skip", "Unknown", ""
			}
			if free < 1 {
				return "warn", fmt.Sprintf("Only %.1f GB free", free), "Free up space on the dataset holding /data."
			}
			return "ok", fmt.Sprintf("%.1f GB free", free), ""
		}},
	)
	if set(store.KeyCalibreSrvURL) != "" {
		cs = append(cs, check{"calibre-server", "Calibre Content server", "#/admin", func(ctx context.Context) (string, string, string) {
			if _, _, err := s.calibreClient().Libraries(ctx); err != nil {
				return errResult(err, "Make sure Calibre is running with its Content server on, then re-check the login in Admin → Calibre Library.")
			}
			return "ok", "Connected", ""
		}})
	}
	if set(store.KeySMTPHost) != "" {
		cs = append(cs, check{"smtp", "Email (Send-to-Kindle)", "#/admin", func(ctx context.Context) (string, string, string) {
			if err := delivery.CheckLogin(s.smtpConfig()); err != nil {
				return errResult(err, "Check the SMTP settings in Admin. Gmail needs an App Password, not your normal password.")
			}
			return "ok", "Signed in (no email sent)", ""
		}})
	}
	if s.Store.SettingBool(store.KeyTunnelEnabled) && s.Tunnel != nil {
		cs = append(cs, check{"tunnel", "Remote access (Cloudflare Tunnel)", "#/admin", func(ctx context.Context) (string, string, string) {
			st := s.Tunnel.Status()
			if st.State == "connected" {
				return "ok", "Connected", ""
			}
			return "error", "Not connected (" + st.State + ")" + strings.TrimSpace(" "+st.LastError), "Open Admin → Remote access and check the token and the connector log."
		}})
	}
	if base := ollamaBase(set(store.KeyLLMBaseURL)); base != "" {
		cs = append(cs, check{"ollama", "Ollama server", "#/system", func(ctx context.Context) (string, string, string) {
			srv, err := ollama.Probe(ctx, base)
			if err != nil {
				return errResult(err, "Check the Ollama app is Running in TrueNAS.")
			}
			return "ok", fmt.Sprintf("Version %s, %d models downloaded", srv.Version, len(srv.Models)), ""
		}})
	}
	if s.Store.SettingBool(store.KeyCheckUpdates) && s.Updates != nil {
		cs = append(cs, check{"updates", "Update check (GitHub)", "", func(ctx context.Context) (string, string, string) {
			st := s.Updates.Status(ctx, true)
			if st.Error != "" {
				return "warn", st.Error, "NovelCheck will try again later."
			}
			if st.UpdateAvailable {
				return "warn", "Version " + st.Latest + " is available", "Update in TrueNAS: Apps → novelcheck → Update."
			}
			return "ok", "Up to date (" + st.Current + ")", ""
		}})
	}
	return cs
}

// checkModel sends a tiny request (a few tokens) to confirm the model answers.
func (s *Server) checkModel(ctx context.Context, model string) (string, string, string) {
	if model == "" {
		return "error", "No model set", "Pick an AI provider in Admin → LLM Analysis Engine."
	}
	out, usage, err := s.llmClient().Complete(ctx, model, "You are a health check. Reply with the single word OK.", "ping")
	if errors.Is(err, context.DeadlineExceeded) {
		if llm.IsLocal(s.Store.Setting(store.KeyLLMBaseURL)) {
			return "error", "No answer in time", "Ollama may still be loading the model: wait a minute and check again. If it keeps happening, the model is probably running on the CPU (see Usage → Ollama) or is too big for your GPU; try a smaller one."
		}
		return "error", "No answer in time", "The AI service is slow or unreachable right now. Try again shortly."
	}
	if err != nil {
		return errResult(err, "Check the API key, model name and credit with your AI provider (Admin → LLM Analysis Engine → Show setup steps).")
	}
	if strings.TrimSpace(out) == "" {
		return "warn", "Answered, but with an empty reply", "Try a different model."
	}
	return "ok", fmt.Sprintf("Answering (%d tokens used for this check)", usage.Total()), ""
}

// llmClient builds the configured provider client (same logic as the worker).
func (s *Server) llmClient() llm.Completer {
	// JSON mode off: the health check asks for a plain "OK", not JSON.
	return llm.New(s.Store.Setting(store.KeyLLMProvider), s.Store.Setting(store.KeyLLMBaseURL),
		s.Store.Setting(store.KeyLLMAPIKey), false)
}

func (s *Server) smtpConfig() delivery.SMTPConfig {
	return delivery.SMTPConfig{
		Host: s.Store.Setting(store.KeySMTPHost), Port: s.Store.Setting(store.KeySMTPPort),
		Username: s.Store.Setting(store.KeySMTPUser), Password: s.Store.Setting(store.KeySMTPPassword),
		From: s.Store.Setting(store.KeySMTPFrom),
	}
}

// ollamaBase returns the Ollama server address when the LLM settings point
// at one (default Ollama/TrueNAS ports), else "".
func ollamaBase(llmURL string) string {
	base, err := ollama.Normalize(llmURL)
	if err != nil {
		return ""
	}
	if u, perr := url.Parse(base); perr != nil || !llm.IsOllamaPort(u.Port()) {
		return ""
	}
	return base
}
