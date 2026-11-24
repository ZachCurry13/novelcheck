package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// The Kindle backup folder: what it holds (Admin), and books moved on the Kindle.

// handleKindleBackup: GET /api/admin/kindle-backup lists what the Kindle copies hold: per person and device, the
// number of files, their size and the newest ones, and the folder they are in (so it can be opened on the NAS).
func (s *Server) handleKindleBackup(w http.ResponseWriter, r *http.Request) {
	root := filepath.Join(s.Cfg.DataDir, "kindle-backup")
	type dev struct {
		User   string   `json:"user"`
		Name   string   `json:"name"`
		Files  int      `json:"files"`
		Bytes  int64    `json:"bytes"`
		Newest []string `json:"newest"`
	}
	out := []dev{}
	users, _ := os.ReadDir(root)
	for _, u := range users {
		devs, _ := os.ReadDir(filepath.Join(root, u.Name()))
		for _, d := range devs {
			one := dev{User: u.Name(), Name: d.Name()}
			type seen struct {
				name string
				mod  int64
			}
			var all []seen
			_ = filepath.WalkDir(filepath.Join(root, u.Name(), d.Name()), func(p string, e os.DirEntry, err error) error {
				if err != nil || e.IsDir() || strings.HasSuffix(p, ".part") {
					return nil
				}
				if fi, err := e.Info(); err == nil {
					one.Files++
					one.Bytes += fi.Size()
					all = append(all, seen{e.Name(), fi.ModTime().Unix()})
				}
				return nil
			})
			sort.Slice(all, func(i, j int) bool { return all[i].mod > all[j].mod })
			for i := 0; i < len(all) && i < 8; i++ {
				one.Newest = append(one.Newest, all[i].name)
			}
			out = append(out, one)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": out, "folder": "kindle-backup (in NovelCheck's data folder)"})
}

// handlePluginKindleMoved: POST /kosync/plugin/kindle/moved {"device", "moves": [{"from", "to"}]}. The plugin moved
// files on the Kindle (Tidy my Kindle's books), so the copy on the server moves the same way and the next backup
// does not send them again. Folders (a book's .sdr) move too. Nothing is overwritten or deleted.
func (s *Server) handlePluginKindleMoved(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	var body struct {
		Device string `json:"device"`
		Moves  []struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"moves"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&body); err != nil || len(body.Moves) > 5000 {
		kosyncJSON(w, http.StatusBadRequest, map[string]string{"message": "bad request"})
		return
	}
	root := s.kindleDir(u.ID, body.Device)
	moved := 0
	for _, m := range body.Moves {
		from, to := kindleRel(m.From), kindleRel(m.To)
		if from == "" || to == "" {
			continue
		}
		src, dst := filepath.Join(root, filepath.FromSlash(from)), filepath.Join(root, filepath.FromSlash(to))
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if _, err := os.Stat(dst); err == nil {
			continue // never overwrite
		}
		if os.MkdirAll(filepath.Dir(dst), 0o755) == nil && os.Rename(src, dst) == nil {
			moved++
		}
	}
	kosyncJSON(w, http.StatusOK, map[string]int{"moved": moved})
}

// kindleBookOnNightstand puts a book file from the Kindle on the Nightstand when NovelCheck already knows the book
// (by its file name, either way round: "Author - Title" or "Title - Author") and this person lets their e-reader's
// books go there (Settings). A book NovelCheck does not know is not made up: it is noted for review (Library
// cleanup → Books on your Kindle NovelCheck doesn't know), so no second copy appears by itself. A book they removed
// or hid from the Nightstand is not put back. It says whether the book was added to the Nightstand.
func (s *Server) kindleBookOnNightstand(userID int64, device, rel string, size int64) bool {
	if !kindleExts[ext(rel)] || ext(rel) == ".pdf" || strings.Contains(strings.ToLower("/"+rel), "/novelcheck/") || kindleStoreName.MatchString(path.Base(rel)) {
		return false // not a book, a PDF (manuals and papers stay in the backup only), one NovelCheck put there itself, or a name made of a store code
	}
	id := s.Store.MatchDeviceBook(store.MatchRef{Filename: path.Base(rel)})
	if id <= 0 {
		s.Store.NoteKindleUnknown(userID, device, rel, path.Base(rel), size)
		return false
	}
	if !s.Store.KindleAutoAdd(userID) {
		return false
	}
	return s.Store.EnqueueAuto(userID, id, "queued")
}
