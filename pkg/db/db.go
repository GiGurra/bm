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

const currentSchemaVersion = 4

func migrate(db *sql.DB) error {
	// Ensure schema_version table exists
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("create schema_version table: %w", err)
	}

	var version int
	err := db.QueryRow(`SELECT version FROM schema_version`).Scan(&version)
	if err == sql.ErrNoRows {
		// Check if this is truly a fresh DB or a pre-versioning DB
		var tableCount int
		_ = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='bookmarks'`).Scan(&tableCount)
		if tableCount > 0 {
			version = 1 // existing DB from before versioning
		}
		_, _ = db.Exec(`INSERT INTO schema_version (version) VALUES (?)`, version)
	} else if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	// Apply migrations in order
	for version < currentSchemaVersion {
		nextVersion := version + 1
		if err := applyMigration(db, nextVersion); err != nil {
			return fmt.Errorf("migration to v%d: %w", nextVersion, err)
		}
		if _, err := db.Exec(`UPDATE schema_version SET version = ?`, nextVersion); err != nil {
			return fmt.Errorf("update schema version to %d: %w", nextVersion, err)
		}
		version = nextVersion
	}

	return nil
}

func applyMigration(db *sql.DB, version int) error {
	switch version {
	case 1:
		return migrateV1(db)
	case 2:
		return migrateV2(db)
	case 3:
		return migrateV3(db)
	case 4:
		return migrateV4(db)
	default:
		return fmt.Errorf("unknown migration version %d", version)
	}
}

// migrateV1 creates the initial schema (for fresh databases).
func migrateV1(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS bookmarks (
			url TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL DEFAULT '',
			folder_path TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT 'chrome',
			source_name TEXT NOT NULL DEFAULT '',
			content_text TEXT NOT NULL DEFAULT '',
			fetched_at TEXT NOT NULL DEFAULT '',
			fetch_status TEXT NOT NULL DEFAULT '',
			added_at TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT '',
			chrome_added_at TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (url, folder_path, source)
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS bookmarks_fts USING fts5(
			url, title, content_text,
			content='bookmarks',
			content_rowid='rowid'
		)`,
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
			folder_path TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT '',
			chunk_index INTEGER NOT NULL,
			chunk_text TEXT NOT NULL DEFAULT '',
			embedding BLOB,
			model TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT '',
			content_hash TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (url, folder_path, source, chunk_index)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("%w\n  SQL: %s", err, stmt)
		}
	}
	return nil
}

// migrateV2 changes the bookmarks PK from (url) to (url, folder_path, source).
func migrateV2(db *sql.DB) error {
	stmts := []string{
		`DROP TRIGGER IF EXISTS bookmarks_ai`,
		`DROP TRIGGER IF EXISTS bookmarks_ad`,
		`DROP TRIGGER IF EXISTS bookmarks_au`,
		`DROP TABLE IF EXISTS bookmarks_fts`,
		`CREATE TABLE bookmarks_new (
			url TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL DEFAULT '',
			folder_path TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT 'chrome',
			source_name TEXT NOT NULL DEFAULT '',
			content_text TEXT NOT NULL DEFAULT '',
			fetched_at TEXT NOT NULL DEFAULT '',
			fetch_status TEXT NOT NULL DEFAULT '',
			added_at TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT '',
			chrome_added_at TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (url, folder_path, source)
		)`,
		`INSERT OR IGNORE INTO bookmarks_new (url, title, folder_path, source, source_name, content_text, fetched_at, fetch_status, added_at, updated_at, chrome_added_at)
			SELECT url, title, folder_path, source, source_name, content_text, fetched_at, fetch_status, added_at, updated_at, chrome_added_at FROM bookmarks`,
		`DROP TABLE bookmarks`,
		`ALTER TABLE bookmarks_new RENAME TO bookmarks`,
		// Recreate FTS and triggers
		`CREATE VIRTUAL TABLE IF NOT EXISTS bookmarks_fts USING fts5(
			url, title, content_text,
			content='bookmarks',
			content_rowid='rowid'
		)`,
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
		// Rebuild FTS index from existing data
		`INSERT INTO bookmarks_fts(bookmarks_fts) VALUES('rebuild')`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("%w\n  SQL: %s", err, stmt)
		}
	}
	return nil
}

// migrateV3 adds content_hash column to bookmark_embeddings for change detection.
// On fresh databases (v1 already has the column), this is a no-op.
func migrateV3(db *sql.DB) error {
	// Check if column already exists (v1 schema includes it)
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('bookmark_embeddings') WHERE name='content_hash'`).Scan(&count)
	if count > 0 {
		return nil
	}
	_, err := db.Exec(`ALTER TABLE bookmark_embeddings ADD COLUMN content_hash TEXT NOT NULL DEFAULT ''`)
	return err
}

// migrateV4 changes bookmark_embeddings PK from (url, chunk_index) to
// (url, folder_path, source, chunk_index) to match the bookmarks table keying.
// Existing embeddings are dropped — they'll be regenerated on next `bm index`.
func migrateV4(db *sql.DB) error {
	stmts := []string{
		`DROP TABLE IF EXISTS bookmark_embeddings`,
		`CREATE TABLE bookmark_embeddings (
			url TEXT NOT NULL,
			folder_path TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT '',
			chunk_index INTEGER NOT NULL,
			chunk_text TEXT NOT NULL DEFAULT '',
			embedding BLOB,
			model TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT '',
			content_hash TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (url, folder_path, source, chunk_index)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("%w\n  SQL: %s", err, stmt)
		}
	}
	return nil
}
