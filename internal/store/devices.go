package store

import (
	"strings"
	"time"
)

// What's on a reader's e-readers, from the KOReader plugin's reports.

// DeviceBook is one file on a device.
type DeviceBook struct {
	MD5      string `json:"md5"`
	Filename string `json:"filename"`
	Title    string `json:"title"`
	Format   string `json:"format"`
	Size     int64  `json:"size"`
	Author   string `json:"author"` // from the book's own metadata, when the plugin could read it
	// KOReader's own "finished" mark for the book, and the day it was made ("2026-11-09").
	Finished   bool    `json:"finished"`
	FinishedOn string  `json:"finished_on"`
	Percent    float64 `json:"percent"`   // how far KOReader (or, from the Kindle app, Amazon's reader) is in it (0 to 1)
	OpenedAt   int64   `json:"opened_at"` // the Kindle app: when Amazon's reader last opened it (Unix seconds)
}

// Device is an e-reader that has reported.
type Device struct {
	Name       string `db:"device" json:"name"`
	Books      int    `db:"books" json:"books"`
	InLibrary  int    `db:"in_library" json:"in_library"`
	ReportedAt int64  `db:"reported_at" json:"reported_at"`
	Plugin     string `db:"plugin_version" json:"plugin_version"`
}

// SaveDeviceInfo notes which plugin version a device runs.
func (s *Store) SaveDeviceInfo(userID int64, device, version string) {
	now := time.Now().Unix()
	_, _ = s.DB.Exec(`INSERT INTO device_info (user_id, device, plugin_version, reported_at, version_since) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id, device) DO UPDATE SET
			version_since = CASE WHEN plugin_version != excluded.plugin_version THEN excluded.reported_at ELSE version_since END,
			plugin_version = excluded.plugin_version, reported_at = excluded.reported_at`, userID, device, version, now, now)
}

// SaveDeviceBooks replaces a device's list with a new report, matching each
// file to the library by its fingerprint (known from downloads) or its title.
// It returns how many matched.
func (s *Store) SaveDeviceBooks(userID int64, device string, books []DeviceBook) (int, error) {
	ids := make([]int64, len(books))
	ignored := make([]bool, len(books))
	for i, b := range books {
		ids[i], ignored[i] = s.findForDevice(userID, b)
	}
	tx, err := s.DB.Beginx()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM device_books WHERE user_id = ? AND device = ?`, userID, device); err != nil {
		return 0, err
	}
	now, matched := time.Now().Unix(), 0
	for i, b := range books {
		var bookID any
		if ids[i] > 0 {
			bookID = ids[i]
			matched++
		}
		if _, err := tx.Exec(`INSERT OR REPLACE INTO device_books (user_id, device, md5, filename, title, author, format, size, book_id, ignored, reported_at, percent, opened_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, userID, device, b.MD5, b.Filename, b.Title, b.Author, b.Format, b.Size, bookID, ignored[i], now, b.Percent, b.OpenedAt); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	done := map[int64]string{} // library books KOReader says are finished
	for i, b := range books {
		if ids[i] > 0 && !ignored[i] && b.Finished {
			done[ids[i]] = b.FinishedOn
		}
	}
	s.FinishOnDevice(userID, done)
	return matched, nil
}

// Devices lists someone's e-readers, most recently reported first.
func (s *Store) Devices(userID int64) []Device {
	out := []Device{}
	_ = s.DB.Select(&out, `SELECT b.device, COUNT(*) AS books, SUM(b.book_id IS NOT NULL) AS in_library, MAX(b.reported_at) AS reported_at,
		COALESCE((SELECT i.plugin_version FROM device_info i WHERE i.user_id = b.user_id AND i.device = b.device), '') AS plugin_version
		FROM device_books b WHERE b.user_id = ? AND b.ignored = 0 GROUP BY b.device ORDER BY MAX(b.reported_at) DESC`, userID)
	return out
}

// DeviceBookIDs is the library books on a device (the ones its report matched).
func (s *Store) DeviceBookIDs(userID int64, device string) []int64 {
	var ids []int64
	_ = s.DB.Select(&ids, `SELECT DISTINCT book_id FROM device_books WHERE user_id = ? AND device = ? AND book_id IS NOT NULL ORDER BY book_id`, userID, device)
	return ids
}

// DeviceName is the proper title and author of a library book on a device.
type DeviceName struct {
	MD5      string `db:"md5" json:"md5"`
	Filename string `db:"filename" json:"filename"`
	Title    string `db:"title" json:"title"`
	Author   string `db:"author" json:"author"`
}

