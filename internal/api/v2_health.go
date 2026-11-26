package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/zachcurry13/novelcheck/internal/hardcover"
	"github.com/zachcurry13/novelcheck/internal/store"
	kindleapp "github.com/zachcurry13/novelcheck/kindle-app"
	kindleplugin "github.com/zachcurry13/novelcheck/kindle-plugin"
)

// Health: one list that says whether NovelCheck is working, from what the server already knows (no outside call), each
// problem with what to do about it. The worst level is the page's colour: ok, warn or bad.

type forgetDevice struct {
	UserID int64  `json:"user_id"`
	Device string `json:"device"`
}

type healthItem struct {
	ID     string        `json:"id"`
	Level  string        `json:"level"` // ok | warn | bad
	Title  string        `json:"title"`
	Detail string        `json:"detail"`
	Fix    string        `json:"fix,omitempty"`    // what to do, in words
	Link   string        `json:"link,omitempty"`   // where
	Forget *forgetDevice `json:"forget,omitempty"` // a Kindle that stopped reporting can be forgotten
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
		out = append(out, healthItem{ID: id, Level: level, Title: title, Detail: detail, Fix: fix, Link: link})
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
		if d := hardcover.CoolingDown(s.Store.RatingsToken()); d > 0 {
			add("hardcover", "warn", "Hardcover asked for a rest", "Too many requests were sent; everything that uses Hardcover waits about "+ago(d)+" and then carries on by itself.", "", "")
		} else {
			add("hardcover", "ok", "Hardcover", "Connected.", "", "")
		}
		g := s.Store.GenreSources()
		switch {
		case g[""] > 100:
			add("genres", "warn", fmt.Sprintf("%d books have no genre", g[""]), fmt.Sprintf("Genres come from Calibre's tags, then Hardcover. %d have the AI's from before; Hardcover's replace them as each book is looked up.", g["ai"]), "Look them up now from the banner on Admin.", "#/admin")
		case g["ai"] > 0 || g[""] > 0:
			add("genres", "ok", "Genres", fmt.Sprintf("From Calibre %d · Hardcover %d · the AI's %d (being replaced by Hardcover's) · none %d.", g["calibre"], g["hardcover"], g["ai"], g[""]), "", "")
		}
	}
	// covers: what is still without one, and what the finder gave up on
	s.missing.mu.Lock() // the kept list only: building it asks Audiobookshelf about every book, and Health must not wait on that
	n, listed := len(s.missing.list), s.missing.ok
	s.missing.mu.Unlock()
	left := len(s.Store.LeftOut())
	if !listed {
		// not built yet: no Covers line until the page or the finder has made the list
	} else if n > 0 {
		add("covers", "warn", fmt.Sprintf("%d books have no cover", n), fmt.Sprintf("The finder looks again by itself; %d more it found nothing for are left out of the lists.", left), "Choose a cover by hand under Admin → Library → Fix titles and covers.", "#/admin?tab=cleanup")
	} else if left > 0 {
		add("covers", "ok", "Covers", fmt.Sprintf("Every book has one, except %d the finder found nothing for (left out of the lists).", left), "", "")
	}
	// Kindles: one line each. A Kindle is the KOReader plugin's report and the Kindle app's report of the same device (they are
	// linked: "Same Kindle as…"), so two reports of one Kindle are one line, with everything about it.
	type kindle struct {
		name, user string
		userID     int64
		devs       []store.DeviceSeen
		newest     int64
	}
	kindles, order, linksOf := map[string]*kindle{}, []string{}, map[int64]map[string]string{}
	for _, d := range s.Store.AllDevices() {
		if linksOf[d.UserID] == nil {
			linksOf[d.UserID] = s.Store.DeviceLinks(d.UserID)
		}
		name := d.Device
		if l := linksOf[d.UserID][d.Device]; l != "" {
			name = l
		}
		key := fmt.Sprint(d.UserID, "|", name)
		k := kindles[key]
		if k == nil {
			k = &kindle{name: name, user: d.User, userID: d.UserID}
			kindles[key], order = k, append(order, key)
		}
		k.devs = append(k.devs, d)
		k.newest = max(k.newest, d.Reported)
	}
	for _, key := range order {
		k := kindles[key]
		seen := time.Since(time.Unix(k.newest, 0))
		label := k.name + " (" + k.user + ")"
		var issues []string
		stale := seen > 14*24*time.Hour
		if stale {
			issues = append(issues, "hasn't reported for "+ago(seen))
		}
		backedUp := false
		for _, d := range k.devs {
			if strings.HasPrefix(d.Version, "app ") && versionOlder(strings.TrimPrefix(d.Version, "app "), kindleapp.Version) {
				issues = append(issues, "Kindle app "+strings.TrimPrefix(d.Version, "app ")+" is out of date (newest "+kindleapp.Version+")")
			} else if !strings.HasPrefix(d.Version, "app ") && versionOlder(d.Version, kindleplugin.Version()) {
				issues = append(issues, "KOReader plugin "+d.Version+" is out of date (newest "+kindleplugin.Version()+")")
			}
			backedUp = backedUp || s.hasKindleBackup(d.UserID, d.Device)
		}
		if !stale && !backedUp {
			issues = append(issues, "has no backup in NovelCheck")
		}
		if len(issues) == 0 {
			continue
		}
		// A Kindle that only runs an older app or plugin is a note, not a problem: it updates itself when it is opened.
		level := "info"
		if stale || !backedUp {
			level = "warn"
		}
		fix := "Open NovelCheck on it with Wi-Fi on, press Sync now, and update if it offers. Back it up from the app's Menu or KOReader's Copy my Kindle."
		it := healthItem{ID: "kindle-" + label, Level: level, Title: label, Detail: strings.Join(issues, "; "), Fix: fix, Link: "#/kindle"}
		if stale {
			it.Forget = &forgetDevice{UserID: k.userID, Device: k.devs[0].Device}
			it.Fix = "If this Kindle is gone or replaced, forget it (its backup files stay). Otherwise open NovelCheck on it with Wi-Fi on."
		}
		out = append(out, it)
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
	rank := map[string]int{"bad": 0, "warn": 1, "info": 2, "ok": 3}
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

// handleForgetDevice: POST /api/admin/devices/forget {"user_id", "device"}: a Kindle that stopped reporting (14 days) is forgotten.
func (s *Server) handleForgetDevice(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UserID int64  `json:"user_id"`
		Device string `json:"device"`
	}
	if !readJSON(w, r, &body, 2<<10) || body.UserID <= 0 || body.Device == "" {
		writeErr(w, http.StatusBadRequest, "which Kindle?")
		return
	}
	for _, d := range s.Store.AllDevices() {
		if d.UserID == body.UserID && d.Device == body.Device {
			if time.Since(time.Unix(d.Reported, 0)) < 14*24*time.Hour {
				writeErr(w, http.StatusConflict, "that Kindle reported in the last two weeks; it isn't gone")
				return
			}
			if err := s.Store.ForgetDevice(body.UserID, body.Device); err != nil {
				writeStoreErr(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
			return
		}
	}
	writeErr(w, http.StatusNotFound, "NovelCheck doesn't know that Kindle")
}
