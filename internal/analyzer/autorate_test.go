package analyzer

import (
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func autoStore(t *testing.T) *store.Store {
	t.Helper()
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return store.New(d)
}

func TestAutoRateDefaultFollowsTheAI(t *testing.T) {
	st := autoStore(t)
	if AutoRateOn(st) { // the default AI is OpenAI: a bill, so off
		t.Fatal("on with a paid AI")
	}
	_ = st.SetSetting(store.KeyLLMBaseURL, "http://192.168.1.20:11434")
	if !AutoRateOn(st) {
		t.Fatal("off with a local AI")
	}
	_ = st.SetSetting(store.KeyAutoRate, "off")
	if AutoRateOn(st) {
		t.Fatal("the admin turned it off")
	}
	_ = st.SetSetting(store.KeyLLMBaseURL, "https://api.openai.com/v1")
	_ = st.SetSetting(store.KeyAutoRate, "on")
	if !AutoRateOn(st) {
		t.Fatal("the admin turned it on")
	}
}

func TestQuietHours(t *testing.T) {
	st := autoStore(t)
	at := func(h int) time.Time { return time.Date(2026, 9, 28, h, 30, 0, 0, time.UTC) }
	if !RateHoursOpen(st, at(12)) {
		t.Fatal("no hours set: any time")
	}
	_ = st.SetSetting(store.KeyAutoRateHours, "23-7")
	for h, want := range map[int]bool{22: false, 23: true, 2: true, 6: true, 7: false, 12: false} {
		if RateHoursOpen(st, at(h)) != want {
			t.Fatalf("23-7 at %d:30 UTC: want %v", h, want)
		}
	}
	// The hours are the admin's: 23:30 in UTC-5 is 04:30 UTC.
	_ = st.SetSetting(store.KeyAutoRateTZ, "America/Chicago")
	if !RateHoursOpen(st, time.Date(2026, 9, 29, 4, 30, 0, 0, time.UTC)) || RateHoursOpen(st, at(23)) {
		t.Fatal("hours not in the admin's time zone")
	}
	_ = st.SetSetting(store.KeyAutoRateHours, "9-17")
	_ = st.SetSetting(store.KeyAutoRateTZ, "")
	if !RateHoursOpen(st, at(9)) || RateHoursOpen(st, at(17)) {
		t.Fatal("9-17")
	}
	for _, bad := range []string{"", "7", "24-3", "5-5", "a-b"} {
		if _, _, ok := ParseHours(bad); ok {
			t.Fatalf("%q read as hours", bad)
		}
	}
}

// An idle worker feeds itself: books someone wants first, then the newest.
func TestFeedTakesWantedBooksFirst(t *testing.T) {
	st := autoStore(t)
	cat, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	add := func(title string) int64 {
		id, _ := st.UpsertBook(title, "Author", "", "")
		_ = st.AddCopy(cat, id, "/c/"+title+".epub", "epub", title)
		return id
	}
	old, wanted, newest := add("Old"), add("Wanted"), add("Newest")
	_, _ = st.DB.Exec(`UPDATE catalog_books SET added_at = datetime('now', '-2 days') WHERE book_id = ?`, old)
	_, _ = st.DB.Exec(`UPDATE catalog_books SET added_at = datetime('now', '-1 days') WHERE book_id = ?`, wanted)
	u, _ := st.CreateUserAge("reader", "password1", store.RoleRestricted, 0)
	_ = st.Enqueue(u.ID, wanted)
	_ = st.SetSetting(store.KeyBatchSize, "2")

	w := New(st)
	if n := w.feed(time.Now()); n != 0 {
		t.Fatalf("fed %d with a paid AI and no choice made", n)
	}
	_ = st.SetSetting(store.KeyAutoRate, "on")
	if n := w.feed(time.Now()); n != 2 || len(w.queue) != 2 || w.queue[0] != wanted || w.queue[1] != newest {
		t.Fatalf("fed %d: %v (wanted %d, newest %d, old %d)", n, w.queue, wanted, newest, old)
	}
	if st.WaitingToRate() != 3 { // two queued, one pending
		t.Fatalf("waiting %d", st.WaitingToRate())
	}
}