// DeviceNames are the device's files that are library books (not just looked
// up), with the library's title and author, for tidying the names on the device.
func (s *Store) DeviceNames(userID int64, device string) []DeviceName {
	out := []DeviceName{}
	_ = s.DB.Select(&out, `SELECT d.md5, d.filename, b.title, b.author FROM device_books d JOIN books b ON b.id = d.book_id
		WHERE d.user_id = ? AND (? = '' OR d.device = ?) AND d.ignored = 0 AND `+ownedCond+` ORDER BY d.filename`, userID, device, device)
	return out
}

// DeviceLinks maps each of a person's linked e-reader names to the Kindle they are part of.
func (s *Store) DeviceLinks(userID int64) map[string]string {
	var rows []struct {
		Device string `db:"device"`
		Kindle string `db:"kindle"`
	}
	_ = s.DB.Select(&rows, `SELECT device, kindle FROM device_links WHERE user_id = ?`, userID)
	out := map[string]string{}
	for _, r := range rows {
		out[r.Device] = r.Kindle
	}
	return out
}

// LinkDevice says an e-reader report is part of the Kindle called kindle ("" unlinks it).
func (s *Store) LinkDevice(userID int64, device, kindle string) error {
	if strings.TrimSpace(kindle) == "" {
		_, err := s.DB.Exec(`DELETE FROM device_links WHERE user_id = ? AND device = ?`, userID, device)
		return err
	}
	_, err := s.DB.Exec(`INSERT INTO device_links (user_id, device, kindle) VALUES (?, ?, ?)
		ON CONFLICT(user_id, device) DO UPDATE SET kindle = excluded.kindle`, userID, device, strings.TrimSpace(kindle))
	return err
}

// AppDeviceSuffix is what the Kindle app adds to the Kindle's name, so its reports never replace the plugin's.
const AppDeviceSuffix = " (Amazon reader)"

// AutoLink links the Kindle app's report and the plugin's when their names say it: "Zach Paperwhite" (plugin) and
// "Zach Paperwhite (Amazon reader)" (app, named when it was downloaded).
func (s *Store) AutoLink(userID int64, device string) {
	s.linkTheOnlyPair(userID)
	base := strings.TrimSuffix(device, AppDeviceSuffix)
	if base == "" {
		return
	}
	var n int
	for _, d := range []string{base, base + AppDeviceSuffix} {
		if d != device {
			_ = s.DB.Get(&n, `SELECT COUNT(*) FROM device_info WHERE user_id = ? AND device = ?`, userID, d)
			if n == 0 {
				return
			}
		}
	}
	_ = s.LinkDevice(userID, base, base)
	_ = s.LinkDevice(userID, base+AppDeviceSuffix, base)
}

// linkTheOnlyPair joins an app and a plugin report that no name ties together ("Kindle (Amazon reader)", from an app
// installed without a name, and "Zach Paperwhite") when the person has exactly one of each that is not linked yet: one
// person with one app and one plugin has one Kindle. With more than one of either it is a guess, so nothing is joined.
func (s *Store) linkTheOnlyPair(userID int64) {
	var rows []struct {
		Device  string `db:"device"`
		Version string `db:"plugin_version"`
	}
	_ = s.DB.Select(&rows, `SELECT device, plugin_version FROM device_info WHERE user_id = ?`, userID)
	links := s.DeviceLinks(userID)
	var apps, plugins []string
	for _, r := range rows {
		if links[r.Device] != "" {
			continue
		}
		if strings.HasPrefix(r.Version, "app ") {
			apps = append(apps, r.Device)
		} else {
			plugins = append(plugins, r.Device)
		}
	}
	if len(apps) == 1 && len(plugins) == 1 {
		_ = s.LinkDevice(userID, plugins[0], plugins[0])
		_ = s.LinkDevice(userID, apps[0], plugins[0])
	}
}

// HasDevice says whether the person has an e-reader report with this name (the plugin's or the app's).
func (s *Store) HasDevice(userID int64, device string) bool {
	var n int
	_ = s.DB.Get(&n, `SELECT (SELECT COUNT(*) FROM device_info WHERE user_id = ? AND device = ?) + (SELECT COUNT(*) FROM device_books WHERE user_id = ? AND device = ?)`, userID, device, userID, device)
	return n > 0
}
