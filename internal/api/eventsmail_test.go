package api_test

import (
	"testing"

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
