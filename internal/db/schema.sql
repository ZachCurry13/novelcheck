-- NovelCheck local cache schema. Every statement is idempotent.

CREATE TABLE IF NOT EXISTS users (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    username         TEXT NOT NULL UNIQUE COLLATE NOCASE,
    password_hash    TEXT NOT NULL,
    role             TEXT NOT NULL DEFAULT 'restricted' CHECK (role IN ('admin', 'editor', 'restricted')),
    hide_open_door   INTEGER NOT NULL DEFAULT 0,
    hide_nudity      INTEGER NOT NULL DEFAULT 0,
    hide_solo_acts   INTEGER NOT NULL DEFAULT 0,
    hide_innuendo    INTEGER NOT NULL DEFAULT 0,
    hide_dark_occult INTEGER NOT NULL DEFAULT 0,
    hide_lgbtq       INTEGER NOT NULL DEFAULT 0,   -- retired in v1.19: now the user_hidden_content rule "lgbtq"
    hide_unrated     INTEGER NOT NULL DEFAULT 0,
    delivery_method  TEXT NOT NULL DEFAULT 'none' CHECK (delivery_method IN ('none', 'email', 'koreader')),
    kindle_email     TEXT NOT NULL DEFAULT '',
    guide_seen       INTEGER NOT NULL DEFAULT 0,
    age_level        INTEGER NOT NULL DEFAULT 0,   -- kid accounts: 1 young kids .. 5 adults; 0 = not set
    max_spice        INTEGER NOT NULL DEFAULT -1,  -- kid accounts: hide books above this many peppers (0-5); -1 = no limit
    only_collections INTEGER NOT NULL DEFAULT 0,   -- kid accounts: see only books in their collections (user_collections)
    ai_features      INTEGER NOT NULL DEFAULT 0,   -- kid accounts: a parent allowed AI features (AI suggestions, Deep Scan requests)
    start_page       TEXT NOT NULL DEFAULT '',     -- the page NovelCheck opens on ('' = Check a book for parents, Library for kids)
    theme            TEXT NOT NULL DEFAULT '',     -- '' = match the device, 'dark', 'light'
    font             TEXT NOT NULL DEFAULT '',     -- '' or 'dyslexic'
    motion           TEXT NOT NULL DEFAULT '',     -- '' = match the device, 'reduce'
    kosync_code      TEXT NOT NULL DEFAULT '',     -- KOReader progress sync code ('' = not set up)
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
    token_hash TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at DATETIME NOT NULL,
    remember   INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS catalogs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL UNIQUE COLLATE NOCASE,
    source     TEXT NOT NULL DEFAULT 'custom' CHECK (source IN ('calibre', 'drive', 'custom')),
    owner_id   INTEGER REFERENCES users(id) ON DELETE SET NULL,  -- who imported it; NULL = the family's (e.g. Calibre)
    private    INTEGER NOT NULL DEFAULT 0,                        -- 1 = only the owner and admins see its books
    physical   INTEGER NOT NULL DEFAULT 0,                        -- 1 = paper books on a shelf (no files)
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- A book is one logical title; catalog_books records where copies live.
CREATE TABLE IF NOT EXISTS books (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    norm_key         TEXT NOT NULL UNIQUE,
    title            TEXT NOT NULL,
    author           TEXT NOT NULL DEFAULT '',
    isbn             TEXT NOT NULL DEFAULT '',
    description      TEXT NOT NULL DEFAULT '',
    blurb            TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending', 'queued', 'processing', 'analyzed', 'error')),
    classification   TEXT,
    nudity           INTEGER NOT NULL DEFAULT 0,
    solo_acts        INTEGER NOT NULL DEFAULT 0,
    heavy_innuendo   INTEGER NOT NULL DEFAULT 0,
    playful_fantasy  INTEGER NOT NULL DEFAULT 0,
    dark_occult      INTEGER NOT NULL DEFAULT 0,
    demonic_presence INTEGER NOT NULL DEFAULT 0,
    lgbtq_content    INTEGER NOT NULL DEFAULT 0,   -- retired in v1.19: now the book_content item "lgbtq"
    summary_verdict  TEXT NOT NULL DEFAULT '',
    confidence       TEXT NOT NULL DEFAULT '',      -- the AI's certainty (high, medium, low)
    premise          TEXT NOT NULL DEFAULT '',     -- 1-2 spoiler-free sentences on what the book is about (the AI, from the blurb)
    approved         INTEGER NOT NULL DEFAULT 0,   -- parent marked "OK": bypasses filters
    approved_by      TEXT NOT NULL DEFAULT '',
    age_level        INTEGER NOT NULL DEFAULT 0,   -- parent-set age group, 1 young kids .. 5 adults; 0 = not set
    spice_level      INTEGER,                      -- 0-5 peppers; NULL = rated before the pepper scale (or not rated)
    spice_reason     TEXT NOT NULL DEFAULT '',     -- short "why this many peppers", e.g. "Heavy innuendo, on-page foreplay"
    rules_version    INTEGER NOT NULL DEFAULT 0,   -- store.RulesVersion the rating was made under
    flags_version    INTEGER NOT NULL DEFAULT 0,   -- custom filters version the rating checked
    content_version  INTEGER NOT NULL DEFAULT 0,   -- content.Version the book was checked for content items; 0 = not yet
    content_amounts  TEXT NOT NULL DEFAULT '',     -- Deep Scan amount per group, e.g. "violence:3,gore:1" (1 a little .. 3 a lot)
    rated_modified   TEXT NOT NULL DEFAULT '',     -- the Calibre change time the rating saw (delta scanning)
    series           TEXT NOT NULL DEFAULT '',     -- series name from Calibre, or taken from the title
    series_index     REAL NOT NULL DEFAULT 0,      -- number in the series; 0 = none
    title_fix        TEXT NOT NULL DEFAULT '',     -- Calibre's title when NovelCheck tidied it ("01 - Dune"); '' = already tidy
    tags             TEXT NOT NULL DEFAULT '',     -- Calibre's tags, comma-separated
    genres           TEXT NOT NULL DEFAULT '',     -- categories (package genres) as ",fantasy,romance,"
    kind             TEXT NOT NULL DEFAULT '',     -- fiction, nonfiction or '' (unknown)
    genre_source     TEXT NOT NULL DEFAULT '',     -- calibre (from tags) or ai; '' = none yet
    age_set_by       TEXT NOT NULL DEFAULT '',
    analysis_model   TEXT NOT NULL DEFAULT '',
    analysis_error   TEXT NOT NULL DEFAULT '',
    analyzed_at      DATETIME,
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_books_status ON books(status);

CREATE TABLE IF NOT EXISTS catalog_books (
    catalog_id  INTEGER NOT NULL REFERENCES catalogs(id) ON DELETE CASCADE,
    book_id     INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    path        TEXT NOT NULL DEFAULT '',
    format      TEXT NOT NULL DEFAULT '',
    external_id TEXT NOT NULL DEFAULT '',
    modified    TEXT NOT NULL DEFAULT '',  -- Calibre's last change to the entry or file (UTC), for delta scanning
    added_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (catalog_id, book_id, path)
);
CREATE INDEX IF NOT EXISTS idx_catalog_books_book ON catalog_books(book_id);

CREATE TABLE IF NOT EXISTS queue_items (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    book_id       INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    position      INTEGER NOT NULL DEFAULT 0,
    status        TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'reading', 'finished')),
    delivery_note TEXT NOT NULL DEFAULT '',
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, book_id)
);

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS token_usage (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    at                DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    book_id           INTEGER,
    model             TEXT NOT NULL,
    prompt_tokens     INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    cost              REAL,                          -- USD at the time (tokens and electricity); NULL on older rows
    seconds           REAL NOT NULL DEFAULT 0        -- how long the call took
);
CREATE INDEX IF NOT EXISTS idx_token_usage_at ON token_usage(at);

