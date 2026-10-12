package api

import (
	"crypto/md5"
	"encoding/hex"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/net/webdav"

	"github.com/zachcurry13/novelcheck/internal/kosync"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// KOReader's reading statistics (Reading statistics → Cloud sync) go
// through a WebDAV folder: each reader has a private one at /dav/, signed in
// with their NovelCheck name and sync code (HTTP Basic), kept under the data
// folder. When KOReader uploads statistics.sqlite3, NovelCheck reads it: the
// books opened on the reader's devices, how far, and for how long.

// davMaxBytes is the most one person's folder may hold.
const davMaxBytes = 100 << 20

// statsFile is the name KOReader gives its statistics.
const statsFile = "statistics.sqlite3"

func init() {
	for _, m := range []string{"PROPFIND", "PROPPATCH", "MKCOL", "COPY", "MOVE", "LOCK", "UNLOCK"} {
		chi.RegisterMethod(m) // before any route: chi only routes methods it knows
	}
}

// davState keeps each reader's WebDAV locks and serializes their imports.
type davState struct {
	mu       sync.Mutex
	locks    map[int64]webdav.LockSystem
	running  map[int64]*sync.Mutex
	attempts map[int64][]DavAttempt
}

func (d *davState) forUser(id int64) (webdav.LockSystem, *sync.Mutex) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.locks == nil {
		d.locks, d.running = map[int64]webdav.LockSystem{}, map[int64]*sync.Mutex{}
	}
	if d.locks[id] == nil {
		d.locks[id], d.running[id] = webdav.NewMemLS(), &sync.Mutex{}
	}
	return d.locks[id], d.running[id]
}

func (s *Server) davDir(userID int64) string {
	return filepath.Join(s.Cfg.DataDir, "koreader", strconv.FormatInt(userID, 10))
}

// statusRecorder notes the status a handler answered with.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (s *Server) handleDAV(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !s.Store.SettingBool(store.KeyModuleKOReader) {
		s.dav.note(0, DavAttempt{Method: r.Method, Path: r.URL.Path, Status: http.StatusForbidden, Note: "the KOReader module is turned off"})
		http.Error(w, "KOReader is turned off in NovelCheck", http.StatusForbidden)
		return
	}
	if s.codeGuesses.blocked(remoteHost(r)) {
		http.Error(w, "Too many wrong sync codes; try again in a few minutes", http.StatusTooManyRequests)
		return
	}
	name, pass, ok := r.BasicAuth()
	sum := md5.Sum([]byte(pass))
	u, good := s.Store.KosyncUser(name, hex.EncodeToString(sum[:]))
	if !ok || !good {
		if ok && name != "" {
			s.codeGuesses.fail(remoteHost(r))
		}
		s.noteDAVRefusal(r, name, ok)
		w.Header().Set("WWW-Authenticate", `Basic realm="NovelCheck"`)
		http.Error(w, "Use your NovelCheck name and the sync code from Profile → KOReader setup", http.StatusUnauthorized)
		return
	}
	dir := s.davDir(u.ID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		http.Error(w, "can't make your folder", http.StatusInternalServerError)
		return
	}
	if r.Method == http.MethodPut {
		if used := dirSize(dir); r.ContentLength > davMaxBytes || used+max(r.ContentLength, 0) > davMaxBytes {
			http.Error(w, "your NovelCheck folder is full (100 MB)", http.StatusInsufficientStorage)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, davMaxBytes)
	}
	// Someone typing "dav" as the start folder means their own folder, not one inside it.
	if rest := strings.TrimPrefix(r.URL.Path, "/dav"); strings.HasPrefix(rest, "/dav/") || rest == "/dav" {
		for strings.HasPrefix(rest, "/dav/") || rest == "/dav" {
			rest = strings.TrimPrefix(rest, "/dav")
		}
		r.URL.Path, r.URL.RawPath = "/dav"+rest, ""
	}
	if r.Method == "PROPFIND" { // KOReader asks for its "start folder": make it, whatever it's called
		if rel := strings.Trim(path.Clean("/"+strings.TrimPrefix(r.URL.Path, "/dav")), "/"); rel != "" && len(rel) < 200 {
			_ = os.MkdirAll(filepath.Join(dir, filepath.FromSlash(rel)), 0o700)
		}
	}
	locks, running := s.dav.forUser(u.ID)
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	(&webdav.Handler{Prefix: "/dav", FileSystem: webdav.Dir(dir), LockSystem: locks}).ServeHTTP(rec, r)
	if r.Method != "OPTIONS" {
		s.dav.note(u.ID, DavAttempt{Method: r.Method, Path: r.URL.Path, Status: rec.status, Name: name, Note: map[bool]string{true: "worked", false: "NovelCheck answered but not with success"}[rec.status < 300]})
	}
	switch r.Method {
	case http.MethodPut, "MOVE", "COPY":
		if rec.status < 300 {
			go func() {
				running.Lock()
				defer running.Unlock()
				s.importKOStats(u.ID)
			}()
		}
	}
}

