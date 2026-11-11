package llm

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestQuotaRest(t *testing.T) {
	daily := errors.New(`LLM error (429 Too Many Requests): You exceeded your current quota. * Quota exceeded for metric: generate_content_free_tier_requests, limit: 20, model: gemini-3.8-flash, description: Generate content requests per day limit exceeded on Free Tier. Please retry in 4h28m24.969921172s.`)
	if d := quotaRest(daily); d < 4*time.Hour || d > 5*time.Hour {
		t.Fatalf("the service's own wait is used: %v", d)
	}
	if d := quotaRest(errors.New("LLM error (429): quota exceeded, please retry in 12.5s")); d != 0 {
		t.Fatalf("a short wait is ordinary busy: %v", d)
	}
	if d := quotaRest(errors.New("LLM error (503): This model is currently experiencing high demand")); d != 0 {
		t.Fatalf("busy is not a used-up quota: %v", d)
	}
	if d := quotaRest(errors.New("requests per day limit exceeded")); d != time.Hour {
		t.Fatalf("a daily limit with no time given rests an hour: %v", d)
	}

	err := noteQuota("https://x/v1", "gemini-test", daily)
	if !strings.Contains(err.Error(), "used up its free requests") || strings.Contains(err.Error(), "aistudio") {
		t.Fatalf("one short sentence, not the service's whole message: %v", err)
	}
	if rested("https://x/v1", "gemini-test") == nil {
		t.Fatal("left alone until then")
	}
	if rested("https://x/v1", "another-model") != nil || rested("https://y/v1", "gemini-test") != nil {
		t.Fatal("only that model on that service")
	}
	if _, ok := Resting()["gemini-test"]; !ok {
		t.Fatal("listed as resting")
	}
	resting.Delete("https://x/v1|gemini-test")
}
