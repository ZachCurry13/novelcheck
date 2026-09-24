package llm

import (
	"testing"
	"time"
)

func TestNormalizeBaseURL(t *testing.T) {
	cases := map[string]string{
		"":                           "",
		"10.13.6.41:30068":           "http://10.13.6.41:30068/v1",
		"10.13.6.41:30068/":          "http://10.13.6.41:30068/v1",
		"http://ollama:11434/v1":     "http://ollama:11434/v1",
		"https://api.openai.com/v1/": "https://api.openai.com/v1",
		"https://api.openai.com/v1/chat/completions": "https://api.openai.com/v1",
		" http://10.0.0.2:8000/v1 ":                  "http://10.0.0.2:8000/v1",
	}
	for in, want := range cases {
		if got := NormalizeBaseURL(in); got != want {
			t.Errorf("NormalizeBaseURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTimeout(t *testing.T) {
	cases := []struct {
		url      string
		override int
		want     time.Duration
	}{
		{"10.13.6.41:30068", 0, 10 * time.Minute},
		{"http://ollama:11434/v1", 0, 10 * time.Minute},
		{"http://192.168.1.20:8000/v1", 0, 10 * time.Minute},
		{"https://api.openai.com/v1", 0, 2 * time.Minute},
		{"https://api.openai.com/v1", 300, 5 * time.Minute},
		{"http://10.0.0.5:30068", 99999, time.Hour},
	}
	for _, c := range cases {
		if got := Timeout(c.url, c.override); got != c.want {
			t.Errorf("Timeout(%q, %d) = %s, want %s", c.url, c.override, got, c.want)
		}
	}
}
