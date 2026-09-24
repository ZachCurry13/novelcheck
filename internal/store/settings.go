package store

import (
	"strconv"
	"strings"
)

// Setting keys editable from the admin panel.
const (
	KeyLLMProvider       = "llm_provider" // "openai" (any OpenAI-compatible API) or "anthropic" (Claude)
	KeyLLMBaseURL        = "llm_base_url"
	KeyLLMAPIKey         = "llm_api_key"
	KeyLLMModel          = "llm_model"
	KeyLLMFallbackModel  = "llm_fallback_model"
	KeyLLMJSONMode       = "llm_json_mode"
	KeyPriceInputPerM    = "price_input_per_million"
	KeyPriceOutputPerM   = "price_output_per_million"
	KeyBatchSize         = "batch_size"
	KeyTokensPerHour     = "tokens_per_hour_cap"
	KeyScanDelaySeconds  = "scan_delay_seconds"
	KeyLLMTimeoutSeconds = "llm_timeout_seconds" // 0 = automatic (10 min on your network, 2 min for cloud)

	// Backup AI: a second provider tried when the main one fails.
	KeyBackupEnabled  = "backup_llm_enabled"
	KeyBackupProvider = "backup_llm_provider"
	KeyBackupBaseURL  = "backup_llm_base_url"
	KeyBackupAPIKey   = "backup_llm_api_key"
	KeyBackupModel    = "backup_llm_model" // one or more, comma-separated, in order
	KeyBackupJSONMode = "backup_llm_json_mode"
	KeyBackupPriceIn  = "backup_price_input_per_million"
	KeyBackupPriceOut = "backup_price_output_per_million"

	KeyGoogleBooksAPIKey  = "google_books_api_key"
	KeySMTPHost           = "smtp_host"
	KeySMTPPort           = "smtp_port"
	KeySMTPUser           = "smtp_username"
	KeySMTPPassword       = "smtp_password"
	KeySMTPFrom           = "smtp_from"
	KeyCalibrePollHours   = "calibre_poll_hours"
	KeyCalibreLibraryPath = "calibre_library_path" // folder inside the mount; "" = mount root
	KeyCheckUpdates       = "check_updates"
	KeySessionDays        = "session_days" // "keep me signed in" length; renewed while in use
	KeyTunnelEnabled      = "tunnel_enabled"
	KeyTunnelToken        = "tunnel_token"
	KeyTunnelHostname     = "tunnel_hostname"    // public address, for display and links
	KeyCalibreSrvURL      = "calibre_server_url" // Content server for one-click removal
	KeyCalibreSrvUser     = "calibre_server_user"
	KeyCalibreSrvPassword = "calibre_server_password"
	KeyCalibreSrvLibrary  = "calibre_server_library"
	KeyCalibreLastSync    = "calibre_last_sync"
	KeyCalibreLastResult  = "calibre_last_result"
)

// Defaults favour small, cheap models per the delegation strategy.
var Defaults = map[string]string{
	KeyLLMProvider:        "openai",
	KeyLLMBaseURL:         "https://api.openai.com/v1",
	KeyLLMModel:           "gpt-4o-mini",
	KeyLLMFallbackModel:   "",
	KeyLLMJSONMode:        "true",
	KeyPriceInputPerM:     "0.15",
	KeyPriceOutputPerM:    "0.60",
	KeyBatchSize:          "20",
	KeyTokensPerHour:      "25000",
	KeyScanDelaySeconds:   "2",
	KeyLLMTimeoutSeconds:  "0",
	KeyBackupEnabled:      "false",
	KeyBackupProvider:     "openai",
	KeyBackupBaseURL:      "",
	KeyBackupModel:        "",
	KeyBackupJSONMode:     "true",
	KeyBackupPriceIn:      "0",
	KeyBackupPriceOut:     "0",
	KeySMTPPort:           "587",
	KeyCalibrePollHours:   "6",
	KeyCalibreLibraryPath: "",
	KeyCheckUpdates:       "true",
	KeySessionDays:        "30", // main overrides with NOVELCHECK_SESSION_DAYS
	KeyCalibreLastSync:    "",
	KeyCalibreLastResult:  "",
}

// SecretKeys are never returned to the browser in clear text.
var SecretKeys = map[string]bool{KeyLLMAPIKey: true, KeyBackupAPIKey: true, KeySMTPPassword: true, KeyGoogleBooksAPIKey: true, KeyTunnelToken: true, KeyCalibreSrvPassword: true}

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

// LLMModels returns the models to try, in order: the main model, then each
// fallback (the fallback setting may hold several, comma-separated).
func (s *Store) LLMModels() []string {
	var out []string
	seen := map[string]bool{}
	add := func(m string) {
		if m = strings.TrimSpace(m); m != "" && !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	add(s.Setting(KeyLLMModel))
	for _, m := range strings.Split(s.Setting(KeyLLMFallbackModel), ",") {
		add(m)
	}
	return out
}
