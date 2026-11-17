package db

import "github.com/jmoiron/sqlx"

// A book's rating is kept when the book is deleted and given back when the same book comes again. A book row is
// deleted once no library, list or event holds it (an old event, a book removed from Calibre); the same title
// and author later (the next Stuff Your Kindle day, Calibre adding it back) would be rated all over again. Two
// triggers do it, so every way a book is deleted or added is covered, now and later:
//
//   - before an analyzed book is deleted, what was decided about it is saved in rating_memory (with its content
//     items and custom filters), under its norm_key;
//   - after a new book is added that is still waiting to be rated, and the memory has its norm_key, the rating
//     comes back and the book is analyzed from the start.
//
// The rules version, filters version and content version come back as they were, so a rating made under older
// rules is still offered for re-rating like any other. The changed-in-Calibre time is not kept: a book added
// back is not "changed since it was rated".
func ratingMemory(d *sqlx.DB) error {
	const cols = `classification, nudity, solo_acts, heavy_innuendo, playful_fantasy, dark_occult, demonic_presence, summary_verdict,
		confidence, premise, approved, approved_by, age_level, age_set_by, spice_level, spice_reason, rules_version, flags_version,
		content_version, content_amounts, genres, kind, genre_source, hc_rating, hc_ratings, hc_rated_at, hc_warnings, hc_moods, hc_id,
		big_rules, analysis_model, analyzed_at`
	// Both triggers do nothing when the admin turned remembering off (Library cleanup → Remembered ratings and covers).
	const on = `COALESCE((SELECT value FROM settings WHERE key = 'remember_ratings'), 'true') != 'false'`
	remember := `CREATE TRIGGER books_remember_rating BEFORE DELETE ON books WHEN OLD.status = 'analyzed' AND ` + on + `
BEGIN
  INSERT OR REPLACE INTO rating_memory (norm_key, last_id, ` + cols + `, saved_at)
  VALUES (OLD.norm_key, OLD.id, OLD.classification, OLD.nudity, OLD.solo_acts, OLD.heavy_innuendo, OLD.playful_fantasy, OLD.dark_occult,
    OLD.demonic_presence, OLD.summary_verdict, OLD.confidence, OLD.premise, OLD.approved, OLD.approved_by, OLD.age_level, OLD.age_set_by,
    OLD.spice_level, OLD.spice_reason, OLD.rules_version, OLD.flags_version, OLD.content_version, OLD.content_amounts, OLD.genres, OLD.kind,
    OLD.genre_source, OLD.hc_rating, OLD.hc_ratings, OLD.hc_rated_at, OLD.hc_warnings, OLD.hc_moods, OLD.hc_id, OLD.big_rules,
    OLD.analysis_model, OLD.analyzed_at, CAST(strftime('%s', 'now') AS INTEGER));
  DELETE FROM content_memory WHERE norm_key = OLD.norm_key;
  INSERT OR IGNORE INTO content_memory (norm_key, key, source) SELECT OLD.norm_key, key, source FROM book_content WHERE book_id = OLD.id;
  DELETE FROM flag_memory WHERE norm_key = OLD.norm_key;
  INSERT OR IGNORE INTO flag_memory (norm_key, flag_id) SELECT OLD.norm_key, flag_id FROM book_flags WHERE book_id = OLD.id;
END`
	recall := `CREATE TRIGGER books_recall_rating AFTER INSERT ON books
WHEN NEW.status = 'pending' AND ` + on + ` AND EXISTS (SELECT 1 FROM rating_memory m WHERE m.norm_key = NEW.norm_key)
BEGIN
  UPDATE books SET status = 'analyzed',
    classification = m.classification, nudity = m.nudity, solo_acts = m.solo_acts, heavy_innuendo = m.heavy_innuendo,
    playful_fantasy = m.playful_fantasy, dark_occult = m.dark_occult, demonic_presence = m.demonic_presence,
    summary_verdict = m.summary_verdict, confidence = m.confidence, premise = m.premise, approved = m.approved,
    approved_by = m.approved_by, age_level = m.age_level, age_set_by = m.age_set_by, spice_level = m.spice_level,
    spice_reason = m.spice_reason, rules_version = m.rules_version, flags_version = m.flags_version,
    content_version = m.content_version, content_amounts = m.content_amounts, genres = m.genres, kind = m.kind,
    genre_source = m.genre_source, hc_rating = m.hc_rating, hc_ratings = m.hc_ratings, hc_rated_at = m.hc_rated_at,
    hc_warnings = m.hc_warnings, hc_moods = m.hc_moods, hc_id = m.hc_id, big_rules = m.big_rules,
    analysis_model = m.analysis_model, analyzed_at = m.analyzed_at
  FROM rating_memory m WHERE m.norm_key = NEW.norm_key AND books.id = NEW.id;
  INSERT OR IGNORE INTO book_content (book_id, key, source) SELECT NEW.id, key, source FROM content_memory WHERE norm_key = NEW.norm_key;
  INSERT OR IGNORE INTO book_flags (book_id, flag_id) SELECT NEW.id, flag_id FROM flag_memory
    WHERE norm_key = NEW.norm_key AND flag_id IN (SELECT id FROM custom_flags);
END`
	for _, stmt := range []string{`DROP TRIGGER IF EXISTS books_remember_rating`, remember, `DROP TRIGGER IF EXISTS books_recall_rating`, recall} {
		if _, err := d.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
