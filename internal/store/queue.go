package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// ListQueue returns a user's reading queue: currently-reading first, then the
// "Up Next" items by position. Finished items are included when all is true.
func (s *Store) ListQueue(userID int64, all bool) ([]QueueItem, error) {
	filter := "AND q.status != 'finished'"
	if all {
		filter = ""
	}
	var items []QueueItem
	err := s.DB.Select(&items, `SELECT q.id, q.book_id, q.position, q.status, q.delivery_note,
		q.updated_at, b.title, b.author, b.classification, b.status AS book_status, b.spice_level, `+deepChangeCol+`, `+ownedCond+` AS owned
		FROM queue_items q JOIN books b ON b.id = q.book_id
		WHERE q.user_id = ? `+filter+`
		ORDER BY CASE q.status WHEN 'reading' THEN 0 WHEN 'queued' THEN 1 ELSE 2 END,
			q.position, q.id`, userID)
	return items, err
}

// Enqueue appends a book to the end of the user's queue (no-op if present).
func (s *Store) Enqueue(userID, bookID int64) error {
	_, err := s.DB.Exec(`INSERT INTO queue_items (user_id, book_id, position)
		VALUES (?, ?, COALESCE((SELECT MAX(position) + 1 FROM queue_items WHERE user_id = ?), 0))
		ON CONFLICT(user_id, book_id) DO UPDATE SET
			status = CASE WHEN queue_items.status = 'finished' THEN 'queued' ELSE queue_items.status END,
			updated_at = CURRENT_TIMESTAMP`, userID, bookID, userID)
	return err
}

func (s *Store) QueueItem(userID, itemID int64) (*QueueItem, error) {
	var it QueueItem
	err := s.DB.Get(&it, `SELECT q.id, q.book_id, q.position, q.status, q.delivery_note,
		q.updated_at, b.title, b.author, b.classification, b.status AS book_status, b.spice_level, `+deepChangeCol+`, `+ownedCond+` AS owned
		FROM queue_items q JOIN books b ON b.id = q.book_id
		WHERE q.user_id = ? AND q.id = ?`, userID, itemID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &it, err
}

func (s *Store) RemoveFromQueue(userID, itemID int64) error {
	_, err := s.DB.Exec(`DELETE FROM queue_items WHERE user_id = ? AND id = ?`, userID, itemID)
	return err
}

// ReorderQueue persists a drag-and-drop ordering. ids lists the user's queue
// item ids in their new order; each gets position = index.
func (s *Store) ReorderQueue(userID int64, ids []int64) error {
	tx, err := s.DB.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for pos, id := range ids {
		res, err := tx.Exec(`UPDATE queue_items SET position = ?, updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND id = ?`, pos, userID, id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("queue item %d: %w", id, ErrNotFound)
		}
	}
	return tx.Commit()
}

// SetQueueStatus moves an item between queued / reading / finished.
func (s *Store) SetQueueStatus(userID, itemID int64, status, note string) error {
	if status != "queued" && status != "reading" && status != "finished" {
		return fmt.Errorf("invalid status %q", status)
	}
	_, err := s.DB.Exec(`UPDATE queue_items SET status = ?, delivery_note = ?,
		updated_at = CURRENT_TIMESTAMP WHERE user_id = ? AND id = ?`, status, note, userID, itemID)
	return err
}
