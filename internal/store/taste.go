package store

// The taste profile: books someone marked as want to read, don't want,
// read & liked or read & didn't like (plus their 👍/👎 on suggestions), all
// saved as suggestion votes so they steer Suggested Reads.

// TasteMark is one book someone gave an answer on.
type TasteMark struct {
	BookID int64  `db:"book_id" json:"book_id"` // 0 = not in the library
	Title  string `db:"title" json:"title"`
	Author string `db:"author" json:"author"`
	Mark   string `db:"mark" json:"mark"`     // want, liked, notwant, disliked, up (👍) or down (👎)
	Reason string `db:"reason" json:"reason"` // a 👎's "Why not?" answer
}

// TasteMarks lists someone's answers, newest first.
func (s *Store) TasteMarks(userID int64) ([]TasteMark, error) {
	out := []TasteMark{}
	err := s.DB.Select(&out, `SELECT COALESCE(b.id, 0) AS book_id, v.title, v.author,
		CASE WHEN v.reason IN ('want', 'liked', 'notwant', 'disliked') THEN v.reason WHEN v.vote = 1 THEN 'up' ELSE 'down' END AS mark,
		v.reason FROM suggestion_votes v LEFT JOIN books b ON b.norm_key = v.norm_key
		WHERE v.user_id = ? ORDER BY v.created_at DESC, v.title`, userID)
	return out, err
}
