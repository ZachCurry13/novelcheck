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
    hide_lgbtq       INTEGER NOT NULL DEFAULT 0,
    hide_unrated     INTEGER NOT NULL DEFAULT 0,
    delivery_method  TEXT NOT NULL DEFAULT 'none' CHECK (delivery_method IN ('none', 'email', 'koreader')),
    kindle_email     TEXT NOT NULL DEFAULT '',
    guide_seen       INTEGER NOT NULL DEFAULT 0,
    age_level        INTEGER NOT NULL DEFAULT 0,   -- kid accounts: 1 young kids .. 5 adults; 0 = not set
    max_spice        INTEGER NOT NULL DEFAULT -1,  -- kid accounts: hide books above this many peppers (0-5); -1 = no limit
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
    lgbtq_content    INTEGER NOT NULL DEFAULT 0,
    summary_verdict  TEXT NOT NULL DEFAULT '',
    approved         INTEGER NOT NULL DEFAULT 0,   -- parent marked "OK": bypasses filters
    approved_by      TEXT NOT NULL DEFAULT '',
    age_level        INTEGER NOT NULL DEFAULT 0,   -- parent-set age group, 1 young kids .. 5 adults; 0 = not set
    spice_level      INTEGER,                      -- 0-5 peppers; NULL = rated before the pepper scale (or not rated)
    spice_reason     TEXT NOT NULL DEFAULT '',     -- short "why this many peppers", e.g. "Heavy innuendo, on-page foreplay"
    rules_version    INTEGER NOT NULL DEFAULT 0,   -- store.RulesVersion the rating was made under
    flags_version    INTEGER NOT NULL DEFAULT 0,   -- custom filters version the rating checked
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
    cost              REAL                           -- USD at the time; NULL on older rows
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

-- Deep reads: the AI reads a book's whole EPUB in parts. Admins start them;
-- anyone else requests one for an admin to approve. notes is a JSON list of
-- what each part contained.
CREATE TABLE IF NOT EXISTS deep_reads (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
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
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_deep_reads_open ON deep_reads(book_id) WHERE status IN ('requested', 'queued', 'reading');

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
