package store

import "strings"

// AIConfig is one AI provider NovelCheck can use: the main one, or the
// optional backup that's tried when the main one fails.
type AIConfig struct {
	Name     string // "main" or "backup"
	Provider string // "openai" (any OpenAI-compatible API) or "anthropic"
	BaseURL  string
	APIKey   string
	JSONMode bool
	Models   []string // tried in order
	PriceIn  float64  // USD per 1M input tokens
	PriceOut float64
}

// AIConfigs returns the AIs to try, in order: the main AI, then the backup
// when it's turned on and has a model.
func (s *Store) AIConfigs() []AIConfig {
	out := []AIConfig{{
		Name: "main", Provider: s.Setting(KeyLLMProvider), BaseURL: s.Setting(KeyLLMBaseURL),
		APIKey: s.Setting(KeyLLMAPIKey), JSONMode: s.SettingBool(KeyLLMJSONMode), Models: s.LLMModels(),
		PriceIn: s.SettingFloat(KeyPriceInputPerM), PriceOut: s.SettingFloat(KeyPriceOutputPerM),
	}}
	if s.SettingBool(KeyBackupEnabled) {
		b := AIConfig{
			Name: "backup", Provider: s.Setting(KeyBackupProvider), BaseURL: s.Setting(KeyBackupBaseURL),
			APIKey: s.Setting(KeyBackupAPIKey), JSONMode: s.SettingBool(KeyBackupJSONMode),
			Models:  splitModels(s.Setting(KeyBackupModel)),
			PriceIn: s.SettingFloat(KeyBackupPriceIn), PriceOut: s.SettingFloat(KeyBackupPriceOut),
		}
		if len(b.Models) > 0 && (b.Provider == "anthropic" || strings.TrimSpace(b.BaseURL) != "") {
			out = append(out, b)
		}
	}
	return out
}

func splitModels(list string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range strings.Split(list, ",") {
		if m = strings.TrimSpace(m); m != "" && !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}
