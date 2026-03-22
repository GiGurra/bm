package db

import (
	"fmt"
	"strings"
	"time"
)

type Bookmark struct {
	URL           string
	Title         string
	FolderPath    string
	Source        string // stable source ID (e.g. "chrome:gaia:12345")
	SourceName    string // human-readable source name (e.g. "gigurra@gmail.com (Default)")
	ContentText   string
	FetchedAt     string
	FetchStatus   string // "", "ok", "error:404", "error:403", "error:timeout", etc.
	AddedAt       string
	UpdatedAt     string
	ChromeAddedAt string // original Chrome bookmark creation time
}

type bookmarkKey struct {
	URL        string
	FolderPath string
	Source     string
}

// UpsertBookmark inserts or updates a bookmark. Does NOT overwrite content_text,
// fetched_at, or fetch_status if they already have values (preserves fetched content on re-import).
func UpsertBookmark(b *Bookmark) error {
	db, err := Open()
	if err != nil {
		return err
	}

	now := time.Now().Format(time.RFC3339)
	if b.AddedAt == "" {
		b.AddedAt = now
	}
	b.UpdatedAt = now

	_, err = db.Exec(`INSERT INTO bookmarks (url, title, folder_path, source, source_name, content_text, fetched_at, fetch_status, added_at, updated_at, chrome_added_at)
		VALUES (?, ?, ?, ?, ?, '', '', '', ?, ?, ?)
		ON CONFLICT(url, folder_path, source) DO UPDATE SET
			title=excluded.title,
			source_name=CASE WHEN excluded.source_name != '' THEN excluded.source_name ELSE bookmarks.source_name END,
			updated_at=excluded.updated_at,
			chrome_added_at=CASE WHEN excluded.chrome_added_at != '' THEN excluded.chrome_added_at ELSE bookmarks.chrome_added_at END`,
		b.URL, b.Title, b.FolderPath, b.Source, b.SourceName, b.AddedAt, b.UpdatedAt, b.ChromeAddedAt)
	return err
}

