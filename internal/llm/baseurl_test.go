package llm

import "testing"

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