// dirSize is how much a folder holds.
func dirSize(dir string) int64 {
	var n int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}

// statsPath is the newest statistics file in someone's folder ("" if none).
func (s *Server) statsPath(userID int64) (string, time.Time) {
	var best string
	var at time.Time
	_ = filepath.WalkDir(s.davDir(userID), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(d.Name(), statsFile) {
			return nil
		}
		if info, err := d.Info(); err == nil && info.ModTime().After(at) {
			best, at = p, info.ModTime()
		}
		return nil
	})
	return best, at
}

// importKOStats reads someone's statistics file into their KOReader books.
func (s *Server) importKOStats(userID int64) {
	path, _ := s.statsPath(userID)
	if path == "" {
		return
	}
	books, err := kosync.ReadStats(path)
	if err != nil {
		log.Printf("koreader statistics for user %d: %v", userID, err)
		return
	}
	list := make([]store.KOReaderBook, 0, len(books))
	for _, b := range books {
		list = append(list, store.KOReaderBook{Title: b.Title, Authors: b.Authors, MD5: b.MD5, Pages: b.Pages,
			Percent: b.Percent, ReadSeconds: b.ReadSeconds, LastOpen: b.LastOpen.Unix()})
	}
	if _, err := s.Store.SaveKOReaderBooks(userID, list); err != nil {
		log.Printf("koreader statistics for user %d: %v", userID, err)
	}
	s.resolveUnmatched(userID)
	if hours, err := kosync.ReadHours(path); err != nil {
		log.Printf("koreader reading hours for user %d: %v", userID, err)
	} else {
		rows := make([]store.ReadingHour, 0, len(hours))
		for _, h := range hours {
			rows = append(rows, store.ReadingHour{Title: h.Title, Authors: h.Authors, MD5: h.MD5, Hour: h.Hour, Seconds: h.Seconds, Pages: h.Pages})
		}
		if err := s.Store.SaveReadingHours(userID, rows); err != nil {
			log.Printf("koreader reading hours for user %d: %v", userID, err)
		}
	}
}

// noteDAVRefusal records why a sign-in failed: no name sent, a name nobody
// has, or a sync code that doesn't match.
func (s *Server) noteDAVRefusal(r *http.Request, name string, sent bool) {
	a := DavAttempt{Method: r.Method, Path: r.URL.Path, Status: http.StatusUnauthorized, Name: name}
	if !sent || name == "" { // only KOReader's own unsigned requests are worth a line (not the internet's)
		if ua := strings.ToLower(r.UserAgent()); strings.Contains(ua, "luasocket") || strings.Contains(ua, "koreader") {
			a.Note = "KOReader's request came without a username or sync code (open the WebDAV account in KOReader and fill both in)"
			s.dav.note(0, a)
		}
		return
	}
	owner := int64(0)
	if u, err := s.Store.UserByName(strings.TrimSpace(name)); err == nil {
		owner = u.ID
		a.Note = "the sync code didn't match (it must be the current one from Profile → KOReader setup, not your password)"
	} else {
		a.Note = "no NovelCheck user has this exact name (use your NovelCheck username)"
	}
	s.dav.note(owner, a)
}
