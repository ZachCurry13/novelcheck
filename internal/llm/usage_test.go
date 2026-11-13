package llm

import (
	"encoding/json"
	"testing"
)

func TestThinkingTokensCountAsOutput(t *testing.T) {
	for in, want := range map[string]Usage{
		`{"prompt_tokens":1000,"completion_tokens":50,"total_tokens":1850}`: {1000, 850}, // Gemini: 800 thinking tokens
		`{"prompt_tokens":1000,"completion_tokens":50,"total_tokens":1050}`: {1000, 50},  // adds up
		`{"prompt_tokens":1000,"completion_tokens":50}`:                     {1000, 50},  // no total given
	} {
		var u Usage
		if err := json.Unmarshal([]byte(in), &u); err != nil || u != want {
			t.Fatalf("%s: %+v %v", in, u, err)
		}
	}
}
