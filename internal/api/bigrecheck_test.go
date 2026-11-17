package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// A rating the small model wasn't sure of gets a second opinion from the bigger model by itself, once per rules
// version: the big model's answer replaces it; one that stays unsure keeps its Not sure chip and is not tried
// again; ratings made by hand, approved by a parent or made by a Deep Scan are never touched.
func TestNotSureRatingsGetASecondOpinionFromTheBigModel(t *testing.T) {
	asked := 0
	big := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		asked++
		confidence := "high"
		if strings.Contains(string(raw), "Stubborn Book") {
			confidence = "low"
		}
		verdict, _ := json.Marshal(map[string]any{"spice_level": 3, "spice_reason": "Heavy innuendo", "content_elements": map[string]bool{},
			"spiritual_elements": map[string]bool{}, "content": []string{}, "premise": "A book.", "confidence": confidence, "summary_verdict": "Fine."})
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": string(verdict)}}}})
	}))
	defer big.Close()
	var server *api.Server
	_, st := setupWith(t, func(s *api.Server) { server = s })
	_ = st.SetSetting(store.KeyAutoRate, "on")
	_ = st.SetSetting(store.KeyAutoRateHours, "") // any time
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	two := 2
	rate := func(title, model string) int64 {
		id, _ := st.UpsertBook(title, "Pat Writer", "", "A blurb long enough to count.")
		_ = st.AddCopy(cal, id, "/c/"+title+".epub", "epub", title)
		_ = st.SaveAnalysis(id, store.Analysis{SpiceLevel: &two, Confidence: "low", ContentSource: store.SourceAI, Model: model})
		return id
	}
	unsure, stubborn := rate("Unsure Book", "llama3.2"), rate("Stubborn Book", "llama3.2")
	byHand, approved := rate("Hand Rated", "manual: mum"), rate("Approved Book", "llama3.2")
	_, _ = st.DB.Exec(`UPDATE books SET approved = 1 WHERE id = ?`, approved)

	if n := server.RecheckNotSure(context.Background(), 10); n != 0 {
		t.Fatalf("no Deep Scan machine set up: %d", n)
	}
	_ = st.SetSetting(store.KeyDeepEnabled, "true")
	_ = st.SetSetting(store.KeyDeepBaseURL, big.URL+"/v1")
	_ = st.SetSetting(store.KeyDeepModels, "qwen2.5:14b")
	if n := server.RecheckNotSure(context.Background(), 10); n != 2 {
		t.Fatalf("the two unsure books get a second opinion: %d", n)
	}
	if b, _ := st.BookByID(unsure, nil); b.Confidence != "high" || b.SpiceLevel == nil || *b.SpiceLevel != 3 || b.AnalysisModel == "llama3.2" {
		t.Fatalf("the big model's answer replaced it: %+v", b)
	}
	if b, _ := st.BookByID(stubborn, nil); b.Confidence != "low" {
		t.Fatalf("still unsure after the second opinion: %+v", b)
	}
	for _, id := range []int64{byHand, approved} {
		if b, _ := st.BookByID(id, nil); b.Confidence != "low" || *b.SpiceLevel != 2 {
			t.Fatalf("by hand or approved is left alone: %+v", b)
		}
	}
	before := asked
	if n := server.RecheckNotSure(context.Background(), 10); n != 0 || asked != before {
		t.Fatalf("one second opinion per rules version: %d, %d more questions", n, asked-before)
	}
}
