package store_test

import (
	"math"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestElectricityIsCountedForPastCalls(t *testing.T) {
	st := newStore(t)
	_ = st.SetSetting(store.KeyLLMModel, "llama3.2:latest")
	_ = st.SetSetting(store.KeyPriceInputPerM, "0")
	_ = st.SetSetting(store.KeyPriceOutputPerM, "0")
	_ = st.SetSetting(store.KeyBackupEnabled, "true")
	_ = st.SetSetting(store.KeyBackupBaseURL, "https://cloud.example/v1")
	_ = st.SetSetting(store.KeyBackupModel, "gemini-flash")
	_ = st.SetSetting(store.KeyBackupPriceIn, "0.30")
	_ = st.SetSetting(store.KeyBackupPriceOut, "2.50")
	_ = st.SetSetting(store.KeyPowerPrice, "0.15")
	// Calls made while the watts were still 0: an hour of work on the local
	// AI, a call from before times were kept (same tokens, so the same hour),
	// an old local model, and a paid cloud call.
	for _, row := range []struct {
		model    string
		p, c     int
		cost, at float64
	}{{"llama3.2:latest", 1000, 1000, 0, 3600}, {"llama3.2:latest", 1000, 1000, 0, 0}, {"qwen2.5:3b", 500, 500, 0, 1800}, {"gemini-flash", 1000, 1000, 0.0028, 2}} {
		if _, err := st.DB.Exec(`INSERT INTO token_usage (model, prompt_tokens, completion_tokens, cost, seconds) VALUES (?, ?, ?, ?, ?)`,
			row.model, row.p, row.c, row.cost, row.at); err != nil {
			t.Fatal(err)
		}
	}
	_ = st.SetSetting(store.KeyMainWatts, "100")
	if n, err := st.RecountElectricity(); err != nil || n != 3 {
		t.Fatalf("recounted %d: %v", n, err)
	}
	// 2.5 hours at 100 W and $0.15 per kWh = $0.0375; the cloud call is unchanged.
	if got := st.ElectricityUSD(); math.Abs(got-0.0375) > 1e-9 {
		t.Fatalf("electricity %v", got)
	}
	if got := st.SpentUSD(); math.Abs(got-0.0375-0.0028) > 1e-9 {
		t.Fatalf("spent %v", got)
	}
	_ = st.SetSetting(store.KeyMainWatts, "0") // back to free
	_, _ = st.RecountElectricity()
	if got := st.ElectricityUSD(); got != 0 {
		t.Fatalf("electricity after 0 W: %v", got)
	}
}
