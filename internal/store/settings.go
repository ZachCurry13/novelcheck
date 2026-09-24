package store

import (
	"strconv"
	"strings"
)

// Setting keys editable from the admin panel.
const (
	KeyLLMBaseURL         = "llm_base_url"
	KeyLLMAPIKey          = "llm_api_key"
	KeyLLMModel           = "llm_model"
	KeyLLMFallbackModel   = "llm_fallback_model"
	KeyLLMJSONMode        = "llm_json_mode"
	KeyPriceInputPerM     = "price_input_per_million"
	KeyPriceOutputPerM    = "price_output_per_million"
	KeyBatchSize          = "batch_size"
	KeyTokensPerHour      = "tokens_per_hour_cap"
	KeyScanDelaySeconds   = "scan_delay_seconds"
	KeyGoogleBooksAPIKey  = "google_books_api_key"
	KeySMTPHost           = "smtp_host"
	KeySMTPPort           = "smtp_port"
	KeySMTPUser           = "smtp_username"
	KeySMTPPassword       = "smtp_password"
	KeySMTPFrom           = "smtp_from"
	KeyCalibrePollHours   = "calibre_poll_hours"
	KeyCalibreLibraryPath = "calibre_library_path" // folder inside the mount; "" = mount root
	KeyCheckUpdates       = "check_updates"
	KeyCalibreLastSync    = "calibre_last_sync"
	KeyCalibreLastResult  = "calibre_last_result"
)

// Defaults favour small, cheap models per the delegation strategy.
var Defaults = map[string]string{
	KeyLLMBaseURL:         "https://api.openai.com/v1",
	KeyLLMModel:           "gpt-4o-mini",
	KeyLLMFallbackModel:   "",
	KeyLLMJSONMode:        "true",
	KeyPriceInputPerM:     "0.15",
	KeyPriceOutputPerM:    "0.60",
	KeyBatchSize:          "20",
	KeyTokensPerHour:      "25000",
	KeyScanDelaySeconds:   "2",
	KeySMTPPort:           "587",
	KeyCalibrePollHours:   "6",
	KeyCalibreLibraryPath: "",
	KeyCheckUpdates:       "true",
	KeyCalibreLastSync:    "",
	KeyCalibreLastResult:  "",
}

// SecretKeys are never returned to the browser in clear text.
var SecretKeys = map[string]bool{KeyLLMAPIKey: true, KeySMTPPassword: true, KeyGoogleBooksAPIKey: true}

// AllSettings returns stored values merged over defaults.
func (s *Store) AllSettings() (map[string]string, error) {
	out := make(map[string]string, len(Defaults))
	for k, v := range Defaults {
		out[k] = v
	}
	var rows []struct {
		Key   string `db:"key"`
		Value string `db:"value"`
	}
	if err := s.DB.Select(&rows, `SELECT key, value FROM settings`); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.Key] = r.Value
	}
	return out, nil
}

func (s *Store) Setting(key string) string {
	var v string
	if err := s.DB.Get(&v, `SELECT value FROM settings WHERE key = ?`, key); err != nil {
		return Defaults[key]
	}
	return v
}

func (s *Store) SettingInt(key string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s.Setting(key)))
	return n
}

func (s *Store) SettingFloat(key string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s.Setting(key)), 64)
	return f
}

func (s *Store) SettingBool(key string) bool {
	b, _ := strconv.ParseBool(s.Setting(key))
	return b
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.DB.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}
