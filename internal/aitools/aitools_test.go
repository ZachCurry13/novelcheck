package aitools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestSpeedTestAndPowerCost(t *testing.T) {
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": `{"spice_level": 0, "level": 0}`}}},
			"usage":   map[string]int{"prompt_tokens": 1000, "completion_tokens": 100},
		})
	}))
	defer ai.Close()
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	_ = st.SetSetting(store.KeyLLMBaseURL, ai.URL)
	_ = st.SetSetting(store.KeyLLMModel, "small-model")
	_ = st.SetSetting(store.KeyPriceInputPerM, "0")
	_ = st.SetSetting(store.KeyPriceOutputPerM, "0")
	_ = st.SetSetting(store.KeyMainWatts, "100")
	_ = st.SetSetting(store.KeyPowerPrice, "0.20")

	s := New(st)
	if err := s.StartBench("deep"); err != nil { // Deep Scans on the main machine
		t.Fatal(err)
	}
	for deadline := time.Now().Add(5 * time.Second); s.Status().Benching; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("speed test didn't finish")
		}
	}
	b, ok := st.BenchFor("small-model")
	if !ok || b.RatingSeconds <= 0 || b.PartSeconds <= 0 || b.PartWords < 500 || b.TokensPerSecond <= 0 {
		t.Fatalf("bench: %+v %v", b, ok)
	}
	// Electricity: 100 W for the calls' time at $0.20/kWh, not "free".
	if spent := st.SpentUSD(); spent <= 0 || spent > 0.001 {
		t.Fatalf("power cost %v", spent)
	}
	if err := s.StartBench("backup"); err == nil {
		t.Fatal("no backup AI is set up")
	}
}
