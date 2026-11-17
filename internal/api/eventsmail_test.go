package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// One press copies the Send-to-Kindle email account into the BookBub mailbox (with the matching IMAP server) and
// switches the BookBub event on; it asks for the SMTP details first when they aren't saved.
func TestBookBubMailboxFromSMTPAccount(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	if res, _ := admin.do("POST", "/api/admin/events-mail/use-smtp", map[string]string{}, true); res.StatusCode != 400 {
		t.Fatalf("nothing saved yet: %d", res.StatusCode)
	}
	for host, want := range map[string]string{
		"smtp.gmail.com":       "imap.gmail.com:993",
		"smtp.office365.com":   "outlook.office365.com:993",
		"smtp.example.org:587": "imap.example.org:993",
		"mail.weird.example":   store.Defaults[store.KeyEventsMailHost], // can't be told: what was there stays
	} {
		_ = st.SetSettings(map[string]string{store.KeySMTPHost: host, store.KeySMTPUser: "mum@example.org", store.KeySMTPPassword: "app-password-123", store.KeyEventsAutoBookBub: "false",
			store.KeyEventsMailHost: store.Defaults[store.KeyEventsMailHost]}) // each case starts from the default server
		res, out := admin.do("POST", "/api/admin/events-mail/use-smtp", map[string]string{}, true)
		if res.StatusCode != 200 || out["user"] != "mum@example.org" || out["host"] != want {
			t.Fatalf("%s: %d %v (want %s)", host, res.StatusCode, out, want)
		}
		if st.Setting(store.KeyEventsMailUser) != "mum@example.org" || st.Setting(store.KeyEventsMailPassword) != "app-password-123" ||
			st.Setting(store.KeyEventsMailHost) != want || !st.SettingBool(store.KeyEventsAutoBookBub) {
			t.Fatalf("%s: saved settings wrong", host)
		}
	}
	// The password is never sent back to the page.
	if _, out := admin.do("GET", "/api/admin/settings", nil, false); out != nil {
		if s, ok := out["events_mail_password"].(string); ok && s == "app-password-123" {
			t.Fatal("the app password was sent to the page")
		}
	}
	admin.do("POST", "/api/admin/users", map[string]any{"username": "kid", "password": "kidpass12"}, true)
	kid := login(t, srv, "kid", "kidpass12")
	if res, _ := kid.do("POST", "/api/admin/events-mail/use-smtp", map[string]string{}, true); res.StatusCode == 200 {
		t.Fatal("not for kids")
	}
}

// One press makes the Deep Scan machine the backup AI (free, on the home network), and says so plainly when no Deep
// Scan machine is set up.
func TestDeepMachineBecomesTheBackupAI(t *testing.T) {
	srv, st := setup(t)
	admin := login(t, srv, "admin", "adminpass1")
	if res, _ := admin.do("POST", "/api/admin/backup-ai/use-deep", map[string]string{}, true); res.StatusCode != 400 {
		t.Fatalf("no Deep Scan machine yet: %d", res.StatusCode)
	}
	_ = st.SetSettings(map[string]string{store.KeyDeepEnabled: "true", store.KeyDeepProvider: "openai", store.KeyDeepBaseURL: "http://192.168.1.50:11434/v1",
		store.KeyDeepModels: "llama3.1:8b,qwen2.5:7b", store.KeyBackupEnabled: "false", store.KeyBackupBaseURL: "https://generativelanguage.googleapis.com/v1beta/openai", store.KeyBackupModel: "gemini-flash"})
	res, out := admin.do("POST", "/api/admin/backup-ai/use-deep", map[string]string{}, true)
	if res.StatusCode != 200 || out["models"] != "llama3.1:8b,qwen2.5:7b" {
		t.Fatalf("use deep: %d %v", res.StatusCode, out)
	}
	ais := st.AIConfigs()
	if len(ais) != 2 || ais[1].Name != "backup" || ais[1].BaseURL != "http://192.168.1.50:11434/v1" || len(ais[1].Models) != 2 || ais[1].PriceIn != 0 {
		t.Fatalf("the backup is the Deep Scan machine now: %+v", ais)
	}
}

