package store

import "time"

// What's on a reader's e-readers, from the KOReader plugin's reports.

// DeviceBook is one file on a device.
type DeviceBook struct {
	MD5      string `json:"md5"`
	Filename string `json:"filename"`
	Title    string `json:"title"`
	Format   string `json:"format"`
	Size     int64  `json:"size"`
	Author   string `json:"author"` // from the book's own metadata, when the plugin could read it
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
	_, _ = s.DB.Exec(`INSERT INTO device_info (user_id, device, plugin_version, reported_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id, device) DO UPDATE SET plugin_version = excluded.plugin_version, reported_at = excluded.reported_at`, userID, device, version, time.Now().Unix())
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
		if _, err := tx.Exec(`INSERT OR REPLACE INTO device_books (user_id, device, md5, filename, title, author, format, size, book_id, ignored, reported_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, userID, device, b.MD5, b.Filename, b.Title, b.Author, b.Format, b.Size, bookID, ignored[i], now); err != nil {
			return 0, err
		}
	}
	return matched, tx.Commit()
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