// BulkUpsertBookmarks loads all existing bookmarks into memory, compares with the
// incoming list, and only writes the ones that are new or changed. Bookmarks that
// exist in the DB for the same source(s) but are absent from the incoming list are
// deleted (along with their embeddings). All writes happen in a single transaction.
func BulkUpsertBookmarks(bookmarks []*Bookmark) (inserted, updated, deleted, total int, err error) {
	db, err := Open()
	if err != nil {
		return 0, 0, 0, 0, err
	}

	// Collect which sources are represented in the incoming bookmarks
	incomingSources := make(map[string]bool)
	for _, b := range bookmarks {
		incomingSources[b.Source] = true
	}

	// Load existing bookmarks into a map keyed by (url, folder_path, source)
	existing := make(map[bookmarkKey]Bookmark)
	rows, err := db.Query(`SELECT url, title, folder_path, source, source_name, chrome_added_at FROM bookmarks`)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("load existing bookmarks: %w", err)
	}
	for rows.Next() {
		var b Bookmark
		if err := rows.Scan(&b.URL, &b.Title, &b.FolderPath, &b.Source, &b.SourceName, &b.ChromeAddedAt); err != nil {
			rows.Close()
			return 0, 0, 0, 0, fmt.Errorf("scan existing bookmark: %w", err)
		}
		existing[bookmarkKey{b.URL, b.FolderPath, b.Source}] = b
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("iterate existing bookmarks: %w", err)
	}

	// Deduplicate by composite key, keeping the entry with the latest chrome_added_at
	deduped := make(map[bookmarkKey]*Bookmark, len(bookmarks))
	for _, b := range bookmarks {
		key := bookmarkKey{b.URL, b.FolderPath, b.Source}
		if prev, exists := deduped[key]; !exists || b.ChromeAddedAt > prev.ChromeAddedAt {
			deduped[key] = b
		}
	}
	total = len(deduped)

	// Determine which bookmarks need writing
	now := time.Now().Format(time.RFC3339)
	var toWrite []*Bookmark
	for _, b := range deduped {
		if b.AddedAt == "" {
			b.AddedAt = now
		}
		b.UpdatedAt = now

		key := bookmarkKey{b.URL, b.FolderPath, b.Source}
		if old, exists := existing[key]; exists {
			changed := old.Title != b.Title ||
				(b.SourceName != "" && old.SourceName != b.SourceName) ||
				(b.ChromeAddedAt != "" && old.ChromeAddedAt != b.ChromeAddedAt)
			if !changed {
				continue
			}
			updated++
		} else {
			inserted++
		}
		toWrite = append(toWrite, b)
	}

	// Find stale entries: in DB for an incoming source but absent from the incoming set
	var toDelete []bookmarkKey
	for key, eb := range existing {
		if !incomingSources[eb.Source] {
			continue // different source, don't touch
		}
		if _, stillExists := deduped[key]; !stillExists {
			toDelete = append(toDelete, key)
		}
	}
	deleted = len(toDelete)

	if len(toWrite) == 0 && len(toDelete) == 0 {
		return 0, 0, 0, total, nil
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Upserts
	if len(toWrite) > 0 {
		stmt, err := tx.Prepare(`INSERT INTO bookmarks (url, title, folder_path, source, source_name, content_text, fetched_at, fetch_status, added_at, updated_at, chrome_added_at)
			VALUES (?, ?, ?, ?, ?, '', '', '', ?, ?, ?)
			ON CONFLICT(url, folder_path, source) DO UPDATE SET
				title=excluded.title,
				source_name=CASE WHEN excluded.source_name != '' THEN excluded.source_name ELSE bookmarks.source_name END,
				updated_at=excluded.updated_at,
				chrome_added_at=CASE WHEN excluded.chrome_added_at != '' THEN excluded.chrome_added_at ELSE bookmarks.chrome_added_at END`)
		if err != nil {
			return 0, 0, 0, 0, fmt.Errorf("prepare upsert: %w", err)
		}
		defer stmt.Close()

		for _, b := range toWrite {
			if _, err := stmt.Exec(b.URL, b.Title, b.FolderPath, b.Source, b.SourceName, b.AddedAt, b.UpdatedAt, b.ChromeAddedAt); err != nil {
				return 0, 0, 0, 0, fmt.Errorf("upsert %s: %w", b.URL, err)
			}
		}
	}

	// Deletes
	if len(toDelete) > 0 {
		delBookmark, err := tx.Prepare(`DELETE FROM bookmarks WHERE url = ? AND folder_path = ? AND source = ?`)
		if err != nil {
			return 0, 0, 0, 0, fmt.Errorf("prepare delete bookmark: %w", err)
		}
		defer delBookmark.Close()

		delEmbed, err := tx.Prepare(`DELETE FROM bookmark_embeddings WHERE url = ?`)
		if err != nil {
			return 0, 0, 0, 0, fmt.Errorf("prepare delete embeddings: %w", err)
		}
		defer delEmbed.Close()

		for _, key := range toDelete {
			if _, err := delBookmark.Exec(key.URL, key.FolderPath, key.Source); err != nil {
				return 0, 0, 0, 0, fmt.Errorf("delete bookmark %s: %w", key.URL, err)
			}
			// Clean up embeddings for this URL (only if no other bookmark entries share it)
			var remaining int
			_ = tx.QueryRow(`SELECT COUNT(*) FROM bookmarks WHERE url = ? AND NOT (folder_path = ? AND source = ?)`,
				key.URL, key.FolderPath, key.Source).Scan(&remaining)
			if remaining == 0 {
				if _, err := delEmbed.Exec(key.URL); err != nil {
					return 0, 0, 0, 0, fmt.Errorf("delete embeddings for %s: %w", key.URL, err)
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("commit: %w", err)
	}

	return inserted, updated, deleted, total, nil
}

// UpdateContent sets the fetched content for a bookmark.
func UpdateContent(url, contentText string) error {
	db, err := Open()
	if err != nil {
		return err
	}

	now := time.Now().Format(time.RFC3339)
	_, err = db.Exec(`UPDATE bookmarks SET content_text=?, fetched_at=?, fetch_status='ok', updated_at=? WHERE url=?`,
		contentText, now, now, url)
	return err
}

// UpdateFetchStatus marks a bookmark as unfetchable with a reason.
func UpdateFetchStatus(url, status string) error {
	db, err := Open()
	if err != nil {
		return err
	}

	now := time.Now().Format(time.RFC3339)
	_, err = db.Exec(`UPDATE bookmarks SET fetched_at=?, fetch_status=?, updated_at=? WHERE url=?`,
		now, status, now, url)
	return err
}

// ListBookmarks returns all bookmarks.
func ListBookmarks() ([]Bookmark, error) {
	db, err := Open()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`SELECT url, title, folder_path, source, source_name, content_text, fetched_at, fetch_status, added_at, updated_at, chrome_added_at
		FROM bookmarks ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanBookmarks(rows)
}

// ListFetchable returns bookmarks that haven't been fetched and aren't marked unfetchable.
func ListFetchable() ([]Bookmark, error) {
	db, err := Open()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`SELECT url, title, folder_path, source, source_name, content_text, fetched_at, fetch_status, added_at, updated_at, chrome_added_at
		FROM bookmarks WHERE fetched_at = '' AND fetch_status = '' ORDER BY added_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanBookmarks(rows)
}

// SearchFTS searches bookmarks using FTS5, with a LIKE fallback for queries
// that contain special characters (dots, colons, etc.) that FTS5 may not handle well.
// Searches title, URL, and content text. Case-insensitive.
func SearchFTS(query string, limit int) ([]Bookmark, error) {
	db, err := Open()
	if err != nil {
		return nil, err
	}

	// Try FTS5 first
	rows, err := db.Query(`SELECT b.url, b.title, b.folder_path, b.source, b.source_name, b.content_text, b.fetched_at, b.fetch_status, b.added_at, b.updated_at, b.chrome_added_at
		FROM bookmarks_fts f
		JOIN bookmarks b ON b.rowid = f.rowid
		WHERE bookmarks_fts MATCH ?
		ORDER BY rank
		LIMIT ?`, query, limit)
	if err == nil {
		defer rows.Close()
		results, scanErr := scanBookmarks(rows)
		if scanErr == nil && len(results) > 0 {
			return results, nil
		}
	}

	// Fallback: LIKE search on title and URL (handles special chars, partial matches)
	likePattern := "%" + query + "%"
	rows2, err := db.Query(`SELECT url, title, folder_path, source, source_name, content_text, fetched_at, fetch_status, added_at, updated_at, chrome_added_at
		FROM bookmarks
		WHERE title LIKE ? COLLATE NOCASE OR url LIKE ? COLLATE NOCASE
		ORDER BY chrome_added_at DESC
		LIMIT ?`, likePattern, likePattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()

	return scanBookmarks(rows2)
}

// CountBookmarks returns the total number of bookmarks.
func CountBookmarks() (int, error) {
	db, err := Open()
	if err != nil {
		return 0, err
	}

	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM bookmarks`).Scan(&count)
	return count, err
}

// CountFetched returns the number of bookmarks with content.
func CountFetched() (int, error) {
	db, err := Open()
	if err != nil {
		return 0, err
	}

	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM bookmarks WHERE fetch_status = 'ok'`).Scan(&count)
	return count, err
}

// ProfileStats holds per-profile aggregate stats.
type ProfileStats struct {
	Source     string
	SourceName string
	Total      int
	Fetched    int
	Errors     int
	Indexed    int
}

// ListProfileStats returns aggregate stats grouped by source.
func ListProfileStats() ([]ProfileStats, error) {
	db, err := Open()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT
			b.source,
			b.source_name,
			COUNT(*) as total,
			SUM(CASE WHEN b.fetch_status = 'ok' THEN 1 ELSE 0 END) as fetched,
			SUM(CASE WHEN b.fetch_status LIKE 'error:%' THEN 1 ELSE 0 END) as errors,
			SUM(CASE WHEN EXISTS (SELECT 1 FROM bookmark_embeddings e WHERE e.url = b.url) THEN 1 ELSE 0 END) as indexed
		FROM bookmarks b
		GROUP BY b.source
		ORDER BY total DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ProfileStats
	for rows.Next() {
		var s ProfileStats
		if err := rows.Scan(&s.Source, &s.SourceName, &s.Total, &s.Fetched, &s.Errors, &s.Indexed); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// sourceInClause returns a SQL fragment and args for filtering by source IDs.
// Returns empty string and nil args if sources is nil/empty (no filtering).
func sourceInClause(sources []string) (string, []any) {
	if len(sources) == 0 {
		return "", nil
	}
	placeholders := make([]string, len(sources))
	args := make([]any, len(sources))
	for i, s := range sources {
		placeholders[i] = "?"
		args[i] = s
	}
	return "source IN (" + strings.Join(placeholders, ",") + ")", args
}

// ListBookmarksBySources returns bookmarks filtered by source IDs.
// Pass nil to return all bookmarks (equivalent to ListBookmarks).
func ListBookmarksBySources(sources []string) ([]Bookmark, error) {
	if len(sources) == 0 {
		return ListBookmarks()
	}
	db, err := Open()
	if err != nil {
		return nil, err
	}

	where, args := sourceInClause(sources)
	rows, err := db.Query(`SELECT url, title, folder_path, source, source_name, content_text, fetched_at, fetch_status, added_at, updated_at, chrome_added_at
		FROM bookmarks WHERE `+where+` ORDER BY updated_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanBookmarks(rows)
}

// ListFetchableBySources returns fetchable bookmarks filtered by source IDs.
// Pass nil to return all fetchable bookmarks.
func ListFetchableBySources(sources []string) ([]Bookmark, error) {
	if len(sources) == 0 {
		return ListFetchable()
	}
	db, err := Open()
	if err != nil {
		return nil, err
	}

	where, args := sourceInClause(sources)
	rows, err := db.Query(`SELECT url, title, folder_path, source, source_name, content_text, fetched_at, fetch_status, added_at, updated_at, chrome_added_at
		FROM bookmarks WHERE fetched_at = '' AND fetch_status = '' AND (`+where+`)
		ORDER BY added_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanBookmarks(rows)
}

// YearStats holds per-year aggregate counts.
type YearStats struct {
	Year    string
	Total   int
	Fetched int
	Errors  int
	Indexed int
}

// ListYearStats returns aggregate stats grouped by year of chrome_added_at.
// Pass nil sources for all bookmarks.
func ListYearStats(sources []string) ([]YearStats, error) {
	db, err := Open()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT
			COALESCE(NULLIF(substr(chrome_added_at, 1, 4), ''), '?') as year,
			COUNT(*) as total,
			SUM(CASE WHEN fetch_status = 'ok' THEN 1 ELSE 0 END) as fetched,
			SUM(CASE WHEN fetch_status LIKE 'error:%' THEN 1 ELSE 0 END) as errors,
			SUM(CASE WHEN EXISTS (SELECT 1 FROM bookmark_embeddings e WHERE e.url = bookmarks.url) THEN 1 ELSE 0 END) as indexed
		FROM bookmarks`

	var args []any
	if len(sources) > 0 {
		where, whereArgs := sourceInClause(sources)
		query += ` WHERE ` + where
		args = whereArgs
	}
	query += `
		GROUP BY year
		ORDER BY year`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []YearStats
	for rows.Next() {
		var s YearStats
		if err := rows.Scan(&s.Year, &s.Total, &s.Fetched, &s.Errors, &s.Indexed); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// FetchStatusStats holds counts per fetch status category.
type FetchStatusStats struct {
	Status string
	Count  int
}

// ListFetchStatusStats returns bookmark counts grouped by fetch_status.
// Pass nil sources for all bookmarks.
func ListFetchStatusStats(sources []string) ([]FetchStatusStats, error) {
	db, err := Open()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT
			CASE
				WHEN fetch_status = '' AND fetched_at = '' THEN 'not attempted'
				WHEN fetch_status = 'ok' THEN 'ok'
				ELSE fetch_status
			END as status,
			COUNT(*) as count
		FROM bookmarks`

	var args []any
	if len(sources) > 0 {
		where, whereArgs := sourceInClause(sources)
		query += ` WHERE ` + where
		args = whereArgs
	}
	query += `
		GROUP BY status
		ORDER BY count DESC`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []FetchStatusStats
	for rows.Next() {
		var s FetchStatusStats
		if err := rows.Scan(&s.Status, &s.Count); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func scanBookmarks(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]Bookmark, error) {
	var result []Bookmark
	for rows.Next() {
		var b Bookmark
		if err := rows.Scan(&b.URL, &b.Title, &b.FolderPath, &b.Source, &b.SourceName, &b.ContentText,
			&b.FetchedAt, &b.FetchStatus, &b.AddedAt, &b.UpdatedAt, &b.ChromeAddedAt); err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, rows.Err()
}
