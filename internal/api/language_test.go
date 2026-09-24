package api_test

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestLanguageSettingAndRerate(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	es, _ := st.UpsertBook("Cien años", "G", "", "")
	_ = st.SaveAnalysis(es, store.Analysis{Classification: "No Spice", Model: "llama3.2",
		SummaryVerdict: "Una historia de amor dulce con besos, adecuada para adolescentes."})
	en, _ := st.UpsertBook("English One", "E", "", "")
	_ = st.SaveAnalysis(en, store.Analysis{Classification: "No Spice", Model: "llama3.2",
		SummaryVerdict: "A sweet, clean romance with a few kisses; fine for teens."})
	hand, _ := st.UpsertBook("Hand Rated", "H", "", "")
	_ = st.SaveAnalysis(hand, store.Analysis{Classification: "No Spice", Model: "manual: mom",
		SummaryVerdict: "Una novela limpia y bonita para toda la familia entera."})

	if st.Setting(store.KeyLanguage) != "English (US)" {
		t.Fatalf("default language: %q", st.Setting(store.KeyLanguage))
	}
	_, s := admin.do("GET", "/api/admin/status", nil, false)
	if s["non_english"].(float64) != 1 {
		t.Fatalf("non-English AI summaries: %v", s["non_english"])
	}
	if res, out := admin.do("POST", "/api/admin/rerate", map[string]string{"which": "language"}, true); res.StatusCode != 200 || out["queued"].(float64) != 1 {
		t.Fatalf("rerate language: %d %v", res.StatusCode, out)
	}
	if res, _ := admin.do("PUT", "/api/admin/settings", map[string]string{"language": "Klingon"}, true); res.StatusCode != 400 {
		t.Fatalf("unknown language must be refused: %d", res.StatusCode)
	}
	if res, _ := admin.do("PUT", "/api/admin/settings", map[string]string{"language": "Spanish"}, true); res.StatusCode != 200 {
		t.Fatalf("set Spanish: %d", res.StatusCode)
	}
	if _, s = admin.do("GET", "/api/admin/status", nil, false); s["non_english"].(float64) != 0 {
		t.Fatal("the English check only applies when the language is English")
	}
}
