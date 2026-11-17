package api

import (
	"archive/zip"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
)

// The KOReader plugin's "copy my Kindle" run: it lists every book file on the device (any folder, not only the ones
// KOReader opened) and sends the ones this server doesn't have yet. Every file is kept as it is (protected ones too:
// nothing is unlocked or changed) under /data/kindle-backup/<person>/<device>/. A file that is not locked is also
// added to Calibre unless the library already has that title.

const maxKindleFile = 1 << 30

// kindleStoreName is a file named by Amazon's code instead of the title (B07XYZ1234_EBOK.azw, Title_B07XYZ1234.azw3).
var kindleStoreName = regexp.MustCompile(`(?i)(^|[_ -])B[0-9A-Z]{9}([_. -]|$)|_EBOK|_PDOC|_SAMPLE`)

const maxKindleList = 20000

var kindleExts = map[string]bool{".epub": true, ".pdf": true, ".mobi": true, ".azw": true, ".azw3": true, ".azw1": true,
	".kfx": true, ".kepub": true, ".fb2": true, ".cbz": true, ".prc": true}

// kindleSkip is a path that looks like a book but is not one: the plugin's own logs and KOReader's folders.
func kindleSkip(rel string) bool {
	l := strings.ToLower(rel)
	for _, part := range strings.Split(l, "/") {
		// "novelcheck" is the folder the plugin downloads Nightstand books into: they came from this server
		if strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".sdr") || part == "koreader" || part == "system" || part == "novelcheck.koplugin" || part == "novelcheck" {
			return true
		}
	}
	base := path.Base(l)
	stem := strings.TrimSuffix(base, path.Ext(base))
	for _, w := range []string{"novelcheck", "bug", "debug", "crash"} {
		if strings.HasPrefix(stem, w) {
			return true
		}
	}
	if stem == "log" || stem == "logs" {
		return true
	}
	return !kindleExts[path.Ext(l)]
}

// backupSkip is a path a whole-Kindle backup leaves out: hidden and system folders, KOReader's caches and its
// update downloads, the Trash, and half-finished files. Everything else (the app, plugins, settings, books) is kept.
func backupSkip(rel string) bool {
	l := strings.ToLower(rel)
	for _, part := range strings.Split(l, "/") {
		if strings.HasPrefix(part, ".") || part == "system" || part == "trash" || part == "lost+found" {
			return true
		}
	}
	return strings.HasPrefix(l, "koreader/cache/") || strings.HasPrefix(l, "koreader/ota/") || strings.HasPrefix(l, "koreader/crash.log") ||
		strings.HasSuffix(l, ".part") || strings.HasSuffix(l, ".new")
}

// kindleRel makes a safe relative path from what the device sent ("" when it is not usable).
func kindleRel(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimPrefix(p, "/mnt/us/")
	p = path.Clean("/" + p)
	p = strings.TrimPrefix(p, "/")
	if p == "" || p == "." || strings.Contains(p, "..") || len(p) > 400 {
		return ""
	}
	return p
}

func (s *Server) kindleDir(userID int64, device string) string {
	d := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == 0 || r == ':' {
			return '_'
		}
		return r
	}, strings.TrimSpace(device))
	if d == "" {
		d = "kindle"
	}
	return filepath.Join(s.Cfg.DataDir, "kindle-backup", strconv.FormatInt(userID, 10), d)
}

