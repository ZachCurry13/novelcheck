package analyzer_test

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zachcurry13/novelcheck/internal/analyzer"
	"github.com/zachcurry13/novelcheck/internal/db"
	"github.com/zachcurry13/novelcheck/internal/store"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return store.New(d)
}

// waitNotice waits for an unread notification from source and returns it.
func waitNotice(t *testing.T, st *store.Store, source string) store.Notification {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		items, _, _ := st.Notifications(20)
		for _, n := range items {
			if n.Source == source && !n.Read {
				return n
			}
		}
	}
	t.Fatalf("no %q notification", source)
	return store.Notification{}
}

func TestBatchFinishedNotice(t *testing.T) {
	st := newStore(t)
	var calls atomic.Int32
	srv := fakeLLM(&calls)
	defer srv.Close()
	for k, v := range map[string]string{store.KeyLLMBaseURL: srv.URL, store.KeyLLMModel: "big-model", store.KeyScanDelaySeconds: "0"} {
		_ = st.SetSetting(k, v)
	}
	var ids []int64
	for _, title := range []string{"The Hobbit", "Matilda"} {
		id, _ := st.UpsertBook(title, "Author", "", "")
		_ = st.SetBlurb(id, "A long enough blurb so no enrichment lookup is needed for this book.")
		ids = append(ids, id)
	}
	w := analyzer.New(st)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)
	w.Enqueue(false, ids...)

	n := waitNotice(t, st, "batch-done")
	if !strings.HasPrefix(n.Message, "Rating finished: 2 books rated · 1,100 tokens · $0.0002") || n.Level != "info" {
		t.Fatalf("summary: %+v", n)
	}
}

func TestTokenCapNotice(t *testing.T) {
	st := newStore(t)
	_ = st.SetSetting(store.KeyTokensPerHour, "500")
	_ = st.SetSetting(store.KeyLLMBaseURL, "http://127.0.0.1:1") // never reached: the cap blocks first
	_ = st.RecordUsage(0, "earlier", 1000, 0)
	id, _ := st.UpsertBook("Capped", "Author", "", "")
	_ = st.SetBlurb(id, "A long enough blurb so no enrichment lookup is needed for this book.")
	w := analyzer.New(st)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)
	w.Enqueue(false, id)

	if n := waitNotice(t, st, "token-cap"); !strings.Contains(n.Message, "500 tokens") || n.Level != "warning" {
		t.Fatalf("cap notice: %+v", n)
	}
}

func TestRoutineNoticesCanBeTurnedOff(t *testing.T) {
	st := newStore(t)
	_ = st.SetSetting(store.KeyNotifyRoutine, "false")
	st.NotifyRoutine("batch-done", "Rating finished", "")
	if items, _, _ := st.Notifications(5); len(items) != 0 {
		t.Fatalf("routine notices are off: %+v", items)
	}
	before := time.Now().Add(-time.Second)
	_, _ = st.UpsertBook("New Book", "Author", "", "")
	if n := st.BooksCreatedSince(before); n != 1 {
		t.Fatalf("books created since: %d", n)
	}
	if n := st.BooksCreatedSince(time.Now().Add(time.Hour)); n != 0 {
		t.Fatalf("future: %d", n)
	}
}
