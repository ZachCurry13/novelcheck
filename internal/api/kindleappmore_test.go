package api_test

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// The Kindle app's Discover shows only listed books the family already has and can send; its Collections include the
// AI-made ones (not an idea nobody kept), and a collection's books are the ones that can be downloaded.
func TestKindleAppDiscoverAndCollections(t *testing.T) {
	lib := t.TempDir()
	srv, st := setupWith(t, func(s *api.Server) { s.Cfg.CalibreDir = lib })
	_ = st.SetSetting(store.KeyModuleKOReader, "true")
	admin := login(t, srv, "admin", "adminpass1")
	_, ks := admin.do("GET", "/api/me/kosync", nil, false)
	sum := md5.Sum([]byte(ks["code"].(string)))
	get := func(path string) (int, string) {
		req, _ := http.NewRequest("GET", srv.URL+path, nil)
		req.Header.Set("x-auth-user", "admin")
		req.Header.Set("x-auth-key", hex.EncodeToString(sum[:]))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	have, _ := st.UpsertBook("The Family Has It", "Writer", "9780000000011", "")
	p := filepath.Join(lib, "have.epub")
	_ = os.WriteFile(p, []byte("epub"), 0o644)
	_ = st.AddCopy(cal, have, p, "epub", "7")
	if _, err := st.SaveDiscoverList("nyt:combined-print-and-e-book-fiction", []store.DiscoverEntry{
		{Rank: 1, Title: "The Family Has It", Author: "Writer", ISBN: "9780000000011"},
		{Rank: 2, Title: "Nobody Has This", Author: "Other", ISBN: "9780000000022"},
	}); err != nil {
		t.Fatal(err)
	}
	code, body := get("/kosync/plugin/discover?device=Kindle")
	if code != 200 || !strings.Contains(body, "The Family Has It") || strings.Contains(body, "Nobody Has This") {
		t.Fatalf("only books the family has: %d %s", code, body)
	}
	ai, _ := st.CreateCollection(store.NewCollection{Name: "Cozy Winter Reads", Icon: "❄️", Kind: "ai", By: "AI"})
	idea, _ := st.CreateCollection(store.NewCollection{Name: "Unkept Idea", Icon: "💡", Kind: "idea", By: "AI"})
	_, _ = st.AddToCollection(ai, []int64{have}, nil)
	_, _ = st.AddToCollection(idea, []int64{have}, nil)
	code, body = get("/kosync/plugin/collections")
	if code != 200 || !strings.Contains(body, "Cozy Winter Reads") || !strings.Contains(body, `"ai":true`) || strings.Contains(body, "Unkept Idea") {
		t.Fatalf("collections, the AI-made one included, not an unkept idea: %d %s", code, body)
	}
	code, body = get(fmt.Sprintf("/kosync/plugin/collection/%d?device=Kindle", ai))
	if code != 200 || !strings.Contains(body, "The Family Has It") || !strings.Contains(body, "Cozy Winter Reads") {
		t.Fatalf("a collection's books: %d %s", code, body)
	}
	if code, _ := get("/kosync/plugin/collection/99999"); code != 404 {
		t.Fatalf("no such collection: %d", code)
	}
}
