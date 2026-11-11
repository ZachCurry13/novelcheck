package api

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/zachcurry13/novelcheck/internal/llm"
	"github.com/zachcurry13/novelcheck/internal/store"
	"github.com/zachcurry13/novelcheck/internal/version"
	kindleplugin "github.com/zachcurry13/novelcheck/kindle-plugin"
)

// Admin → Integrations: everything NovelCheck is made of or talks to, in one place. Each row says what it is,
// how it stands (from what the server already knows: no outside call is made to draw the page), and what can
// be done about it. "Check now" runs the connection checks (health.go) and pins each result to its row.

// IntegRow is one part or service.
type IntegRow struct {
	ID     string        `json:"id"`
	Name   string        `json:"name"`
	Badge  string        `json:"badge"` // ok | update | warn | off | info
	Label  string        `json:"label"` // the badge's words
	Detail string        `json:"detail"`
	Link   string        `json:"link,omitempty"`   // where it is set up
	Action string        `json:"action,omitempty"` // a button the page knows how to run (integrations.js)
	Do     string        `json:"do,omitempty"`     // the button's words
	Checks []CheckResult `json:"checks,omitempty"` // from the last "Check now"
}

// IntegGroup is a card of rows.
type IntegGroup struct {
	Title string     `json:"title"`
	Intro string     `json:"intro"`
	Rows  []IntegRow `json:"rows"`
}

type integState struct {
	mu        sync.Mutex
	checks    []CheckResult
	checkedAt time.Time
	running   bool
	cfOnce    sync.Once
	cfVersion string
}

func since(t string) string {
	at, err := time.Parse(time.RFC3339, t)
	if err != nil || at.IsZero() {
		return ""
	}
	d := time.Since(at)
	switch {
	case d < 2*time.Minute:
		return "just now"
	case d < 2*time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	}
	return fmt.Sprintf("%d d ago", int(d.Hours()/24))
}

// connectorVersion asks the bundled Cloudflare connector for its version, once.
func (s *Server) connectorVersion() string {
	s.integ.cfOnce.Do(func() {
		if s.Tunnel == nil || !s.Tunnel.Installed() {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, s.Tunnel.Binary, "--version").Output()
		if err == nil {
			for _, w := range strings.Fields(string(out)) {
				if len(w) > 3 && w[0] >= '0' && w[0] <= '9' && strings.Contains(w, ".") {
					s.integ.cfVersion = w
					break
				}
			}
		}
	})
	return s.integ.cfVersion
}

func onOff(on bool, yes, no string) (string, string) {
	if on {
		return "ok", yes
	}
	return "off", no
}

