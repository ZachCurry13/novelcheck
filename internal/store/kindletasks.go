package store

// KindleTask is something to do on a Kindle at its next sync.
type KindleTask struct {
	ID        int64  `db:"id" json:"id"`
	Device    string `db:"device" json:"device"`
	Kind      string `db:"kind" json:"kind"`
	Args      string `db:"args" json:"args"`
	Status    string `db:"status" json:"status"`
	Note      string `db:"note" json:"note"`
	CreatedAt string `db:"created_at" json:"created_at"`
	DoneAt    string `db:"done_at" json:"done_at"`
}

const kindleTaskCols = `id, device, kind, args, status, note, created_at, COALESCE(done_at, '') AS done_at`

// KindleTaskKinds are the only things a task can ask for.
var KindleTaskKinds = map[string]bool{"backup": true, "tidy": true, "crashlogs": true, "delete_books": true, "download_books": true, "move_books": true,
	"report": true, "highlights": true, "stats": true, "download_nightstand": true, "tidy_names": true, "selftest": true, "update_plugin": true,
	"stop": true, "resume_auto": true, "set_auto": true, "set_home": true, "remove_store": true}

// KindleAutoSwitches are the plugin's automatic switches a task may turn on or off.
var KindleAutoSwitches = map[string]bool{"auto_report": true, "auto_download": true, "auto_stats": true, "auto_highlights": true, "auto_finish": true,
	"auto_kindle": true, "auto_backup": true, "auto_crashlogs": true}

func (s *Store) CreateKindleTask(userID int64, device, kind, args string) int64 {
	res, err := s.DB.Exec(`INSERT INTO kindle_tasks (user_id, device, kind, args) VALUES (?, ?, ?, ?)`, userID, device, kind, args)
	if err != nil {
		return 0
	}
	id, _ := res.LastInsertId()
	return id
}

// WaitingKindleTasks are the tasks for this Kindle (its own and the ones for whichever checks in), oldest first.
func (s *Store) WaitingKindleTasks(userID int64, device string) []KindleTask {
	out := []KindleTask{}
	_ = s.DB.Select(&out, `SELECT `+kindleTaskCols+` FROM kindle_tasks WHERE user_id = ? AND status = 'waiting' AND (device = '' OR device = ?) ORDER BY id LIMIT 50`, userID, device)
	return out
}

// KindleTasksOf lists a person's newest tasks.
func (s *Store) KindleTasksOf(userID int64, limit int) []KindleTask {
	out := []KindleTask{}
	_ = s.DB.Select(&out, `SELECT `+kindleTaskCols+` FROM kindle_tasks WHERE user_id = ? ORDER BY id DESC LIMIT ?`, userID, limit)
	return out
}

// SetKindleTaskResult records what the Kindle did (done, failed or skipped) for a waiting task.
func (s *Store) SetKindleTaskResult(userID, id int64, status, note string) bool {
	if status != "done" && status != "failed" && status != "skipped" {
		return false
	}
	if len(note) > 300 {
		note = note[:300]
	}
	res, err := s.DB.Exec(`UPDATE kindle_tasks SET status = ?, note = ?, done_at = CURRENT_TIMESTAMP WHERE user_id = ? AND id = ? AND status = 'waiting'`, status, note, userID, id)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n > 0
}

// CancelKindleTask takes back a task that has not been done yet.
func (s *Store) CancelKindleTask(userID, id int64) bool {
	res, err := s.DB.Exec(`UPDATE kindle_tasks SET status = 'canceled', done_at = CURRENT_TIMESTAMP WHERE user_id = ? AND id = ? AND status = 'waiting'`, userID, id)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n > 0
}

// DeviceFile is one book file a Kindle last reported, with the library book it matched (0 = none).
type DeviceFile struct {
	Filename string  `db:"filename"`
	Title    string  `db:"title"`
	Author   string  `db:"author"`
	Format   string  `db:"format"`
	Size     int64   `db:"size"`
	Percent  float64 `db:"percent"`
	BookID   int64   `db:"book_id"`
	MD5      string  `db:"md5"` // the Kindle app: "asin:<id>" for an Amazon store book
}

// DeviceFiles are the book files a Kindle last reported.
func (s *Store) DeviceFiles(userID int64, device string) []DeviceFile {
	out := []DeviceFile{}
	_ = s.DB.Select(&out, `SELECT filename, title, author, format, size, percent, COALESCE(book_id, 0) AS book_id, md5 FROM device_books
		WHERE user_id = ? AND device = ? ORDER BY title COLLATE NOCASE LIMIT 3000`, userID, device)
	return out
}