-- Admin notifications: problems NovelCheck noticed on its own. Repeats of the
-- same unread problem bump count instead of adding rows.
CREATE TABLE IF NOT EXISTS notifications (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    level      TEXT NOT NULL DEFAULT 'warning' CHECK (level IN ('info', 'warning', 'error')),
    source     TEXT NOT NULL,
    message    TEXT NOT NULL,
    link       TEXT NOT NULL DEFAULT '',
    count      INTEGER NOT NULL DEFAULT 1,
    read       INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_notifications_unread ON notifications(read, updated_at);

-- Parents' notes on books (e.g. after reading them).
CREATE TABLE IF NOT EXISTS book_notes (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id    INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body       TEXT NOT NULL,
    visibility TEXT NOT NULL DEFAULT 'everyone' CHECK (visibility IN ('everyone', 'parents')),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_book_notes_book ON book_notes(book_id);

-- The family's own AI filters (e.g. "Heavy swearing"): the AI checks every
-- book for each one, and the Library gets a Hide checkbox per filter.
CREATE TABLE IF NOT EXISTS custom_flags (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    key         TEXT NOT NULL UNIQUE,      -- JSON key the AI answers with, e.g. "swearing"
    label       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',  -- what the AI should look for
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS book_flags (
    book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    flag_id INTEGER NOT NULL REFERENCES custom_flags(id) ON DELETE CASCADE,
    PRIMARY KEY (book_id, flag_id)
);

-- Detailed content items a book contains (package content) and who said so:
-- the quick AI rating, a Deep Scan, or a parent.
CREATE TABLE IF NOT EXISTS book_content (
    book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    key     TEXT NOT NULL,
    source  TEXT NOT NULL DEFAULT 'ai' CHECK (source IN ('ai', 'deep', 'parent')),
    PRIMARY KEY (book_id, key)
);
CREATE INDEX IF NOT EXISTS idx_book_content_key ON book_content(key);

-- A user's content hide rules: an item key, "g:<group>" or "flag:<custom key>".
CREATE TABLE IF NOT EXISTS user_hidden_content (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key     TEXT NOT NULL,
    PRIMARY KEY (user_id, key)
);

-- Stuff Your Kindle events: an event's list of books (often free for a
-- day). An event is archived when it ends (ends_at, UTC), or else after 30
-- days unless pinned. asin/link point at Amazon when known.
CREATE TABLE IF NOT EXISTS events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    source_url  TEXT NOT NULL DEFAULT '',
    created_by  TEXT NOT NULL DEFAULT '',
    pinned      INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ends_at     TEXT NOT NULL DEFAULT '',
    archived_at TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS event_books (
    event_id INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    book_id  INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    asin     TEXT NOT NULL DEFAULT '',
    link     TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (event_id, book_id)
);

-- Collections: shelves across libraries, made by a parent or filled by the AI
-- from a theme. "idea" = the AI's weekly proposal, waiting for a parent.
-- season = the seasonal shelf (internal/seasons) the collection stands for.
CREATE TABLE IF NOT EXISTS collections (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    icon        TEXT NOT NULL DEFAULT '📚',
    description TEXT NOT NULL DEFAULT '',
    kind        TEXT NOT NULL DEFAULT 'manual' CHECK (kind IN ('manual', 'ai', 'idea')),
    theme       TEXT NOT NULL DEFAULT '',
    season      TEXT NOT NULL DEFAULT '',
    created_by  TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS collection_books (
    collection_id INTEGER NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    book_id       INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    reason        TEXT NOT NULL DEFAULT '',  -- why the AI picked it
    added_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (collection_id, book_id)
);
CREATE INDEX IF NOT EXISTS idx_collection_books_book ON collection_books(book_id);
-- The collections a kid with only_collections may see.
CREATE TABLE IF NOT EXISTS user_collections (
    user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    collection_id INTEGER NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, collection_id)
);

-- Deep reads: the AI reads a book's whole EPUB in parts. Admins start them;
-- anyone else requests one for an admin to approve. notes is a JSON list of
-- what each part contained.
CREATE TABLE IF NOT EXISTS deep_reads (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    position     INTEGER NOT NULL DEFAULT 0,  -- order in the queue (an admin can drag it)
    book_id      INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    status       TEXT NOT NULL DEFAULT 'requested'
                 CHECK (status IN ('requested', 'queued', 'reading', 'done', 'error', 'declined', 'cancelled')),
    requested_by TEXT NOT NULL DEFAULT '',
    reason       TEXT NOT NULL DEFAULT '',
    approved_by  TEXT NOT NULL DEFAULT '',
    words        INTEGER NOT NULL DEFAULT 0,
    parts_total  INTEGER NOT NULL DEFAULT 0,
    parts_done   INTEGER NOT NULL DEFAULT 0,
    est_tokens   INTEGER NOT NULL DEFAULT 0,
    model        TEXT NOT NULL DEFAULT '',
    notes        TEXT NOT NULL DEFAULT '',
    error        TEXT NOT NULL DEFAULT '',
    source       TEXT NOT NULL DEFAULT 'admin', -- admin | request | batch | auto (a chosen user's Up Next)
    prev_level   INTEGER,                       -- peppers before the scan (the blurb rating); NULL = unrated
    new_level    INTEGER,                       -- peppers the full text earned
    checks       INTEGER NOT NULL DEFAULT 1,    -- version of Deep Scan's checks it was made with (store.DeepChecks)
    held         INTEGER NOT NULL DEFAULT 0,    -- 1 = a big jump waiting for an admin to accept
    proposed_level INTEGER,                     -- the level a held scan suggests
    proposal     TEXT NOT NULL DEFAULT '',      -- JSON: the rating a held scan would save
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_deep_reads_open ON deep_reads(book_id) WHERE status IN ('requested', 'queued', 'reading');

-- The family wishlist: books someone would like to get (usually found with
-- Check a book). Parents approve them ("to get") and they're marked acquired
-- once they show up in a library.
CREATE TABLE IF NOT EXISTS wishlist (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id    INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    user_id    INTEGER REFERENCES users(id) ON DELETE SET NULL,
    username   TEXT NOT NULL DEFAULT '',
    note       TEXT NOT NULL DEFAULT '',
    status     TEXT NOT NULL DEFAULT 'wanted' CHECK (status IN ('wanted', 'approved', 'acquired', 'declined')),
    decided_by TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_wishlist_open ON wishlist(book_id, user_id) WHERE status IN ('wanted', 'approved');

-- Private KOReader (OPDS) feed addresses: /opds/<token> lists the person's
-- Up Next with download links. KOReader can't sign in, so the token is the key.
CREATE TABLE IF NOT EXISTS opds_tokens (
    user_id    INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    token      TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Phones and browsers that turned on push notifications. scope: "all"
-- (problems and everyday events) or "problems"; kids only get their own.
CREATE TABLE IF NOT EXISTS push_subscriptions (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint   TEXT NOT NULL UNIQUE,
    p256dh     TEXT NOT NULL,
    auth       TEXT NOT NULL,
    scope      TEXT NOT NULL DEFAULT 'all' CHECK (scope IN ('all', 'problems')),
    device     TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Requests (by any user) to delete a book; an admin reviews them. Title and
-- author are copied so the history survives the book being deleted.
CREATE TABLE IF NOT EXISTS delete_requests (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id    INTEGER REFERENCES books(id) ON DELETE SET NULL,
    title      TEXT NOT NULL,
    author     TEXT NOT NULL DEFAULT '',
    user_id    INTEGER REFERENCES users(id) ON DELETE SET NULL,
    username   TEXT NOT NULL DEFAULT '',
    reason     TEXT NOT NULL DEFAULT '',
    status     TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'deleted', 'dismissed', 'done')),
    decided_by TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    decided_at DATETIME
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_delete_requests_pending ON delete_requests(book_id, user_id) WHERE status = 'pending';

-- Suggested Reads: each person's 👍/👎 on a suggestion. norm_key is the
-- book's NormKey, so a vote on a book outside the library still counts if
-- the book is added later.
CREATE TABLE IF NOT EXISTS suggestion_votes (
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    norm_key   TEXT NOT NULL,
    title      TEXT NOT NULL,
    author     TEXT NOT NULL DEFAULT '',
    vote       INTEGER NOT NULL CHECK (vote IN (-1, 1)),
    reason     TEXT NOT NULL DEFAULT '',     -- why a 👎: read, story, author, series, spicy; '' = not given
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, norm_key)
);

-- The AI's latest suggestions per person (made at most once a day).
CREATE TABLE IF NOT EXISTS suggestion_sets (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    made_at TEXT NOT NULL,                -- UTC "2006-01-02 15:04:05"
    mode    TEXT NOT NULL DEFAULT '',     -- the suggest_mode setting it was made under
    data    TEXT NOT NULL DEFAULT '{}'    -- JSON: library picks with reasons, books outside the library
);

-- Covers someone flagged as wrong, for an admin to fix in Calibre.
CREATE TABLE IF NOT EXISTS cover_reports (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id    INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    user_id    INTEGER REFERENCES users(id) ON DELETE SET NULL,
    username   TEXT NOT NULL DEFAULT '',
    note       TEXT NOT NULL DEFAULT '',
    status     TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'fixed', 'dismissed')),
    decided_by TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cover_reports_open ON cover_reports(book_id, user_id) WHERE status = 'open';

-- "Report a problem or idea" from anyone in the family, for an admin to
-- look into (and pass on through Diagnose if it's a NovelCheck bug).
CREATE TABLE IF NOT EXISTS problem_reports (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER REFERENCES users(id) ON DELETE SET NULL,
    username   TEXT NOT NULL DEFAULT '',
    text       TEXT NOT NULL,
    page       TEXT NOT NULL DEFAULT '',   -- where they were in the app
    device     TEXT NOT NULL DEFAULT '',   -- the browser's user agent
    version    TEXT NOT NULL DEFAULT '',   -- NovelCheck version at the time
    status     TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'done')),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Discover: books on outside lists (New York Times best sellers, Open Library
-- classics), refreshed daily. list is "nyt:<list name>" or "ol:<subject>".
CREATE TABLE IF NOT EXISTS discover_items (
    list          TEXT NOT NULL,
    rank          INTEGER NOT NULL,
    book_id       INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    weeks_on_list INTEGER NOT NULL DEFAULT 0,   -- NYT: 1 = new on the list this week
    cover_url     TEXT NOT NULL DEFAULT '',     -- the list's cover image, used when Open Library has none
    link          TEXT NOT NULL DEFAULT '',     -- the book's Open Library page, when known
    PRIMARY KEY (list, book_id)
);
CREATE INDEX IF NOT EXISTS idx_discover_items_book ON discover_items(book_id);

-- Each person's last searches in the Library (newest first, 10 kept).
CREATE TABLE IF NOT EXISTS search_history (
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    query       TEXT NOT NULL COLLATE NOCASE,
    searched_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, query)
);

-- Books a seasonal shelf matches by its words (see store.seasonReady): made
-- again when the books change, so Discover and the Library needn't search
-- every description on each visit.
CREATE TABLE IF NOT EXISTS season_books (
    season  TEXT NOT NULL,
    book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    PRIMARY KEY (season, book_id)
);

-- Titles a parent said are the same book as another ("Same book as…"): a
-- book under one of these keys joins that book instead of becoming a new one.
CREATE TABLE IF NOT EXISTS book_aliases (
    norm_key TEXT PRIMARY KEY,
    book_id  INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE
);

-- Box sets (omnibus editions, "Books 1-3"): found by title, split into their
-- books only after a parent confirms. A split box set's books share its
-- files and count as owned wherever the box set is.
CREATE TABLE IF NOT EXISTS box_sets (
    book_id    INTEGER PRIMARY KEY REFERENCES books(id) ON DELETE CASCADE,
    state      TEXT NOT NULL DEFAULT 'found' CHECK (state IN ('found', 'split', 'not_box')),
    proposal   TEXT NOT NULL DEFAULT '[]',
    decided_by TEXT NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS box_members (
    box_id   INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    book_id  INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    created  INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (box_id, book_id)
);
CREATE INDEX IF NOT EXISTS idx_box_members_book ON box_members(book_id);

-- KOReader progress sync: which book each KOReader fingerprint is (worked
-- out from the files NovelCheck has), and where each reader is in it.
CREATE TABLE IF NOT EXISTS kosync_docs (
    document TEXT PRIMARY KEY,
    book_id  INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS kosync_progress (
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    document   TEXT NOT NULL,
    progress   TEXT NOT NULL DEFAULT '',
    percentage REAL NOT NULL DEFAULT 0,
    device     TEXT NOT NULL DEFAULT '',
    device_id  TEXT NOT NULL DEFAULT '',
    updated_at INTEGER NOT NULL DEFAULT 0,  -- Unix seconds, as KOReader expects
    PRIMARY KEY (user_id, document)
);
