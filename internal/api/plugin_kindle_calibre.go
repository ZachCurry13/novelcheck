package api

import (
	"archive/zip"
	"encoding/binary"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// Adding "Copy my Kindle" books to Calibre, and telling protected (locked) books from plain ones.

// KeyKindleToCalibre switches on adding unprotected Kindle books to Calibre by "Copy my Kindle" (off unless "true").
// Off by default: the whole-Kindle backup keeps every file anyway, and adding to Calibre made second copies of books
// the library already had under a slightly different name.
const KeyKindleToCalibre = "kindle_add_to_calibre"

// addKindleToCalibre adds an unlocked e-book to Calibre (when switched on) unless the library already has that title.
func (s *Server) addKindleToCalibre(r *http.Request, file, name string) bool {
	if s.Store.Setting(KeyKindleToCalibre) != "true" || s.Store.Setting(store.KeyCalibreSrvURL) == "" || !ebookExts[ext(name)] || ext(name) == ".azw" {
		return false
	}
	stem := strings.TrimSuffix(name, path.Ext(name))
	title, author := stem, ""
	// every way a file can be named: "Author - Title", "Title - Author" (also "Last, First"), or just the title
	cands := []string{stem}
	if x, y, ok := strings.Cut(stem, " - "); ok {
		cands = append(cands, x, y)
		author, title = x, y
	}
	for _, t := range cands {
		key, _, _ := strings.Cut(store.NormKey(t, ""), "|")
		if key != "" && s.Store.LibraryBookByTitleKey(key) != 0 {
			return false
		}
	}
	if s.Store.LibraryBookByKey(store.NormKey(title, author)) != 0 {
		return false
	}
	data, err := os.ReadFile(file)
	if err != nil || len(data) > maxEbookBytes {
		return false
	}
	if _, err := s.calibreClient().AddBook(r.Context(), name, data); err != nil {
		return false
	}
	s.syncSoon(nil)
	return true
}

// kindleLocked says whether a file is protected (DRM): Kindle formats that only open with the owner's account, an
// EPUB with encrypted content (fonts alone don't count) or a MOBI/AZW3 whose header says it is encrypted.
func kindleLocked(file string) bool {
	switch ext(file) {
	case ".kfx", ".azw", ".azw1":
		return true
	case ".epub", ".kepub":
		zr, err := zip.OpenReader(file)
		if err != nil {
			return false
		}
		defer zr.Close()
		for _, f := range zr.File {
			if f.Name != "META-INF/encryption.xml" {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				return true
			}
			b, _ := io.ReadAll(io.LimitReader(rc, 1<<20))
			_ = rc.Close()
			s := strings.ToLower(string(b))
			return strings.Contains(s, "<enc:encrypteddata") && !(strings.Contains(s, "fontobfuscation") || strings.Contains(s, "2008/embedding")) ||
				strings.Contains(s, "adobe") || strings.Contains(s, "adept")
		}
	case ".mobi", ".azw3", ".prc":
		f, err := os.Open(file)
		if err != nil {
			return false
		}
		defer f.Close()
		head := make([]byte, 96)
		if _, err := io.ReadFull(f, head); err != nil {
			return false
		}
		rec0 := int64(binary.BigEndian.Uint32(head[78:82]))
		var enc [2]byte
		if _, err := f.ReadAt(enc[:], rec0+12); err != nil {
			return false
		}
		return binary.BigEndian.Uint16(enc[:]) != 0
	}
	return false
}
