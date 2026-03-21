package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

var (
	once     sync.Once
	globalDB *sql.DB
)

func dbPath() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".bm")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "bm.sqlite")
}

// Open returns the singleton database connection, creating it if needed.
func Open() (*sql.DB, error) {
	var err error
	once.Do(func() {
		if globalDB != nil {
			// Already set (e.g. by OpenMem for testing)
			return
		}
		globalDB, err = sql.Open("sqlite", dbPath()+"?_journal_mode=WAL")
		if err != nil {
			return
		}
		err = migrate(globalDB)
	})
	if err != nil {
		return nil, err
	}
	return globalDB, nil
}

// OpenMem creates a fresh in-memory database. Intended for testing.
// Resets the singleton so subsequent Open() calls return this DB.
func OpenMem() (*sql.DB, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	globalDB = db
	once = sync.Once{}
	once.Do(func() {}) // mark as done
	return db, nil
}

func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS bookmarks (
			url TEXT PRIMARY KEY,
			title TEXT NOT NULL DEFAULT '',
			folder_path TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT 'chrome',
			content_text TEXT NOT NULL DEFAULT '',
			fetched_at TEXT NOT NULL DEFAULT '',
			added_at TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS bookmarks_fts USING fts5(
			url, title, content_text,
			content='bookmarks',
			content_rowid='rowid'
		)`,
		// Triggers to keep FTS in sync
		`CREATE TRIGGER IF NOT EXISTS bookmarks_ai AFTER INSERT ON bookmarks BEGIN
			INSERT INTO bookmarks_fts(rowid, url, title, content_text)
			VALUES (new.rowid, new.url, new.title, new.content_text);
		END`,
		`CREATE TRIGGER IF NOT EXISTS bookmarks_ad AFTER DELETE ON bookmarks BEGIN
			INSERT INTO bookmarks_fts(bookmarks_fts, rowid, url, title, content_text)
			VALUES ('delete', old.rowid, old.url, old.title, old.content_text);
		END`,
		`CREATE TRIGGER IF NOT EXISTS bookmarks_au AFTER UPDATE ON bookmarks BEGIN
			INSERT INTO bookmarks_fts(bookmarks_fts, rowid, url, title, content_text)
			VALUES ('delete', old.rowid, old.url, old.title, old.content_text);
			INSERT INTO bookmarks_fts(rowid, url, title, content_text)
			VALUES (new.rowid, new.url, new.title, new.content_text);
		END`,
		`CREATE TABLE IF NOT EXISTS bookmark_embeddings (
			url TEXT NOT NULL,
			chunk_index INTEGER NOT NULL,
			chunk_text TEXT NOT NULL DEFAULT '',
			embedding BLOB,
			model TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (url, chunk_index)
		)`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: %w\n  SQL: %s", err, stmt)
		}
	}

	// Incremental migrations for columns added after initial schema
	alterStmts := []string{
		// fetch_status: "" (not attempted), "ok", "error:404", "error:403", "error:timeout", etc.
		`ALTER TABLE bookmarks ADD COLUMN fetch_status TEXT NOT NULL DEFAULT ''`,
		// chrome_added_at: original Chrome bookmark creation time (for age filtering)
		`ALTER TABLE bookmarks ADD COLUMN chrome_added_at TEXT NOT NULL DEFAULT ''`,
		// source_name: human-readable name for the source (e.g. profile email)
		`ALTER TABLE bookmarks ADD COLUMN source_name TEXT NOT NULL DEFAULT ''`,
	}
	for _, stmt := range alterStmts {
		_, _ = db.Exec(stmt) // ignore "duplicate column" errors
	}

	return nil
}