// integrations builds the page from what the server knows right now.
func (s *Server) integrations(ctx context.Context) []IntegGroup {
	set := s.Store.Setting
	row := func(id, name, badge, label, detail, link string) IntegRow {
		return IntegRow{ID: id, Name: name, Badge: badge, Label: label, Detail: detail, Link: link}
	}
	act := func(r IntegRow, action, words string) IntegRow {
		r.Action, r.Do = action, words
		return r
	}

	// NovelCheck itself.
	app := row("updates", "NovelCheck "+version.Version, "info", "Not checked", "Turn on the update check under System & Toggles to see new versions here.", "#/admin?tab=system")
	if s.Updates != nil && s.Store.SettingBool(store.KeyCheckUpdates) {
		st := s.Updates.Status(ctx, false)
		switch {
		case st.Error != "":
			app.Badge, app.Label, app.Detail = "warn", "Couldn't check", st.Error
		case st.UpdateAvailable:
			app.Badge, app.Label, app.Detail = "update", "Update available", "Version "+st.Latest+" is out. Update in TrueNAS: Apps → novelcheck → Update."
		case st.Latest != "":
			app.Badge, app.Label, app.Detail = "ok", "Latest", "You have the newest version."
		}
		if w := since(st.CheckedAt); w != "" {
			app.Detail += " Checked " + w + "."
		}
	}
	var dbBytes int64
	_ = s.Store.DB.Get(&dbBytes, `SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size()`)
	db := act(row("database", "Database (SQLite)", "ok", "In use", fmt.Sprintf("%.1f MB, kept in NovelCheck's data folder. A backup is one file.", float64(dbBytes)/(1<<20)), "#/admin?tab=system"), "db-check", "Check it")
	cf := row("tunnel", "Cloudflare connector", "off", "Not installed", "Remote access is not part of this build.", "#/admin?tab=delivery")
	if s.Tunnel != nil {
		st := s.Tunnel.Status()
		if st.Installed {
			v := s.connectorVersion()
			cf.Name = strings.TrimSpace("Cloudflare connector " + v)
			cf.Badge, cf.Label = onOff(st.State == "connected", "Connected", "Off")
			cf.Detail = "Bundled with NovelCheck; it is updated with NovelCheck."
			if st.Since != "" && cf.Badge == "ok" {
				cf.Detail += " Connected " + since(st.Since) + "."
			}
			if st.LastError != "" {
				cf.Badge, cf.Label, cf.Detail = "warn", "Problem", st.LastError
			} else if cf.Badge == "off" && s.Store.SettingBool(store.KeyTunnelEnabled) {
				cf.Badge, cf.Label = "warn", strings.ToUpper(st.State[:1])+st.State[1:]
			}
		}
	}
	plugin := row("plugin", "KOReader plugin "+kindleplugin.Version(), "info", "Bundled", s.pluginDevices(), "#/settings?s=kindle")
	groups := []IntegGroup{{"NovelCheck", "The parts NovelCheck is made of. They are updated together, when you update the app.", []IntegRow{app, db, cf, plugin}}}

	// The AIs.
	var ais []IntegRow
	for _, ai := range s.Store.AIConfigs() {
		where := "on the internet"
		if llm.IsLocal(ai.BaseURL) {
			where = "on your network"
		}
		name := map[string]string{"main": "Your AI", "backup": "Backup AI"}[ai.Name]
		if name == "" {
			name = ai.Name + " AI"
		}
		ais = append(ais, row("ai-"+ai.Name, name, "ok", "Set up", strings.Join(ai.Models, ", ")+" · "+where, "#/admin?tab=ai"))
	}
	if len(ais) == 0 {
		ais = append(ais, row("ai-main", "Your AI", "warn", "Not set up", "No AI model is chosen yet, so books can't be rated.", "#/admin?tab=ai"))
	}
	if deep, ok := s.Store.DeepAI(); ok {
		r := row("ai-deep", "Deep Scan machine", "ok", "Answering", strings.Join(deep.Models, ", "), "#/admin?tab=ai")
		if s.Deep != nil && s.Deep.MachineAway() {
			r.Badge, r.Label = "off", "Off or asleep"
			r.Detail += " · Deep Scans wait until it answers."
		}
		ais = append(ais, r)
	}
	if s.AITools != nil {
		st := s.AITools.Status()
		r := act(row("models", "Ollama models", "ok", "Up to date", "NovelCheck looks for newer versions of your models once a day.", "#/admin?tab=ai"), "models-check", "Check now")
		if n := len(st.Updates); n > 0 {
			names := make([]string, 0, n)
			for _, u := range st.Updates {
				names = append(names, u.Model)
			}
			r.Badge, r.Label, r.Detail = "update", "Update available", "Newer: "+strings.Join(names, ", ")+". Update them under AI & Scans → Machines & costs."
		} else if st.CheckErr != "" {
			r.Badge, r.Label, r.Detail = "warn", "Couldn't check", st.CheckErr
		}
		ais = append(ais, r)
	}
	groups = append(groups, IntegGroup{"Your AI", "What rates the books. \"Check now\" asks each model a tiny question to see that it answers.", ais})

	// Library servers and accounts.
	cal := act(row("calibre", "Calibre library folder", "warn", "Not found", "Choose the library folder under Delivery & Services → Calibre Library.", "#/admin?tab=delivery"), "calibre-sync", "Read now")
	if s.Syncer != nil && s.Syncer.Available() {
		cal.Badge, cal.Label, cal.Detail = "ok", "Found", "Read "+since(set(store.KeyCalibreLastSync))+"."
	}
	b, l := onOff(set(store.KeyCalibreSrvURL) != "", "Connected", "Not set up")
	srv := row("calibre-server", "Calibre Content server", b, l, "Used for every change to Calibre: tidy titles, convert, remove copies and formats.", "#/admin?tab=delivery")
	b, l = onOff(set(store.KeyCalibreWebURL) != "", "Address saved", "Not set up")
	web := row("calibre-web", "Calibre-Web", b, l, "Only a link on each book (\"Open in Calibre-Web\"); NovelCheck does not change anything there.", "#/admin?tab=delivery")
	b, l = onOff(set(store.KeyABSURL) != "", "Connected", "Not set up")
	abs := row("audiobookshelf", "Audiobookshelf", b, l, "Audiobooks, their lengths and listening.", "#/admin?tab=delivery")
	if b == "ok" {
		abs = act(abs, "abs-sync", "Read now")
		abs.Detail = strings.TrimSpace(set(store.KeyABSLastResult) + " · read " + since(set(store.KeyABSLastSync)))
		if strings.HasPrefix(strings.ToLower(set(store.KeyABSLastResult)), "failed") {
			abs.Badge, abs.Label = "warn", "Last read failed"
		}
	}
	var hcUsers, libbyUsers int
	_ = s.Store.DB.Get(&hcUsers, `SELECT COUNT(*) FROM users WHERE hardcover_token != ''`)
	_ = s.Store.DB.Get(&libbyUsers, `SELECT COUNT(*) FROM users WHERE libby_library != ''`)
	b, l = onOff(hcUsers > 0, fmt.Sprintf("%d connected", hcUsers), "Nobody connected")
	hc := row("hardcover", "Hardcover", b, l, "Each person connects their own account under Settings → Connections: shelves, ratings, goals.", "#/settings?s=connections")
	b, l = onOff(libbyUsers > 0, fmt.Sprintf("%d with a library", libbyUsers), "No library chosen")
	lib := row("libby", "Libby (public library)", b, l, "Shows whether your library has a book. Each person picks their library under Settings → Connections.", "#/settings?s=connections")
	b, l = onOff(set(store.KeySMTPHost) != "", "Set up", "Not set up")
	mail := row("smtp", "Email (Send-to-Kindle)", b, l, strings.TrimSpace(set(store.KeySMTPHost)), "#/admin?tab=delivery")
	groups = append(groups, IntegGroup{"Your library and accounts", "The servers and accounts NovelCheck reads from and writes to.", []IntegRow{cal, srv, web, abs, hc, lib, mail}})

	// Outside book data.
	ol := row("openlibrary", "Open Library", "ok", "No key needed", "Book descriptions, ISBNs and covers. Free.", "")
	b, l = onOff(set(store.KeyGoogleBooksAPIKey) != "", "Your own key", "Shared free quota")
	if b == "off" {
		b = "info"
	}
	gb := row("googlebooks", "Google Books", b, l, "Book descriptions and covers when Open Library has none.", "#/admin?tab=ai")
	b, l = onOff(set(store.KeyNYTAPIKey) != "", "Key saved", "No key")
	disc := act(row("discover", "Discover lists (best sellers, classics)", b, l, strings.TrimSpace(set(store.KeyDiscoverLastResult)+" · updated "+since(set(store.KeyDiscoverLastRefresh))), "#/admin?tab=delivery"), "discover-refresh", "Update now")
	covers := act(row("covers", "Cover finder", "ok", "Runs daily", "Looks for covers of books Calibre has none for, in Open Library and Google Books.", "#/admin?tab=cleanup"), "covers-find", "Find now")
	groups = append(groups, IntegGroup{"Book data from outside", "Free services NovelCheck asks about books. Keys stay on your server and are never shown again.", []IntegRow{ol, gb, disc, covers}})

	// Pin the last check's results to their rows.
	s.integ.mu.Lock()
	checks := append([]CheckResult{}, s.integ.checks...)
	s.integ.mu.Unlock()
	home := map[string]string{"disk": "database", "public-address": "tunnel", "ollama": "models"}
	for _, c := range checks {
		id := c.ID
		if h := home[id]; h != "" {
			id = h
		} else if strings.HasPrefix(id, "llm-backup") {
			id = "ai-backup"
		} else if strings.HasPrefix(id, "llm") {
			id = "ai-main"
		}
		for gi := range groups {
			for ri := range groups[gi].Rows {
				if groups[gi].Rows[ri].ID == id {
					groups[gi].Rows[ri].Checks = append(groups[gi].Rows[ri].Checks, c)
				}
			}
		}
	}
	return groups
}
