package api_test

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/zachcurry13/novelcheck/internal/api"
	"github.com/zachcurry13/novelcheck/internal/kosync"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Reading in NovelCheck's reader writes your place as KOReader progress for the same EPUB file, so KOReader's
// progress sync opens the book there; a malformed place is not written, and the reader never offers its own
// place back as "your e-reader".
func TestReaderPlaceGoesToKOReader(t *testing.T) {
	var calDir string
	srv, st := setupWith(t, func(s *api.Server) { calDir = s.Cfg.CalibreDir })
	admin := login(t, srv, "admin", "adminpass1")
	file := filepath.Join(calDir, "Author", "Book (1)", "book.epub")
	_ = os.MkdirAll(filepath.Dir(file), 0o755)
	_ = os.WriteFile(file, bytes.Repeat([]byte("a chapter of a book. "), 3000), 0o644)
	cal, _ := st.EnsureCatalog(store.CalibreCatalogName, "calibre")
	id, _ := st.UpsertBook("Read Here", "Author", "", "")
	_ = st.AddCopy(cal, id, file, "epub", "1")
	doc, _ := kosync.PartialMD5(file)
	st.MarkFingerprint(file, "d41d8cd98f00b204e9800998ecf8427e", id) // kept from before Calibre rewrote the file
	path := "/api/books/" + itoa(id) + "/position"

	_, out := admin.do("PUT", path, map[string]any{"cfi": "epubcfi(/6/8!/4/2)", "fraction": 0.3, "xpointer": "/body/DocFragment[4]/script; drop"}, true)
	if out["koreader"] != false {
		t.Fatalf("a malformed place is not sent to KOReader: %v", out)
	}
	_, out = admin.do("PUT", path, map[string]any{"cfi": "epubcfi(/6/8!/4/2)", "fraction": 0.3, "xpointer": "/body/DocFragment[4]/body/div/p[7]"}, true)
	if out["koreader"] != true {
		t.Fatalf("the place is written for KOReader: %v", out)
	}

	// KOReader asks for its progress the way it does on a Kindle.
	_, me := admin.do("GET", "/api/me/kosync", nil, false)
	sum := md5.Sum([]byte(me["code"].(string)))
	req, _ := http.NewRequest("GET", srv.URL+"/kosync/syncs/progress/"+doc, nil)
	req.Header.Set("x-auth-user", "admin")
	req.Header.Set("x-auth-key", hex.EncodeToString(sum[:]))
	req.Header.Set("accept", "application/vnd.koreader.v1+json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	ko := map[string]any{}
	_ = json.NewDecoder(res.Body).Decode(&ko)
	if ko["progress"] != "/body/DocFragment[4]/body/div/p[7]" || ko["percentage"] != 0.3 || ko["device"] != "NovelCheck reader" {
		t.Fatalf("KOReader gets the reader's place: %v", ko)
	}
	if _, pos := admin.do("GET", path, nil, false); pos["koreader"] != nil {
		t.Fatalf("the reader's own place is not offered back as the e-reader's: %v", pos)
	}
}
