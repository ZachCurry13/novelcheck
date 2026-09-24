package analyzer

import "testing"

func TestBatchSummaryWording(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 18400: "18,400", 1234567: "1,234,567", -2500: "-2,500"} {
		if got := thousands(n); got != want {
			t.Errorf("thousands(%d) = %q, want %q", n, got, want)
		}
	}
	if got := batchSummary(1, 0, 900, 0); got != "Rating finished: 1 book rated · 900 tokens" {
		t.Errorf("free local AI: %q", got)
	}
	if got := batchSummary(18, 2, 18400, 0.0123); got != "Rating finished: 18 books rated, 2 failed · 18,400 tokens · $0.0123" {
		t.Errorf("summary: %q", got)
	}
}