// handlePluginKindleCheck: POST /kosync/plugin/kindle/check {"device", "files": [{"path", "size"}]} answers the
// paths to send: books that are not on the server yet (same path and size) and not something that only looks like one.
func (s *Server) handlePluginKindleCheck(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	var body struct {
		Device string `json:"device"`
		Mode   string `json:"mode"` // "backup": every file of the Kindle, not only the books
		Files  []struct {
			Path  string `json:"path"`
			Size  int64  `json:"size"`
			Mtime int64  `json:"mtime"`
		} `json:"files"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&body); err != nil || len(body.Files) > maxKindleList {
		kosyncJSON(w, http.StatusBadRequest, map[string]string{"message": "bad request"})
		return
	}
	root := s.kindleDir(u.ID, body.Device)
	send, have, skipped := []string{}, 0, 0
	for _, f := range body.Files {
		rel := kindleRel(f.Path)
		backup := body.Mode == "backup"
		if rel == "" || f.Size > maxKindleFile || (backup && (backupSkip(rel) || f.Size < 0)) || (!backup && (kindleSkip(rel) || f.Size < 2048)) {
			skipped++
			continue
		}
		// the same size and, for a backup, the same change time: settings change without changing size
		if fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err == nil && fi.Size() == f.Size && (!backup || f.Mtime == 0 || fi.ModTime().Unix() == f.Mtime) {
			have++
			continue
		}
		send = append(send, f.Path)
	}
	kosyncJSON(w, http.StatusOK, map[string]any{"send": send, "have": have, "skipped": skipped})
}

// handlePluginKindleFile: PUT /kosync/plugin/kindle/file?device=&path= with the file as the body.
func (s *Server) handlePluginKindleFile(w http.ResponseWriter, r *http.Request) {
	u := s.kosyncUser(w, r)
	if u == nil {
		return
	}
	rel := kindleRel(r.URL.Query().Get("path"))
	backup := r.URL.Query().Get("mode") == "backup"
	if rel == "" || (backup && backupSkip(rel)) || (!backup && kindleSkip(rel)) {
		kosyncJSON(w, http.StatusBadRequest, map[string]string{"message": "not a book file"})
		return
	}
	dest := filepath.Join(s.kindleDir(u.ID, r.URL.Query().Get("device")), filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		kosyncJSON(w, http.StatusInternalServerError, map[string]string{"message": "can't make the folder"})
		return
	}
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		kosyncJSON(w, http.StatusInternalServerError, map[string]string{"message": "can't save the file"})
		return
	}
	n, err := io.Copy(f, http.MaxBytesReader(w, r.Body, maxKindleFile))
	_ = f.Close()
	if err != nil || n < 0 || (!backup && n < 2048) {
		_ = os.Remove(tmp)
		kosyncJSON(w, http.StatusBadRequest, map[string]string{"message": "the upload didn't finish"})
		return
	}
	if err := os.Rename(tmp, dest); err != nil {
		kosyncJSON(w, http.StatusInternalServerError, map[string]string{"message": "can't save the file"})
		return
	}
	if m, _ := strconv.ParseInt(r.URL.Query().Get("mtime"), 10, 64); backup && m > 0 {
		_ = os.Chtimes(dest, time.Unix(m, 0), time.Unix(m, 0)) // so the next check sees the file as unchanged
	}
	locked := kindleLocked(dest)
	added := false
	if !locked && !backup {
		added = s.addKindleToCalibre(r, dest, path.Base(rel))
	}
	onShelf := s.kindleBookOnNightstand(u.ID, rel)
	kosyncJSON(w, http.StatusOK, map[string]any{"stored": true, "locked": locked, "added_to_calibre": added, "on_nightstand": onShelf})
}

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

// kindleBookOnNightstand makes a book file from the Kindle a book NovelCheck knows (a book it already has is reused;
// else it joins the Looked up list under its tidy title and author, like "Add to NovelCheck" on an e-reader entry)
// and, when this person lets their e-reader's books go onto their Nightstand (Settings), puts it there. A book they
// removed or hid from the Nightstand is not put back. It says whether the book was added to the Nightstand.
func (s *Server) kindleBookOnNightstand(userID int64, rel string) bool {
	if !kindleExts[ext(rel)] || ext(rel) == ".pdf" || strings.Contains(strings.ToLower("/"+rel), "/novelcheck/") || kindleStoreName.MatchString(path.Base(rel)) {
		return false // not a book, a PDF (manuals and papers stay in the backup only), one NovelCheck put there itself, or a name made of a store code
	}
	id, err := s.Store.AddDeviceBook(store.MatchRef{Filename: path.Base(rel)})
	if err != nil || id <= 0 {
		return false
	}
	if !s.Store.KindleAutoAdd(userID) {
		return false
	}
	return s.Store.EnqueueAuto(userID, id, "queued")
}
