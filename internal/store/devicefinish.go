package store

// FinishOnDevice moves books that KOReader marked finished from "reading" or "queued" to
// "finished" on the person's Nightstand. It only touches books already there (a device holds
// years of reading), and not a book put back on the Nightstand after the day it was finished
// (to read again). books maps a book to the day KOReader finished it ("" when unknown).
func (s *Store) FinishOnDevice(userID int64, books map[int64]string) {
	for id, day := range books {
		_, _ = s.DB.Exec(`UPDATE queue_items SET status = 'finished', updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND book_id = ? AND status IN ('queued', 'reading') AND (? = '' OR date(updated_at) <= ?)`,
			userID, id, day, day)
	}
}
