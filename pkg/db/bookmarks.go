package db

import (
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
		ON CONFLICT(url) DO UPDATE SET
			title=excluded.title,
			folder_path=excluded.folder_path,
			source=excluded.source,
			source_name=CASE WHEN excluded.source_name != '' THEN excluded.source_name ELSE bookmarks.source_name END,
			updated_at=excluded.updated_at,
			chrome_added_at=CASE WHEN excluded.chrome_added_at != '' THEN excluded.chrome_added_at ELSE bookmarks.chrome_added_at END`,
		b.URL, b.Title, b.FolderPath, b.Source, b.SourceName, b.AddedAt, b.UpdatedAt, b.ChromeAddedAt)
	return err
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
			(SELECT COUNT(DISTINCT e.url) FROM bookmark_embeddings e
			 WHERE e.url IN (SELECT url FROM bookmarks WHERE source = b.source)) as indexed
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

// ListBookmarksBySource returns bookmarks filtered by source ID or source name.
func ListBookmarksBySource(sourceFilter string) ([]Bookmark, error) {
	db, err := Open()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`SELECT url, title, folder_path, source, source_name, content_text, fetched_at, fetch_status, added_at, updated_at, chrome_added_at
		FROM bookmarks WHERE source = ? OR source_name LIKE ? ORDER BY updated_at DESC`,
		sourceFilter, "%"+sourceFilter+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanBookmarks(rows)
}

// ListFetchableBySource returns fetchable bookmarks filtered by source.
func ListFetchableBySource(sourceFilter string) ([]Bookmark, error) {
	db, err := Open()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`SELECT url, title, folder_path, source, source_name, content_text, fetched_at, fetch_status, added_at, updated_at, chrome_added_at
		FROM bookmarks WHERE fetched_at = '' AND fetch_status = '' AND (source = ? OR source_name LIKE ?)
		ORDER BY added_at DESC`,
		sourceFilter, "%"+sourceFilter+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanBookmarks(rows)
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
