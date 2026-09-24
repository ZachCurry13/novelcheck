package store

import (
	"log"
	"strings"
)

// Notification is a problem shown to admins and editors under the bell icon.
type Notification struct {
	ID        int64  `db:"id" json:"id"`
	Level     string `db:"level" json:"level"` // info | warning | error
	Source    string `db:"source" json:"source"`
	Message   string `db:"message" json:"message"`
	Link      string `db:"link" json:"link"` // in-app page that helps fix it
	Count     int    `db:"count" json:"count"`
	Read      bool   `db:"read" json:"read"`
	CreatedAt string `db:"created_at" json:"created_at"`
	UpdatedAt string `db:"updated_at" json:"updated_at"`
}

const maxNotifications = 300

// Notify records a problem. If the same unread problem already exists, its
// count and time are bumped instead of adding a duplicate. Errors are only
// logged: notifying must never break the caller.
func (s *Store) Notify(level, source, message, link string) {
	if level != "info" && level != "error" {
		level = "warning"
	}
	message = strings.TrimSpace(message)
	if len(message) > 500 {
		message = message[:500] + "…"
	}
	res, err := s.DB.Exec(`UPDATE notifications SET count = count + 1, updated_at = CURRENT_TIMESTAMP,
		level = ?, link = ? WHERE read = 0 AND source = ? AND message = ?`, level, link, source, message)
	if err == nil {
		if n, _ := res.RowsAffected(); n > 0 {
			return
		}
		_, err = s.DB.Exec(`INSERT INTO notifications (level, source, message, link) VALUES (?, ?, ?, ?)`,
			level, source, message, link)
	}
	if err != nil {
		log.Printf("notify: %v", err)
		return
	}
	_, _ = s.DB.Exec(`DELETE FROM notifications WHERE id NOT IN
		(SELECT id FROM notifications ORDER BY updated_at DESC, id DESC LIMIT ?)`, maxNotifications)
}

// NotifyRoutine records a routine event (a sync or batch finished, a book was
// sent), unless the admin turned routine events off.
func (s *Store) NotifyRoutine(source, message, link string) {
	if s.SettingBool(KeyNotifyRoutine) {
		s.Notify("info", source, message, link)
	}
}

// Resolve marks every unread notification from source as read, used when a
// problem fixes itself (e.g. remote access reconnects).
func (s *Store) Resolve(source string) {
	_, _ = s.DB.Exec(`UPDATE notifications SET read = 1 WHERE read = 0 AND source = ?`, source)
}

// Notifications lists the newest notifications, unread first.
func (s *Store) Notifications(limit int) ([]Notification, int, error) {
	var items []Notification
	if err := s.DB.Select(&items, `SELECT id, level, source, message, link, count, read, created_at, updated_at
		FROM notifications ORDER BY read, updated_at DESC, id DESC LIMIT ?`, limit); err != nil {
		return nil, 0, err
	}
	var unread int
	err := s.DB.Get(&unread, `SELECT COUNT(*) FROM notifications WHERE read = 0`)
	return items, unread, err
}

// MarkNotificationsRead marks one notification (id > 0) or all as read.
func (s *Store) MarkNotificationsRead(id int64) error {
	if id > 0 {
		_, err := s.DB.Exec(`UPDATE notifications SET read = 1 WHERE id = ?`, id)
		return err
	}
	_, err := s.DB.Exec(`UPDATE notifications SET read = 1`)
	return err
}

// ClearNotifications deletes all notifications.
func (s *Store) ClearNotifications() error {
	_, err := s.DB.Exec(`DELETE FROM notifications`)
	return err
}
