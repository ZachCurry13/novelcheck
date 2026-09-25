package covers

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const openLibrary = "https://covers.openlibrary.org"

var isbnRE = regexp.MustCompile(`^(\d{9}[\dX]|\d{13})$`)

// OpenLibrary returns the Open Library cover for isbn (for books that aren't
// in Calibre, such as ones found with Check a book), fetched once and kept.
// A miss is remembered for a week so the same book isn't asked about on
// every page.
func (c Cache) OpenLibrary(ctx context.Context, isbn string) (string, os.FileInfo, bool) {
	isbn = strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(isbn))
	if c.Dir == "" || !isbnRE.MatchString(isbn) {
		return "", nil, false
	}
	out := filepath.Join(c.Dir, "ol-"+isbn+".jpg")
	if info, err := os.Stat(out); err == nil {
		return out, info, true
	}
	miss := out + ".none"
	if info, err := os.Stat(miss); err == nil && time.Since(info.ModTime()) < 7*24*time.Hour {
		return "", nil, false
	}
	base := c.OpenLibraryURL
	if base == "" {
		base = openLibrary
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/b/isbn/"+isbn+"-M.jpg?default=false", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, false // offline: try again next time
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if res.StatusCode != http.StatusOK || err != nil || len(body) < 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "image/") {
		if res.StatusCode == http.StatusNotFound {
			_ = os.MkdirAll(c.Dir, 0o755)
			_ = os.WriteFile(miss, nil, 0o644)
		}
		return "", nil, false
	}
	if os.MkdirAll(c.Dir, 0o755) != nil || os.WriteFile(out, body, 0o644) != nil {
		return "", nil, false
	}
	info, err := os.Stat(out)
	return out, info, err == nil
}
