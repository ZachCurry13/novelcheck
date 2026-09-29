package store

// Events over time: an event is archived when its end passes, or after 30
// days when it has no end and isn't pinned. Archived events keep their books
// (and ratings) and can be restored; deleting one is still possible.

import (
	"database/sql"
	"strings"
	"time"
)

// EventTime turns a time sent by a browser into how events store it (RFC
// 3339, UTC, to the second), or "" for none. ok is false for a bad time.
func EventTime(v string) (string, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", true
	}
	t, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		return "", false
	}
	return t.UTC().Format(time.RFC3339), true
}

// nowSQL is the current time as events store it.
const nowSQL = `strftime('%Y-%m-%dT%H:%M:%SZ', 'now')`

// ended is true for an event whose end has passed.
const ended = `(e.ends_at != '' AND julianday(e.ends_at) <= julianday('now'))`

// ArchiveEnded archives the events whose end passed, and those without an
// end that are over 30 days old and not pinned. It says how many.
func (s *Store) ArchiveEnded() (int, error) {
	res, err := s.DB.Exec(`UPDATE events AS e SET archived_at = ` + nowSQL + ` WHERE e.archived_at = '' AND (` + ended + `
		OR (e.ends_at = '' AND e.pinned = 0 AND e.created_at < datetime('now', '-` + eventDaysSQL + ` days')))`)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// ArchiveEvent archives an event now, or restores it. A restored event whose
// end passed loses that end, and one that would go straight back to the
// archive (over 30 days old, no end) is pinned.
func (s *Store) ArchiveEvent(id int64, archived bool) error {
	q := `UPDATE events AS e SET archived_at = CASE WHEN e.archived_at = '' THEN ` + nowSQL + ` ELSE e.archived_at END WHERE e.id = ?`
	if !archived {
		q = `UPDATE events AS e SET archived_at = '',
			pinned = CASE WHEN (e.ends_at = '' OR ` + ended + `) AND e.created_at < datetime('now', '-` + eventDaysSQL + ` days') THEN 1 ELSE e.pinned END,
			ends_at = CASE WHEN ` + ended + ` THEN '' ELSE e.ends_at END
			WHERE e.id = ?`
	}
	return s.mustChange(s.DB.Exec(q, id))
}

// UpdateEvent renames an event and sets when it ends (endsAt as EventTime
// returns it; "" for no end).
func (s *Store) UpdateEvent(id int64, name, endsAt string) error {
	return s.mustChange(s.DB.Exec(`UPDATE events SET name = ?, ends_at = ? WHERE id = ?`, strings.TrimSpace(name), endsAt, id))
}

// PinEvent keeps (or no longer keeps) an event without an end out of the
// archive after its 30 days, and first in the list.
func (s *Store) PinEvent(id int64, pinned bool) error {
	return s.mustChange(s.DB.Exec(`UPDATE events SET pinned = ? WHERE id = ?`, pinned, id))
}

// DeleteEvent removes an event; its books stay only where the family has them.
func (s *Store) DeleteEvent(id int64) error {
	if err := s.mustChange(s.DB.Exec(`DELETE FROM events WHERE id = ?`, id)); err != nil {
		return err
	}
	return s.dropUnlistedEventCopies()
}

// dropUnlistedEventCopies takes books off the Events catalog once no event
// lists them (a claimed book keeps its library).
func (s *Store) dropUnlistedEventCopies() error {
	_, err := s.DB.Exec(`DELETE FROM catalog_books WHERE catalog_id = (SELECT id FROM catalogs WHERE name = ?)
		AND book_id NOT IN (SELECT book_id FROM event_books)`, EventsCatalog)
	return err
}

// mustChange turns "no row changed" into ErrNotFound.
func (s *Store) mustChange(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
