// Package config loads process-level configuration from environment variables.
// Runtime-tunable options (LLM, SMTP, rate caps, polling) live in the settings
// table instead so admins can change them from the UI.
package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Addr          string   // HTTP listen address
	DataDir       string   // directory holding novelcheck.db (SSD dataset)
	CalibreDir    string   // Calibre library mount (HDD pool), read-only
	AdminUser     string   // bootstrap admin username
	AdminPassword string   // bootstrap admin password (random if empty)
	TrustProxy    bool     // honor X-Forwarded-For / CF-Connecting-IP
	CORSOrigins   []string // allowed cross-origin callers (empty = same-origin only)
	SessionDays   int      // session lifetime
}

func Load() Config {
	return Config{
		Addr:          env("NOVELCHECK_ADDR", ":8080"),
		DataDir:       env("NOVELCHECK_DATA_DIR", "/data"),
		CalibreDir:    env("NOVELCHECK_CALIBRE_DIR", "/calibre"),
		AdminUser:     env("NOVELCHECK_ADMIN_USER", "admin"),
		AdminPassword: os.Getenv("NOVELCHECK_ADMIN_PASSWORD"),
		TrustProxy:    envBool("NOVELCHECK_TRUST_PROXY", true),
		CORSOrigins:   splitList(os.Getenv("NOVELCHECK_CORS_ORIGINS")),
		SessionDays:   envInt("NOVELCHECK_SESSION_DAYS", 30),
	}
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	if b, err := strconv.ParseBool(os.Getenv(key)); err == nil {
		return b
	}
	return def
}

func envInt(key string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(key)); err == nil && n > 0 {
		return n
	}
	return def
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
