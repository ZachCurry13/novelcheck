package store

// AI hardware settings (v1.26): an optional separate machine for Deep Scans
// (a bigger model; scans wait for it while it's off), each machine's power
// draw and the electricity price (so a local AI's cost isn't "free"), and
// the speed test's results.

import (
	"encoding/json"
	"strings"
	"time"
)

const (
	KeyDeepEnabled  = "deep_llm_enabled"  // Deep Scans run on their own machine
	KeyDeepProvider = "deep_llm_provider" // openai-compatible or anthropic
	KeyDeepBaseURL  = "deep_llm_base_url"
	KeyDeepAPIKey   = "deep_llm_api_key"
	KeyDeepModels   = "deep_llm_model" // one or more, comma-separated, in order
	KeyDeepJSONMode = "deep_llm_json_mode"

	KeyPowerPrice  = "power_price_kwh" // electricity, per kWh (0 = don't count)
	KeyMainWatts   = "main_ai_watts"   // extra watts while the machine works (local AIs)
	KeyBackupWatts = "backup_ai_watts"
	KeyDeepWatts   = "deep_ai_watts"

	KeyBenchmarks = "ai_benchmarks" // JSON: the speed test's results per model
)

func init() {
	for k, v := range map[string]string{KeyDeepEnabled: "false", KeyDeepProvider: "openai", KeyDeepBaseURL: "", KeyDeepAPIKey: "", KeyDeepModels: "",
		KeyDeepJSONMode: "true", KeyPowerPrice: "0", KeyMainWatts: "0", KeyBackupWatts: "0", KeyDeepWatts: "0"} {
		Defaults[k] = v
	}
	SecretKeys[KeyDeepAPIKey] = true
}

// DeepAI is the separate Deep Scan machine, when one is set up.
func (s *Store) DeepAI() (AIConfig, bool) {
	if !s.SettingBool(KeyDeepEnabled) {
		return AIConfig{}, false
	}
	ai := AIConfig{Name: "Deep Scan", Provider: s.Setting(KeyDeepProvider), BaseURL: s.Setting(KeyDeepBaseURL),
		APIKey: s.Setting(KeyDeepAPIKey), JSONMode: s.SettingBool(KeyDeepJSONMode), Models: splitModels(s.Setting(KeyDeepModels)),
		Watts: s.SettingFloat(KeyDeepWatts)}
	return ai, len(ai.Models) > 0 && (ai.Provider == "anthropic" || strings.TrimSpace(ai.BaseURL) != "")
}

// powerCost is what the machine's electricity cost for a call that took so long.
func (s *Store) powerCost(ai AIConfig, took time.Duration) float64 {
	price := s.SettingFloat(KeyPowerPrice)
	if ai.Watts <= 0 || price <= 0 || took <= 0 {
		return 0
	}
	return ai.Watts / 1000 * took.Hours() * price
}

// Bench is one model's speed test: how long a rating and a Deep Scan part
// took on a sample book.
type Bench struct {
	AI              string  `json:"ai"` // main, backup or Deep Scan
	Model           string  `json:"model"`
	RatingSeconds   float64 `json:"rating_s"`
	PartSeconds     float64 `json:"part_s"`
	PartWords       int     `json:"part_words"`
	TokensPerSecond float64 `json:"tok_s"`
	At              string  `json:"at"`
	Error           string  `json:"error,omitempty"`
}

// Benchmarks returns the speed tests by "AI|model".
func (s *Store) Benchmarks() map[string]Bench {
	out := map[string]Bench{}
	_ = json.Unmarshal([]byte(s.Setting(KeyBenchmarks)), &out)
	return out
}

// SaveBench keeps a model's latest speed test.
func (s *Store) SaveBench(b Bench) error {
	all := s.Benchmarks()
	all[b.AI+"|"+b.Model] = b
	data, err := json.Marshal(all)
	if err != nil {
		return err
	}
	return s.SetSetting(KeyBenchmarks, string(data))
}

// BenchFor is the latest successful speed test of model on any machine.
func (s *Store) BenchFor(model string) (Bench, bool) {
	var best Bench
	found := false
	for _, b := range s.Benchmarks() {
		if b.Model == model && b.Error == "" && b.PartSeconds > 0 && (!found || b.At > best.At) {
			best, found = b, true
		}
	}
	return best, found
}
