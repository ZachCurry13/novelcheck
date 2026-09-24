package llm

import (
	"net"
	"net/url"
	"strings"
	"time"
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

// IsLocal reports whether baseURL points at a server on the home network
// (Ollama, LM Studio, vLLM…): a private or loopback IP, a single-word host
// name like "ollama", or one of Ollama's ports.
func IsLocal(baseURL string) bool {
	u, err := url.Parse(NormalizeBaseURL(baseURL))
	if err != nil || u.Host == "" {
		return false
	}
	if IsOllamaPort(u.Port()) {
		return true
	}
	host := u.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()
	}
	return !strings.Contains(host, ".") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".lan")
}

// Timeout is how long to wait for one answer: generous for models running on
// your own hardware (loading a model, or a CPU-only server, can take
// minutes), shorter for cloud APIs. override > 0 wins.
func Timeout(baseURL string, override int) time.Duration {
	switch {
	case override > 0:
		return time.Duration(min(override, 3600)) * time.Second
	case IsLocal(baseURL):
		return 10 * time.Minute
	default:
		return 2 * time.Minute
	}
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
