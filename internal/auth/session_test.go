package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func newManager(t *testing.T) (*Manager, *store.Store) {
	t.Helper()
	d, err := db.OpenDSN(fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=foreign_keys(1)", t.Name()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	st := store.New(d)
	hash, _ := HashPassword("password123")
	if _, err := st.CreateUser("mom", hash, store.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	return &Manager{Store: st, SessionDays: 30}, st
}

func login(t *testing.T, m *Manager, remember bool) *http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	if _, err := m.Login(rec, httptest.NewRequest("POST", "/", nil), "mom", "password123", remember); err != nil {
		t.Fatal(err)
	}
	return rec.Result().Cookies()[0]
}

// A regular user's sign-in rolls forward while they keep using the app.
func TestRememberedSessionRenewsWhileActive(t *testing.T) {
	m, st := newManager(t)
	c := login(t, m, true)
	if c.MaxAge != 30*86400 {
		t.Fatalf("remembered cookie MaxAge = %d", c.MaxAge)
	}
	// Pretend the last activity was 20 days ago (10 days left).
	st.DB.MustExec(`UPDATE sessions SET expires_at = datetime('now', '+10 days')`)

	ok := m.RequireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest("GET", "/api/me", nil)
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	ok.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("request rejected: %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Set-Cookie"), "Max-Age=2592000") {
		t.Fatalf("cookie not refreshed: %q", rec.Header().Get("Set-Cookie"))
	}
	sess, err := st.SessionByToken(hashToken(c.Value))
	if err != nil || sess.SecondsLeft < 29*86400 {
		t.Fatalf("expiry not extended: %+v %v", sess, err)
	}

	// A second request right away doesn't write again.
	rec = httptest.NewRecorder()
	ok.ServeHTTP(rec, req)
	if rec.Header().Get("Set-Cookie") != "" {
		t.Fatal("renewal should be throttled to once a day")
	}
}

func TestSessionLengthSettingAndBrowserOnlyLogin(t *testing.T) {
	m, st := newManager(t)
	_ = st.SetSetting(store.KeySessionDays, "7")
	if c := login(t, m, true); c.MaxAge != 7*86400 {
		t.Fatalf("admin setting ignored: MaxAge %d", c.MaxAge)
	}
	c := login(t, m, false)
	if c.MaxAge != 0 {
		t.Fatalf("browser-only login should have no Max-Age, got %d", c.MaxAge)
	}
	sess, err := st.SessionByToken(hashToken(c.Value))
	if err != nil || sess.Remember || sess.SecondsLeft > shortSession {
		t.Fatalf("unexpected short session %+v %v", sess, err)
	}
}
