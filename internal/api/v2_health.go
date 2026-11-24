package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/store"
	kindleapp "github.com/zachcurry13/novelcheck/kindle-app"
	kindleplugin "github.com/zachcurry13/novelcheck/kindle-plugin"
)

// Health: one list that says whether NovelCheck is working, from what the server already knows (no outside call), each
// problem with what to do about it. The worst level is the page's colour: ok, warn or bad.

type healthItem struct {
	ID     string `json:"id"`
	Level  string `json:"level"` // ok | warn | bad
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`  // what to do, in words
	Link   string `json:"link,omitempty"` // where
}

// ago says how long ago a time was, in words ("3 h", "2 days").
func ago(d time.Duration) string {
	switch {
	case d < 2*time.Hour:
		return fmt.Sprintf("%d min", max(1, int(d.Minutes())))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d h", int(d.Hours()))
	}
	return fmt.Sprintf("%d days", int(d.Hours()/24))
}

func (s *Server) healthItems() []healthItem {
	var out []healthItem
	add := func(id, level, title, detail, fix, link string) {
		out = append(out, healthItem{id, level, title, detail, fix, link})
	}
	// Calibre is read often; a long silence means the connection or the schedule broke
	if s.Store.Setting(store.KeyCalibreSrvURL) != "" || s.Syncer != nil && s.Syncer.Available() {
		if at, err := time.Parse(time.RFC3339, s.Store.Setting(store.KeyCalibreLastSync)); err != nil {
			add("calibre", "warn", "Calibre hasn't been read yet", "NovelCheck has not read your Calibre library since it started.", "Press Check again on the duplicates page, or check the Calibre folder under Admin.", "#/admin?tab=delivery")
		} else if d := time.Since(at); d > 30*time.Hour {
			add("calibre", "warn", "Calibre was last read "+ago(d)+" ago", "New books won't show up until it is read again.", "Press Check again on the duplicates page; if it keeps failing, see Admin → Integrations.", "#/system")
		} else {
			add("calibre", "ok", "Calibre", "Last read "+ago(d)+" ago.", "", "")
		}
	}
	// the AI: without it nothing is rated
	if len(s.Store.AIConfigs()) == 0 {
		add("ai", "bad", "No AI is set up", "Books can't be rated.", "Choose an AI under Admin → AI & Scans.", "#/admin?tab=ai")
	} else if n := s.Store.WaitingToRate(); n > 100 {
		add("ai", "warn", fmt.Sprintf("%d books are waiting to be rated", n), "The rating can't keep up, or it is switched off for now.", "Check the automatic rating switch and hours under Admin → AI & Scans.", "#/admin?tab=ai")
	} else {
		add("ai", "ok", "Rating", fmt.Sprintf("%d waiting.", n), "", "")
	}
	// Hardcover feeds ratings, genres, covers and ISBNs
	if s.Store.RatingsToken() == "" {
		add("hardcover", "warn", "Hardcover isn't connected", "Hardcover ratings, genres, ISBNs and many covers are off.", "Someone connects their Hardcover account under Settings → Connections.", "#/settings")
	} else {
		add("hardcover", "ok", "Hardcover", "Connected.", "", "")
	}
	// Kindles
	for _, d := range s.Store.AllDevices() {
		seen := time.Since(time.Unix(d.Reported, 0))
		name := d.Device + " (" + d.User + ")"
		switch {
		case seen > 14*24*time.Hour:
			add("kindle-"+name, "warn", name+" hasn't reported for "+ago(seen), "Its books and reading aren't being tracked.", "Open NovelCheck on it with Wi-Fi on and press Sync now.", "#/kindle")
		case strings.HasPrefix(d.Version, "app ") && versionOlder(strings.TrimPrefix(d.Version, "app "), kindleapp.Version):
			add("kindle-"+name, "warn", name+": Kindle app "+strings.TrimPrefix(d.Version, "app ")+" is out of date", "Newest is "+kindleapp.Version+".", "Open the app with Wi-Fi on: it updates itself.", "#/kindle")
		case !strings.HasPrefix(d.Version, "app ") && versionOlder(d.Version, kindleplugin.Version()):
			add("kindle-"+name, "warn", name+": KOReader plugin "+d.Version+" is out of date", "Newest is "+kindleplugin.Version()+".", "Open KOReader on it with Wi-Fi on: the plugin offers to update.", "#/kindle")
		}
	}
	// the same failure over and over in the background (the Deep Scan machine's program crashing, a service that refuses)
	day := time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02 15:04:05")
	shownErr := 0
	for _, g := range groupErrors(s.Store.RecentErrors(400), 12) {
		if g.Count >= 5 && g.At >= day && shownErr < 3 && (g.Method == "background" || g.Status >= 500) { // a request the app refused (4xx) is not a fault
			shownErr++
			what := strings.TrimSpace(g.Detail)
			if len(what) > 120 {
				what = what[:120] + "…"
			}
			where := g.Path
			if g.Method == "background" {
				where = "in the background (" + g.Path + ")"
			}
			add("repeat-"+g.Code, "warn", fmt.Sprintf("The same error %d times: %s", g.Count, where), what, "Send me the bug link: a repeating error usually has one cause.", "")
		}
	}
	// Calibre's Content server lacking books the library has (it is what makes removing copies fail)
	if items, _, err := s.Store.Notifications(60); err == nil {
		for _, n := range items {
			if n.Source == "calibre-behind" && !n.Read {
				add("calibre-behind", "warn", "Calibre's Content server is behind", n.Message, "Restart the Calibre app (or its Content server) so it sees the newer books.", "")
				break
			}
		}
	}
	// a Kindle with no copy of its books on the server
	for _, d := range s.Store.AllDevices() {
		if time.Since(time.Unix(d.Reported, 0)) < 14*24*time.Hour && !s.hasKindleBackup(d.UserID, d.Device) {
			add("nobackup-"+d.Device, "warn", d.Device+" ("+d.User+") has no backup in NovelCheck", "If the Kindle is lost or reset, its books can't be put back.", "In KOReader: NovelCheck → Copy my Kindle. In the Kindle app: Menu → Back up my books.", "#/kindle")
		}
	}
	// disk
	if s.SysInfo != nil {
		n := s.SysInfo.Snapshot()
		gb := float64(n.DiskFree) / (1 << 30)
		switch {
		case n.DiskTotal > 0 && gb < 1:
			add("disk", "bad", fmt.Sprintf("Only %.1f GB free", gb), "NovelCheck's data folder is almost full.", "Free up space on the dataset NovelCheck's data is on.", "#/system")
		case n.DiskTotal > 0 && gb < 5:
			add("disk", "warn", fmt.Sprintf("%.1f GB free", gb), "The data folder is getting full.", "Free up some space soon.", "#/system")
		}
	}
	// errors in the last day
	if n := s.Store.ErrorsSince(time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02 15:04:05")); n > 25 {
		add("errors", "warn", fmt.Sprintf("%d errors in the last day", n), "Something fails over and over.", "Open the bug link (Report a problem) and send it to me.", "")
	}
	// the safety copy of the database
	if at, ok := s.lastBackup(); !ok {
		add("backup", "warn", "No backup of NovelCheck's data yet", "Ratings, Nightstands and settings live in one database file.", "The first backup is made a few minutes after start, then daily.", "")
	} else if d := time.Since(at); d > 50*time.Hour {
		add("backup", "warn", "The last backup is "+ago(d)+" old", "Daily backups have stopped.", "Check the data folder's free space and permissions.", "")
	} else {
		add("backup", "ok", "Backup", "Last copy "+ago(d)+" ago, in the data folder's backups.", "", "")
	}
	rank := map[string]int{"bad": 0, "warn": 1, "ok": 2}
	sort.SliceStable(out, func(i, j int) bool { return rank[out[i].Level] < rank[out[j].Level] })
	return out
}

// healthLevel is the worst level on the list.
func healthLevel(items []healthItem) string {
	level := "ok"
	for _, it := range items {
		if it.Level == "bad" {
			return "bad"
		}
		if it.Level == "warn" {
			level = "warn"
		}
	}
	return level
}

// handleHealth: GET /api/admin/health.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	items := s.healthItems()
	writeJSON(w, http.StatusOK, map[string]any{"level": healthLevel(items), "items": items})
}

// lastBackup is when the newest database backup in the data folder was made.
func (s *Server) lastBackup() (time.Time, bool) {
	files, _ := filepath.Glob(filepath.Join(s.Cfg.DataDir, "backups", "novelcheck-*.db"))
	var newest time.Time
	for _, f := range files {
		if info, err := os.Stat(f); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest, !newest.IsZero()
}

// versionOlder says whether version a ("1.2.0") is older than b.
func versionOlder(a, b string) bool {
	parts := func(v string) [3]int {
		var out [3]int
		for i, p := range strings.SplitN(strings.TrimSpace(v), ".", 3) {
			fmt.Sscanf(p, "%d", &out[i])
		}
		return out
	}
	x, y := parts(a), parts(b)
	for i := range x {
		if x[i] != y[i] {
			return x[i] < y[i]
		}
	}
	return false
}

// hasKindleBackup says whether the server holds at least one file of this e-reader's backup.
func (s *Server) hasKindleBackup(userID int64, device string) bool {
	found := false
	_ = filepath.WalkDir(s.kindleDir(userID, device), func(p string, e os.DirEntry, err error) error {
		if err == nil && !e.IsDir() && !strings.HasSuffix(p, ".part") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
