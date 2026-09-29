package api_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// A Kindle reader's Up Next book with nothing Amazon takes (only on an
// imported list, or only AZW3 in Calibre) says so, and ▶ marks it as
// reading instead of failing to email it.
func TestStartReadingWithoutAFileToSend(t *testing.T) {
	lib := t.TempDir()
	srv, st := setupWith(t, func(s *api.Server) { s.Cfg.CalibreDir = lib })
	admin := login(t, srv, "admin", "adminpass1")
	admin.do("PUT", "/api/me/delivery", map[string]string{"delivery_method": "email", "kindle_email": "reader@kindle.com"}, true)
	_ = st.SetSetting(store.KeyModuleKindle, "true")

	list, _ := st.EnsureCatalog("Reader's Kindle", "drive")
	onList, _ := st.UpsertBook("Only On A List", "Author", "", "")
	_ = st.AddCopy(list, onList, "", "list", "")
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	azw, _ := st.UpsertBook("Only Azw Three", "Author", "", "")
	p := filepath.Join(lib, "a.azw3")
	_ = os.WriteFile(p, []byte("x"), 0o644)
	_ = st.AddCopy(cal, azw, p, "azw3", "1")
	for _, id := range []int64{onList, azw} {
		admin.do("POST", "/api/queue", map[string]int64{"book_id": id}, true)
	}

	_, items := admin.doList("GET", "/api/queue")
	files := map[string]string{}
	ids := map[string]string{}
	for _, it := range items {
		files[it["title"].(string)] = it["file"].(string)
		ids[it["title"].(string)] = itoa(int64(it["id"].(float64)))
	}
	if files["Only On A List"] != "none" || files["Only Azw Three"] != "other" {
		t.Fatalf("files: %v", files)
	}
	res, out := admin.do("POST", "/api/queue/"+ids["Only On A List"]+"/start", nil, true)
	if res.StatusCode != 200 || out["status"] != "reading" || !strings.Contains(out["delivery_note"].(string), "Nothing was sent") {
		t.Fatalf("list book: %d %v", res.StatusCode, out)
	}
	res, out = admin.do("POST", "/api/queue/"+ids["Only Azw Three"]+"/start", nil, true)
	if res.StatusCode != 200 || !strings.Contains(out["delivery_note"].(string), "AZW3") {
		t.Fatalf("azw3 book: %d %v", res.StatusCode, out)
	}
}
