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
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
    token_hash TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at DATETIME NOT NULL,
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
    completion_tokens INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_token_usage_at ON token_usage(at);
