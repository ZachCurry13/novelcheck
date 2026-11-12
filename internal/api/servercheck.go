package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/audiobookshelf"
	"github.com/zachcurry13/novelcheck/internal/store"
)

// "Check my servers" (Settings → Developer): harmless read-only checks of the Calibre Content server, Audiobookshelf
// and Calibre-Web, so what they really do (versions, library names, which routes exist, what the token may do) is
// known instead of guessed. The answers are shown, and added to the server report (so bug reports carry them).

var (
	checkMu   sync.Mutex
	checkText string
	checkAt   time.Time
)

type checkOut struct{ b strings.Builder }

func (o *checkOut) line(f string, a ...any) { fmt.Fprintf(&o.b, f+"\n", a...) }

func (s *Server) runServerChecks() string {
	var o checkOut
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	s.checkCalibre(ctx, &o)
	s.checkABS(ctx, &o)
	s.checkCalibreWeb(ctx, &o)
	txt := strings.TrimRight(o.b.String(), "\n")
	checkMu.Lock()
	checkText, checkAt = txt, time.Now()
	checkMu.Unlock()
	return txt
}

func (s *Server) checkCalibre(ctx context.Context, o *checkOut) {
	o.line("== Calibre Content server ==")
	if s.Store.Setting(store.KeyCalibreSrvURL) == "" {
		o.line("not set up")
		return
	}
	c := s.calibreClient()
	libs, def, err := c.Libraries(ctx)
	if err != nil {
		o.line("can't read its libraries: %v", err)
		return
	}
	var names []string
	for id, name := range libs {
		names = append(names, id+" = “"+name+"”")
	}
	o.line("libraries it serves: %s · its default: %s · NovelCheck uses: %s", strings.Join(names, "; "), def, orNone(c.Library))
	o.line("NovelCheck reads the folder: %s", orNone(s.Store.Setting("calibre_library_path")))
	want := map[int][]string{}
	for i, ref := range s.Store.CalibreBookRefs() {
		if i >= 30 {
			break
		}
		want[ref.CalibreID] = []string{ref.Title}
	}
	if lib, report := s.matchServerLibrary(ctx, want); len(report) > 0 {
		o.line("which library holds NovelCheck's books (a sample of %d): best match %s", len(want), orNone(lib))
		for _, r := range report {
			o.line("  %s", r)
		}
	}
	if ids := s.Store.CalibreBookRefs(); len(ids) > 0 {
		if d, err := c.Details(ctx, []int{ids[0].CalibreID}); err != nil {
			o.line("reading a book's details: failed (%v)", err)
		} else {
			o.line("reading a book's details: works (%d of 1 found)", len(d))
		}
	}
	for _, p := range []struct{ what, path string }{
		{"adding a book (upload)", "/cdb/add-book/0/y/probe.epub"},
		{"setting a cover", "/cdb/set-cover/1"},
		{"converting a format", "/conversion/start/1"},
		{"editing fields (set-fields)", "/cdb/set-fields/1"},
	} {
		o.line("route for %s: %s", p.what, c.Probe(ctx, p.path))
	}
}

func (s *Server) checkABS(ctx context.Context, o *checkOut) {
	o.line("\n== Audiobookshelf ==")
	c, ok := audiobookshelf.FromSettings(s.Store)
	if !ok {
		o.line("not set up")
		return
	}
	if resp, err := (&http.Client{Timeout: 10 * time.Second}).Get(c.Base + "/status"); err == nil {
		resp.Body.Close()
		o.line("reachable at %s (status %d)", c.Base, resp.StatusCode)
	} else {
		o.line("can't reach %s: %v", c.Base, err)
		return
	}
	var auth struct {
		User struct {
			Type        string         `json:"type"`
			Permissions map[string]any `json:"permissions"`
		} `json:"user"`
		ServerSettings struct {
			Version string `json:"version"`
		} `json:"serverSettings"`
	}
	// Audiobookshelf answers "who is this token" only to a POST (a GET gives 404 even with a good token).
	if err := c.JSON(ctx, http.MethodPost, "/api/authorize", nil, &auth); err != nil {
		o.line("the token: %v", err)
	} else {
		o.line("server version %s · token's account type %s · may upload: %v · may download: %v", orNone(auth.ServerSettings.Version),
			auth.User.Type, auth.User.Permissions["upload"], auth.User.Permissions["download"])
	}
	libs, err := c.Libraries(ctx)
	if err != nil {
		o.line("libraries: %v", err)
		return
	}
	for _, l := range libs {
		var one struct {
			Folders []struct {
				ID       string `json:"id"`
				FullPath string `json:"fullPath"`
			} `json:"folders"`
		}
		_ = c.JSON(ctx, http.MethodGet, "/api/libraries/"+l.ID, nil, &one)
		var f []string
		for _, x := range one.Folders {
			f = append(f, x.ID+" "+x.FullPath)
		}
		o.line("library “%s” (%s, id %s) folders: %s", l.Name, l.MediaType, l.ID, orNone(strings.Join(f, "; ")))
	}
}

func (s *Server) checkCalibreWeb(ctx context.Context, o *checkOut) {
	o.line("\n== Calibre-Web ==")
	base := strings.TrimRight(s.Store.Setting(store.KeyCalibreWebURL), "/")
	if base == "" {
		o.line("no address saved (Admin → Calibre Library)")
		return
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/login", nil)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		o.line("can't reach %s: %v", base, err)
		return
	}
	resp.Body.Close()
	o.line("reachable at %s (login page status %d)", base, resp.StatusCode)
}

// serverChecksSection adds the last check to the server report.
func (s *Server) serverChecksSection(line func(string, ...any)) {
	checkMu.Lock()
	txt, at := checkText, checkAt
	checkMu.Unlock()
	line("\n== Server checks (Settings → Developer → Check my servers) ==")
	if txt == "" {
		line("(not run since the server started)")
		return
	}
	line("run %s UTC", at.UTC().Format("2006-01-02 15:04:05"))
	line("%s", txt)
}

// POST /api/admin/server-checks runs the checks; GET returns the last answer.
func (s *Server) handleServerChecks(w http.ResponseWriter, r *http.Request) {
	txt := s.runServerChecks()
	writeJSON(w, http.StatusOK, map[string]string{"text": txt})
}
