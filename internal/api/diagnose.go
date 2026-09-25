package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
	"github.com/zachcurry13/novelcheck/internal/version"
)

const diagnosePrompt = `You are the built-in support assistant of NovelCheck, a self-hosted app (usually on TrueNAS, in Docker) that reads a Calibre e-book library, rates books for romance/sexual content ("peppers") and other themes with an AI model (OpenAI, Claude, Gemini, Perplexity or a local Ollama server), emails books to Kindles over SMTP (Gmail needs an App Password), and can be reached remotely through a built-in Cloudflare Tunnel (cloudflared).

You get a diagnostics report (settings without secrets, recent problems, worker state, resources and the cloudflared log) and, optionally, the user's description of the problem.

Explain for a non-technical parent. Separate real problems from harmless noise (for example, cloudflared's "failed to sufficiently increase receive buffer size" warning, or QUIC failing on one region while it falls back to http2, are harmless if connections were registered). Never invent settings or log lines that aren't in the report.

Reply with JSON only:
{
  "diagnosis": "plain-English explanation of what's wrong (or that everything looks fine), 2-6 sentences",
  "fix_steps": ["short step the user can do in NovelCheck or TrueNAS", "..."],
  "title": "short bug report title, under 80 characters",
  "summary": "2-4 sentences for the developer: what fails, where, and the key evidence",
  "suspected_cause": "the most likely technical cause, or 'unknown'",
  "is_bug": true or false (false when it's a setup issue the user can fix)
}`

type diagnosis struct {
	Diagnosis      string   `json:"diagnosis"`
	FixSteps       []string `json:"fix_steps"`
	Title          string   `json:"title"`
	Summary        string   `json:"summary"`
	SuspectedCause string   `json:"suspected_cause"`
	IsBug          bool     `json:"is_bug"`
	AIError        string   `json:"ai_error,omitempty"`
	Model          string   `json:"model,omitempty"`
	Report         string   `json:"report"` // markdown for the GitHub issue
}

// handleDiagnose asks the configured AI to explain a problem from the
// diagnostics report and drafts a GitHub bug report. The bug report is made
// even when the AI itself is what's broken.
func (s *Server) handleDiagnose(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Problem string `json:"problem"`
	}
	if r.ContentLength > 0 && !readJSON(w, r, &body, 16<<10) {
		return
	}
	problem := strings.TrimSpace(body.Problem)
	diag := s.diagnosticsText()
	d := diagnosis{}

	// Ask the main AI; if it can't answer (it may be the problem), the backup.
	user := "User's description: " + orNone(problem) + "\n\nDiagnostics report:\n" + lastChars(diag, 14000)
	var aiErrs []string
	for _, ai := range s.Store.AIConfigs() {
		if len(ai.Models) == 0 {
			aiErrs = append(aiErrs, "no AI model is set up (Admin → AI & Scans → LLM Analysis Engine)")
			continue
		}
		model := ai.Models[0]
		ctx, cancel := context.WithTimeout(r.Context(), min(llm.Timeout(ai.BaseURL, s.Store.SettingInt(store.KeyLLMTimeoutSeconds)), 5*time.Minute))
		prompt := diagnosePrompt + "\n\nWrite \"diagnosis\" and \"fix_steps\" in " + llm.LanguageRule(s.Store.Setting(store.KeyLanguage)) +
			"; write \"title\", \"summary\" and \"suspected_cause\" in English (they go to the developer)."
		out, usage, err := llm.New(ai.Provider, ai.BaseURL, ai.APIKey, ai.JSONMode).Complete(ctx, model, prompt, user)
		cancel()
		if usage.Total() > 0 {
			_ = s.Store.RecordUsageCost(0, model, usage.PromptTokens, usage.CompletionTokens, ai)
		}
		if err != nil {
			aiErrs = append(aiErrs, fmt.Sprintf("%s AI (%s): %v", ai.Name, model, err))
			continue
		}
		d.Model = model
		if ai.Name == "backup" {
			d.Model += " (backup AI)"
		}
		if !parseDiagnosis(out, &d) {
			d.Diagnosis = strings.TrimSpace(out) // the model ignored the format; still show what it said
		}
		aiErrs = nil
		break
	}
	d.AIError = strings.Join(aiErrs, " · ")
	if d.Title == "" {
		d.Title = firstLineOf(problem, "Problem report")
	}
	d.Report = bugReport(d, problem, diag)
	// Keep the public web address out of a public GitHub issue.
	if host := s.Store.Setting(store.KeyTunnelHostname); strings.TrimSpace(host) != "" {
		d.Report = strings.ReplaceAll(d.Report, strings.TrimSpace(host), "your-address.example.com")
	}
	writeJSON(w, http.StatusOK, d)
}

func parseDiagnosis(out string, d *diagnosis) bool {
	start, end := strings.Index(out, "{"), strings.LastIndex(out, "}")
	if start < 0 || end <= start {
		return false
	}
	var tmp diagnosis
	if json.Unmarshal([]byte(out[start:end+1]), &tmp) != nil || strings.TrimSpace(tmp.Diagnosis) == "" {
		return false
	}
	d.Diagnosis, d.FixSteps, d.Title = strings.TrimSpace(tmp.Diagnosis), tmp.FixSteps, strings.TrimSpace(tmp.Title)
	d.Summary, d.SuspectedCause, d.IsBug = strings.TrimSpace(tmp.Summary), strings.TrimSpace(tmp.SuspectedCause), tmp.IsBug
	return true
}

// bugReport is the GitHub issue body: the AI's reading plus the real data.
func bugReport(d diagnosis, problem, diag string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**NovelCheck version:** %s\n\n", version.Version)
	fmt.Fprintf(&b, "### What happened\n%s\n\n", orNone(problem))
	if d.Summary != "" {
		fmt.Fprintf(&b, "### Summary (written by NovelCheck's AI, %s)\n%s\n\n", d.Model, d.Summary)
	}
	if d.Diagnosis != "" {
		fmt.Fprintf(&b, "### AI diagnosis\n%s\n\n", d.Diagnosis)
	}
	if d.SuspectedCause != "" {
		fmt.Fprintf(&b, "**Suspected cause:** %s\n\n", d.SuspectedCause)
	}
	if len(d.FixSteps) > 0 {
		b.WriteString("**Suggested fix steps:**\n")
		for i, st := range d.FixSteps {
			fmt.Fprintf(&b, "%d. %s\n", i+1, st)
		}
		b.WriteString("\n")
	}
	if d.AIError != "" {
		fmt.Fprintf(&b, "_AI diagnosis unavailable: %s_\n\n", d.AIError)
	}
	fmt.Fprintf(&b, "<details><summary>Diagnostics (no passwords or keys)</summary>\n\n```text\n%s\n```\n</details>\n", lastChars(diag, 50000))
	return b.String()
}

// lastChars keeps the end of s (logs are newest-last).
func lastChars(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…(earlier lines trimmed)…\n" + s[len(s)-n:]
}

func firstLineOf(s, fallback string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return fallback
	}
	if len(s) > 80 {
		s = s[:77] + "…"
	}
	return s
}
