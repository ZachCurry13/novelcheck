package api

import (
	"crypto/md5"
	"encoding/hex"
	"io/fs"
	"log"
	"net/http"
	"os"
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
	mu      sync.Mutex
	locks   map[int64]webdav.LockSystem
	running map[int64]*sync.Mutex
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
		http.Error(w, "KOReader is turned off in NovelCheck", http.StatusForbidden)
		return
	}
	name, pass, ok := r.BasicAuth()
	sum := md5.Sum([]byte(pass))
	u, good := s.Store.KosyncUser(name, hex.EncodeToString(sum[:]))
	if !ok || !good {
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
	locks, running := s.dav.forUser(u.ID)
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	(&webdav.Handler{Prefix: "/dav", FileSystem: webdav.Dir(dir), LockSystem: locks}).ServeHTTP(rec, r)
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
}
