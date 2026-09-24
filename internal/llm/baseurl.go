package llm

import (
	"net/url"
	"strings"
)

// OllamaPorts are the ports Ollama usually listens on (its default, and the
// TrueNAS app's default).
var OllamaPorts = []string{"11434", "30068"}

// NormalizeBaseURL tidies a hand-typed API address: it adds http:// when the
// scheme is missing (e.g. "10.0.0.5:30068"), drops a pasted
// "/chat/completions" suffix, and adds Ollama's "/v1" when the address is a
// bare Ollama host.
func NormalizeBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	raw = strings.TrimRight(raw, "/")
	raw = strings.TrimSuffix(raw, "/chat/completions")
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	if u.Path == "" && IsOllamaPort(u.Port()) {
		u.Path = "/v1"
	}
	return u.String()
}

// IsOllamaPort reports whether port is one of Ollama's usual ports.
func IsOllamaPort(port string) bool {
	for _, p := range OllamaPorts {
		if port == p {
			return true
		}
	}
	return false
}