// One press makes Google's free Gemini the backup AI: the models are what Google lists for the key (the newest
// fast one first), not a guess, with the key pasted or one already saved for Google.
func TestGoogleBecomesTheBackupAI(t *testing.T) {
	google := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key-123" {
			http.Error(w, "no", http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"models/gemini-2.5-flash"},{"id":"models/gemini-3.8-flash"},{"id":"models/gemini-3.8-flash-lite"},{"id":"models/gemini-3.8-flash-preview-tts"},{"id":"models/gemini-3.5-flash-lite"},{"id":"models/text-embedding-9"},{"id":"models/gemma-4-26b-it"},{"id":"models/gemma-4-31b-it"},{"id":"models/gemma-3-27b-it"}]}`))
	}))
	defer google.Close()
	srv, st := setupWith(t, func(s *api.Server) { s.GoogleBase = google.URL })
	admin := login(t, srv, "admin", "adminpass1")
	if res, _ := admin.do("POST", "/api/admin/backup-ai/use-google", map[string]string{}, true); res.StatusCode != 400 {
		t.Fatalf("no key yet: %d", res.StatusCode)
	}
	if res, _ := admin.do("POST", "/api/admin/backup-ai/use-google", map[string]string{"key": "wrong"}, true); res.StatusCode != 502 {
		t.Fatalf("a wrong key is said plainly: %d", res.StatusCode)
	}
	res, out := admin.do("POST", "/api/admin/backup-ai/use-google", map[string]string{"key": "test-key-123"}, true)
	if res.StatusCode != 200 || out["models"] != "gemini-3.8-flash-lite,gemini-3.5-flash-lite,gemma-4-31b-it" {
		t.Fatalf("use google: %d %v", res.StatusCode, out)
	}
	ais := st.AIConfigs()
	if len(ais) != 2 || ais[1].APIKey != "test-key-123" || len(ais[1].Models) != 3 || ais[1].PriceIn != 0 {
		t.Fatalf("the backup is Google now: %+v", ais)
	}
	_ = st.SetSetting(store.KeyBackupBaseURL, "https://generativelanguage.googleapis.com/v1beta/openai")
	if res, _ := admin.do("POST", "/api/admin/backup-ai/use-google", map[string]string{"key": "********"}, true); res.StatusCode != 200 {
		t.Fatal("a saved Google key is reused when the box shows stars")
	}
}

// One press makes Google the main AI and the AI that was the main one the backup; the pause between books becomes 4
// seconds; the free Gemini models are the ones Google lists, two flash-lite first and the biggest Gemma 4 last.
func TestGoogleBecomesTheMainAIAndLocalTheBackup(t *testing.T) {
	google := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"models/gemini-3.5-flash-lite"},{"id":"models/gemini-3.1-flash-lite"},{"id":"models/gemma-4-31b-it"},{"id":"models/gemini-3.8-flash"}]}`))
	}))
	defer google.Close()
	srv, st := setupWith(t, func(s *api.Server) { s.GoogleBase = google.URL })
	admin := login(t, srv, "admin", "adminpass1")
	_ = st.SetSettings(map[string]string{store.KeyLLMProvider: "openai", store.KeyLLMBaseURL: "http://192.168.1.50:11434/v1", store.KeyLLMModel: "llama3.2",
		store.KeyLLMFallbackModel: "qwen2.5:3b", store.KeyMainWatts: "150", store.KeyScanDelaySeconds: "2"})
	res, out := admin.do("POST", "/api/admin/main-ai/use-google", map[string]string{"key": "k-1"}, true)
	if res.StatusCode != 200 || out["models"] != "gemini-3.5-flash-lite,gemini-3.1-flash-lite,gemma-4-31b-it" {
		t.Fatalf("use google first: %d %v", res.StatusCode, out)
	}
	ais := st.AIConfigs()
	if len(ais) != 2 || ais[0].BaseURL != google.URL || ais[0].Models[0] != "gemini-3.5-flash-lite" || len(ais[0].Models) != 3 || ais[0].Watts != 0 {
		t.Fatalf("Google is the main AI now: %+v", ais)
	}
	if ais[1].BaseURL != "http://192.168.1.50:11434/v1" || len(ais[1].Models) != 2 || ais[1].Models[0] != "llama3.2" || ais[1].Watts != 150 {
		t.Fatalf("the home AI is the backup now: %+v", ais[1])
	}
	if st.Setting(store.KeyScanDelaySeconds) != "4" {
		t.Fatal("4 seconds between books")
	}
	// pressed again, Google is already the main one: the backup stays what it was
	admin.do("POST", "/api/admin/main-ai/use-google", map[string]string{"key": "k-1"}, true)
	if ais = st.AIConfigs(); len(ais) != 2 || ais[1].BaseURL != "http://192.168.1.50:11434/v1" {
		t.Fatalf("the backup is not overwritten by Google: %+v", ais)
	}
}
