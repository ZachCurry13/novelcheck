package api

import (
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/audiobookshelf"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// Upload (Admin → Library cleanup): e-books go into Calibre through its Content server, audiobooks into
// Audiobookshelf. Both are experimental until "Check my servers" has shown the routes exist on the real servers;
// each upload says exactly what the other side answered.

const (
	maxEbookBytes = 200 << 20
	maxAudioBytes = 4 << 30
)

var ebookExts = map[string]bool{".epub": true, ".pdf": true, ".mobi": true, ".azw3": true, ".azw": true, ".kepub": true, ".fb2": true, ".cbz": true}
var audioExts = map[string]bool{".m4b": true, ".mp3": true, ".m4a": true, ".flac": true, ".ogg": true, ".opus": true, ".aac": true, ".wav": true,
	".jpg": true, ".jpeg": true, ".png": true, ".cue": true, ".nfo": true, ".pdf": true, ".epub": true}

func ext(name string) string { return strings.ToLower(path.Ext(name)) }

// GET /api/admin/upload/targets: where uploads can go (Audiobookshelf libraries and folders).
func (s *Server) handleUploadTargets(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{"calibre": s.Store.Setting(store.KeyCalibreSrvURL) != ""}
	if c, ok := audiobookshelf.FromSettings(s.Store); ok {
		libs, err := c.Libraries(r.Context())
		if err != nil {
			out["abs_error"] = err.Error()
		}
		var list []map[string]any
		for _, l := range libs {
			if l.MediaType != "book" {
				continue
			}
			var one struct {
				Folders []struct {
					ID       string `json:"id"`
					FullPath string `json:"fullPath"`
				} `json:"folders"`
			}
			_ = c.JSON(r.Context(), http.MethodGet, "/api/libraries/"+l.ID, nil, &one)
			var folders []map[string]string
			for _, f := range one.Folders {
				folders = append(folders, map[string]string{"id": f.ID, "path": f.FullPath})
			}
			list = append(list, map[string]any{"id": l.ID, "name": l.Name, "folders": folders})
		}
		out["abs"] = list
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /api/admin/upload/ebook: one e-book file (multipart field "file") into Calibre.
func (s *Server) handleUploadEbook(w http.ResponseWriter, r *http.Request) {
	if s.Store.Setting(store.KeyCalibreSrvURL) == "" {
		writeErr(w, http.StatusBadRequest, "set up the calibre Content server connection first (Admin → Calibre Library)")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxEbookBytes+(1<<20)) // before the reader is made, or it limits nothing
	mr, err := r.MultipartReader()
	if err != nil {
		writeErr(w, http.StatusBadRequest, "send the file as a form upload")
		return
	}
	for {
		part, err := mr.NextPart()
		if err != nil {
			writeErr(w, http.StatusBadRequest, "no file came with that")
			return
		}
		name := path.Base(part.FileName())
		if part.FormName() != "file" || name == "." || name == "" {
			continue
		}
		if !ebookExts[ext(name)] {
			writeErr(w, http.StatusBadRequest, "“"+name+"” isn't an e-book file (epub, pdf, mobi, azw3, kepub, fb2, cbz)")
			return
		}
		data, err := io.ReadAll(io.LimitReader(part, maxEbookBytes+1))
		if err != nil || len(data) > maxEbookBytes {
			writeErr(w, http.StatusRequestEntityTooLarge, "that file is larger than 200 MB")
			return
		}
		cal := s.calibreClient()
		id, err := cal.AddBook(r.Context(), name, data)
		if err != nil {
			writeErr(w, http.StatusBadGateway, "Calibre: "+err.Error())
			return
		}
		d, err := cal.Details(r.Context(), []int{id}) // read it back: is it really there?
		if err != nil || d[id].Title == "" {
			writeErr(w, http.StatusBadGateway, "Calibre said it added book #"+strconv.Itoa(id)+" but it can't be read back: check Admin → Check my servers")
			return
		}
		s.syncSoon(nil) // NovelCheck reads Calibre again so the book shows up
		writeJSON(w, http.StatusOK, map[string]any{"calibre_id": id, "title": d[id].Title, "authors": d[id].Authors})
		return
	}
}

// POST /api/admin/upload/audiobook: multipart with the fields title, author, series, library, folder first, then the
// files. They are streamed to Audiobookshelf, never kept here.
func (s *Server) handleUploadAudiobook(w http.ResponseWriter, r *http.Request) {
	c, ok := audiobookshelf.FromSettings(s.Store)
	if !ok {
		writeErr(w, http.StatusBadRequest, "connect Audiobookshelf first (Admin → Audiobookshelf)")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAudioBytes)
	mr, err := r.MultipartReader()
	if err != nil {
		writeErr(w, http.StatusBadRequest, "send the files as a form upload")
		return
	}
	fields := map[string]string{}
	var pending *struct {
		name string
		r    io.Reader
	}
	for pending == nil { // the fields come first; stop at the first file
		part, err := mr.NextPart()
		if err != nil {
			writeErr(w, http.StatusBadRequest, "no files came with that")
			return
		}
		if part.FileName() == "" {
			b, _ := io.ReadAll(io.LimitReader(part, 1024))
			fields[part.FormName()] = strings.TrimSpace(string(b))
			continue
		}
		pending = &struct {
			name string
			r    io.Reader
		}{path.Base(part.FileName()), part}
	}
	if fields["library"] == "" || fields["folder"] == "" || fields["title"] == "" {
		writeErr(w, http.StatusBadRequest, "choose a library and folder and give the book a title")
		return
	}
	var bad string
	next := func() (audiobookshelf.UploadFile, bool) {
		if pending != nil {
			f := pending
			pending = nil
			if !audioExts[ext(f.name)] {
				bad = f.name
				return audiobookshelf.UploadFile{}, false
			}
			return audiobookshelf.UploadFile{Name: f.name, R: f.r}, true
		}
		part, err := mr.NextPart()
		if err != nil || part.FileName() == "" {
			return audiobookshelf.UploadFile{}, false
		}
		name := path.Base(part.FileName())
		if !audioExts[ext(name)] {
			bad = name
			return audiobookshelf.UploadFile{}, false
		}
		return audiobookshelf.UploadFile{Name: name, R: part}, true
	}
	if err := c.Upload(r.Context(), fields["library"], fields["folder"], fields["title"], fields["author"], fields["series"], next); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if bad != "" {
		writeErr(w, http.StatusBadRequest, "“"+bad+"” isn't an audio file (m4b, mp3, m4a, flac, ogg, opus, aac, wav), a cover (jpg, png) or a cue/nfo/pdf: the rest was sent")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"sent": fields["title"]})
}
