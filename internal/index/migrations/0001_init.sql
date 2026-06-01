-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS catalog (
    path         TEXT PRIMARY KEY,
    content_hash TEXT NOT NULL,
    title        TEXT NOT NULL DEFAULT '',
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS chunks (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    path      TEXT NOT NULL,
    title     TEXT NOT NULL DEFAULT '',
    tags      TEXT NOT NULL DEFAULT '',
    heading   TEXT NOT NULL DEFAULT '',
    ord       INTEGER NOT NULL DEFAULT 0,
    body      TEXT NOT NULL,
    token_est INTEGER NOT NULL DEFAULT 0
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_chunks_path ON chunks (path);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5 (
    title, tags, heading, body,
    content='chunks',
    content_rowid='id',
    tokenize='unicode61'
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS chunks_ai AFTER INSERT ON chunks BEGIN
    INSERT INTO chunks_fts (rowid, title, tags, heading, body)
    VALUES (new.id, new.title, new.tags, new.heading, new.body);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS chunks_ad AFTER DELETE ON chunks BEGIN
    INSERT INTO chunks_fts (chunks_fts, rowid, title, tags, heading, body)
    VALUES ('delete', old.id, old.title, old.tags, old.heading, old.body);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS chunks_au AFTER UPDATE ON chunks BEGIN
    INSERT INTO chunks_fts (chunks_fts, rowid, title, tags, heading, body)
    VALUES ('delete', old.id, old.title, old.tags, old.heading, old.body);
    INSERT INTO chunks_fts (rowid, title, tags, heading, body)
    VALUES (new.id, new.title, new.tags, new.heading, new.body);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS vectors (
    chunk_id INTEGER PRIMARY KEY REFERENCES chunks(id) ON DELETE CASCADE,
    dim      INTEGER NOT NULL,
    vec      BLOB NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS vectors;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS chunks_au;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS chunks_ad;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TRIGGER IF EXISTS chunks_ai;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS chunks_fts;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS chunks;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS catalog;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS meta;
-- +goose StatementEnd
