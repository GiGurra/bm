package db

import (
	"database/sql"
	"time"
)

type EmbeddingRow struct {
	URL         string
	FolderPath  string
	Source      string
	ChunkIndex  int
	ChunkText   string
	Embedding   []byte // raw float32 bytes
	Model       string
	CreatedAt   time.Time
	ContentHash string
}

func UpsertEmbedding(row *EmbeddingRow) error {
	db, err := Open()
	if err != nil {
		return err
	}

	_, err = db.Exec(`INSERT INTO bookmark_embeddings (url, folder_path, source, chunk_index, chunk_text, embedding, model, created_at, content_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(url, folder_path, source, chunk_index) DO UPDATE SET
			chunk_text=excluded.chunk_text, embedding=excluded.embedding,
			model=excluded.model, created_at=excluded.created_at, content_hash=excluded.content_hash`,
		row.URL, row.FolderPath, row.Source, row.ChunkIndex, row.ChunkText, row.Embedding, row.Model,
		row.CreatedAt.Format(time.RFC3339Nano), row.ContentHash)
	return err
}

func ListAllEmbeddings() ([]*EmbeddingRow, error) {
	db, err := Open()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`SELECT url, folder_path, source, chunk_index, chunk_text, embedding, model, created_at FROM bookmark_embeddings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEmbeddingRows(rows)
}

func DeleteEmbeddings(url, folderPath, source string) error {
	db, err := Open()
	if err != nil {
		return err
	}

	_, err = db.Exec(`DELETE FROM bookmark_embeddings WHERE url = ? AND folder_path = ? AND source = ?`, url, folderPath, source)
	return err
}

type EmbeddedInfo struct {
	CreatedAt   time.Time
	ContentHash string
}

func ListEmbeddedKeys() (map[BookmarkKey]EmbeddedInfo, error) {
	db, err := Open()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`SELECT url, folder_path, source, MAX(created_at), COALESCE(MAX(content_hash), '') FROM bookmark_embeddings GROUP BY url, folder_path, source`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[BookmarkKey]EmbeddedInfo)
	for rows.Next() {
		var url, folderPath, source, createdAt, hash string
		if err := rows.Scan(&url, &folderPath, &source, &createdAt, &hash); err != nil {
			return nil, err
		}
		t, _ := time.Parse(time.RFC3339Nano, createdAt)
		result[BookmarkKey{url, folderPath, source}] = EmbeddedInfo{CreatedAt: t, ContentHash: hash}
	}
	return result, rows.Err()
}

func ListEmbeddingModels() ([]string, error) {
	db, err := Open()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`SELECT DISTINCT model FROM bookmark_embeddings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var models []string
	for rows.Next() {
		var model string
		if err := rows.Scan(&model); err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	return models, rows.Err()
}

func scanEmbeddingRows(rows *sql.Rows) ([]*EmbeddingRow, error) {
	var result []*EmbeddingRow
	for rows.Next() {
		var r EmbeddingRow
		var createdAt string
		if err := rows.Scan(&r.URL, &r.FolderPath, &r.Source, &r.ChunkIndex, &r.ChunkText, &r.Embedding, &r.Model, &createdAt); err != nil {
			return nil, err
		}
		r.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		result = append(result, &r)
	}
	return result, rows.Err()
}
