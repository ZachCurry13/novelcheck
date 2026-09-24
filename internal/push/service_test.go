package push

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/auth"
	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// phone is a fake push service plus the browser keys it decrypts with.
type phone struct {
	srv  *httptest.Server
	ua   *ecdh.PrivateKey
	auth []byte
	mu   sync.Mutex
	got  []Message
	gone bool
}

func newPhone(t *testing.T) *phone {
	p := &phone{auth: make([]byte, 16)}
	p.ua, _ = ecdh.P256().GenerateKey(rand.Reader)
	_, _ = rand.Read(p.auth)
	p.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.gone {
			w.WriteHeader(http.StatusGone)
			return
		}
		var m Message
		_ = json.Unmarshal(decrypt(t, p.ua, p.auth, must(io.ReadAll(r.Body))), &m)
		p.got = append(p.got, m)
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(p.srv.Close)
	return p
}

func (p *phone) sub(userID int64, scope string) store.PushSub {
	return store.PushSub{UserID: userID, Endpoint: p.srv.URL + "/s", Scope: scope,
		P256dh: base64.RawURLEncoding.EncodeToString(p.ua.PublicKey().Bytes()), Auth: base64.RawURLEncoding.EncodeToString(p.auth)}
}

func (p *phone) wait(t *testing.T, n int) []Message {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		p.mu.Lock()
		got := append([]Message{}, p.got...)
		p.mu.Unlock()
		if len(got) >= n {
			return got
		}
	}
	t.Fatalf("phone got fewer than %d messages", n)
	return nil
}

func TestNoticesReachPhones(t *testing.T) {
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := store.New(d)
	if err := auth.Bootstrap(st, "admin", "adminpass1"); err != nil {
		t.Fatal(err)
	}
	svc := New(st)
	st.OnNotify = svc.FromNotice
	everything, problemsOnly := newPhone(t), newPhone(t)
	_ = st.SavePushSub(everything.sub(1, "all"))
	_ = st.SavePushSub(problemsOnly.sub(1, "problems"))

	st.NotifyRoutine("batch-done", "Rating finished: 20 books rated", "#/usage")
	st.Notify("error", "calibre-sync", "Calibre sync failed: disk full", "#/admin")
	byTag := map[string]Message{} // sent in the background, so in any order
	for _, m := range everything.wait(t, 2) {
		byTag[m.Tag] = m
	}
	if m := byTag["batch-done"]; m.Title != "NovelCheck" || m.URL != "/#/usage" {
		t.Fatalf("everyday notice: %+v", byTag)
	}
	if m := byTag["calibre-sync"]; m.Title != "⛔ NovelCheck problem" || m.Body != "Calibre sync failed: disk full" {
		t.Fatalf("problem notice: %+v", byTag)
	}
	if got := problemsOnly.wait(t, 1); len(got) != 1 || got[0].Tag != "calibre-sync" {
		t.Fatalf("problems-only phone: %+v", got)
	}

	// A repeat of an unread notice only bumps its count: no second push.
	st.Notify("error", "calibre-sync", "Calibre sync failed: disk full", "#/admin")
	time.Sleep(200 * time.Millisecond)
	if n := len(problemsOnly.wait(t, 1)); n != 1 {
		t.Fatalf("repeat was pushed again: %d", n)
	}

	// A phone that unsubscribed is forgotten.
	everything.mu.Lock()
	everything.gone = true
	everything.mu.Unlock()
	subs, _ := st.UserPushSubs(1)
	if _, err := svc.SendTo(t.Context(), subs, Message{Title: "x"}); err != nil {
		t.Fatal(err)
	}
	if subs, _ = st.UserPushSubs(1); len(subs) != 1 || subs[0].Scope != "problems" {
		t.Fatalf("gone phone should be dropped: %+v", subs)
	}

	// The key is made once and kept.
	k1, _ := svc.PublicKey()
	k2, _ := New(st).PublicKey()
	if k1 == "" || k1 != k2 {
		t.Fatalf("public key should persist: %q vs %q", k1, k2)
	}
}
