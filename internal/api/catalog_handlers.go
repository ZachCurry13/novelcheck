package api

import (
	"net/http"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func (s *Server) handleListCatalogs(w http.ResponseWriter, r *http.Request) {
	cs, err := s.Store.ListCatalogs()
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if cs == nil {
		cs = []store.Catalog{}
	}
	writeJSON(w, http.StatusOK, cs)
}

func validCatalogName(n string) (string, bool) {
	n = strings.TrimSpace(n)
	return n, n != "" && len(n) <= 80
}

func (s *Server) handleCreateCatalog(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name   string `json:"name"`
		Source string `json:"source"`
	}
	if !readJSON(w, r, &body, 4<<10) {
		return
	}
	name, ok := validCatalogName(body.Name)
	if !ok {
		writeErr(w, http.StatusBadRequest, "catalog name must be 1-80 characters")
		return
	}
	if body.Source != "drive" {
		body.Source = "custom"
	}
	if strings.EqualFold(name, store.CalibreCatalogName) {
		writeErr(w, http.StatusBadRequest, "that name is reserved for the Calibre library")
		return
	}
	id, err := s.Store.EnsureCatalog(name, body.Source)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "name": name})
}

func (s *Server) handleRenameCatalog(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	var body struct {
		Name string `json:"name"`
	}
	if !ok || !readJSON(w, r, &body, 4<<10) {
		if !ok {
			writeErr(w, http.StatusBadRequest, "invalid id")
		}
		return
	}
	name, valid := validCatalogName(body.Name)
	if !valid || strings.EqualFold(name, store.CalibreCatalogName) {
		writeErr(w, http.StatusBadRequest, "invalid catalog name")
		return
	}
	if err := s.Store.RenameCatalog(id, name); err != nil {
		writeErr(w, http.StatusConflict, "a catalog with that name already exists")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteCatalog(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.Store.DeleteCatalog(id); err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// driveBook is one e-book discovered by the browser drive scanner.
type driveBook struct {
	Title  string `json:"title"`
	Author string `json:"author"`
	ISBN   string `json:"isbn"`
	Path   string `json:"path"`   // path relative to the picked folder
	Format string `json:"format"` // epub | mobi | azw3 | azw | kfx | pdf | list
	ASIN   string `json:"asin"`
}

var driveFormats = map[string]bool{"epub": true, "mobi": true, "azw3": true, "azw": true, "kfx": true, "pdf": true,
	"list": true} // "list" = typed/pasted title, no file

// handleImportDrive ingests metadata extracted client-side from a Kindle or
// local drive into an existing or new catalog. Only metadata is uploaded.
func (s *Server) handleImportDrive(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CatalogID   int64       `json:"catalog_id"`
		CatalogName string      `json:"catalog_name"`
		Books       []driveBook `json:"books"`
	}
	if !readJSON(w, r, &body, 8<<20) {
		return
	}
	if len(body.Books) > 20000 {
		writeErr(w, http.StatusBadRequest, "too many books in one import")
		return
	}
	catID := body.CatalogID
	if catID > 0 {
		c, err := s.Store.CatalogByID(catID)
		if err != nil {
			writeStoreErr(w, err)
			return
		}
		if c.Source == "calibre" {
			writeErr(w, http.StatusBadRequest, "cannot import drive books into the Calibre catalog")
			return
		}
	} else {
		name, ok := validCatalogName(body.CatalogName)
		if !ok || strings.EqualFold(name, store.CalibreCatalogName) {
			writeErr(w, http.StatusBadRequest, "choose a catalog or enter a valid new catalog name")
			return
		}
		var err error
		if catID, err = s.Store.EnsureCatalog(name, "drive"); err != nil {
			writeStoreErr(w, err)
			return
		}
	}
	imported, skipped := 0, 0
	for _, b := range body.Books {
		format := strings.ToLower(strings.TrimPrefix(b.Format, "."))
		title := strings.TrimSpace(b.Title)
		if !driveFormats[format] || title == "" || len(title) > 500 || len(b.Author) > 500 {
			skipped++
			continue
		}
		id, err := s.Store.UpsertBook(title, b.Author, b.ISBN, "")
		if err != nil {
			skipped++
			continue
		}
		ext := b.ASIN
		if ext == "" {
			ext = b.Path
		}
		if err := s.Store.AddCopy(catID, id, truncateStr(b.Path, 1000), format, truncateStr(ext, 1000)); err != nil {
			writeStoreErr(w, err)
			return
		}
		imported++
	}
	writeJSON(w, http.StatusOK, map[string]any{"catalog_id": catID, "imported": imported, "skipped": skipped})
}

func truncateStr(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
