package store

import "testing"

// A rating from older rules is "old"; a parent's or a Deep Scan's never is, nor an unrated book.
func TestOldRating(t *testing.T) {
	for _, c := range []struct {
		b    Book
		want bool
	}{
		{Book{Status: "analyzed", RulesVersion: RulesVersion - 1, AnalysisModel: "gemini"}, true},
		{Book{Status: "analyzed", RulesVersion: RulesVersion, AnalysisModel: "gemini"}, false},
		{Book{Status: "analyzed", RulesVersion: 0, AnalysisModel: "manual: zach"}, false},
		{Book{Status: "analyzed", RulesVersion: 0, AnalysisModel: DeepModelPrefix + "llama"}, false},
		{Book{Status: "pending", RulesVersion: 0}, false},
	} {
		if got := OldRating(&c.b); got != c.want {
			t.Fatalf("%+v: got %v", c.b, got)
		}
	}
}
