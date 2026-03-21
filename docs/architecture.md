# Architecture

## Overview

```
main.go          CLI entry point (cobra command tree)
cmd/
  importcmd/     Import bookmarks from Chrome
  fetch/         Fetch page content (HTML → text)
  index/         Generate Ollama embeddings
  search/        Text (FTS5) and semantic search
  list/          List/filter bookmarks + interactive TUI
  sync/          Run import + fetch + index
  stats/         Database statistics
  profile/       Profile management
  clear/         Clear data
pkg/
  db/            SQLite storage (bookmarks, FTS5, embeddings)
  chrome/        Chrome bookmark file parser
  fetcher/       HTTP page fetch + HTML text extraction
  ollama/        Ollama embedding client
  table/         TUI table widget
```

## Database Schema

Primary key: `(url, folder_path, source)` — the same URL in different folders or from different Chrome profiles is stored as separate entries.

### bookmarks

| Column | Type | Description |
|--------|------|-------------|
| url | TEXT | Bookmark URL |
| folder_path | TEXT | Chrome folder hierarchy |
| source | TEXT | Stable source ID (e.g. `chrome:gaia:12345`) |
| title | TEXT | Page title |
| source_name | TEXT | Human-readable source name |
| content_text | TEXT | Fetched page text |
| fetched_at | TEXT | When content was fetched |
| fetch_status | TEXT | `""`, `"ok"`, `"error:404"`, etc. |
| added_at | TEXT | When added to bm database |
| updated_at | TEXT | Last update time |
| chrome_added_at | TEXT | Original Chrome bookmark timestamp |

### bookmarks_fts

FTS5 virtual table synced via triggers. Indexes `url`, `title`, and `content_text`.

### bookmark_embeddings

| Column | Type | Description |
|--------|------|-------------|
| url | TEXT | Bookmark URL |
| chunk_index | INTEGER | Chunk number (0 = metadata, 1+ = content) |
| chunk_text | TEXT | Text that was embedded |
| embedding | BLOB | Float32 vector |
| model | TEXT | Ollama model used |
| created_at | TEXT | When embedding was created |

### schema_version

Tracks migration state. Migrations are applied sequentially on startup.

## Import Pipeline

1. **Parse** — Read Chrome's `Bookmarks` JSON file (one per profile)
2. **Deduplicate** — Collapse entries with same `(url, folder_path, source)`, keeping latest `chrome_added_at`
3. **Diff** — Load existing bookmarks into memory, compare against incoming
4. **Write** — Only insert/update changed entries, in a single transaction

This makes re-imports near-instant even with thousands of bookmarks.

## Search

### Text Search (FTS5)

Uses SQLite's FTS5 extension with `MATCH` queries, falling back to `LIKE` for queries with special characters.

### Semantic Search

1. Query text is embedded via Ollama
2. All stored embeddings are loaded
3. Cosine similarity is computed against each
4. Top-N results are returned

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/GiGurra/boa` | Typed params wrapper for cobra |
| `github.com/spf13/cobra` | CLI framework |
| `modernc.org/sqlite` | Pure-Go SQLite (no CGO) |
| `golang.org/x/net/html` | HTML parsing for content extraction |
| `github.com/jedib0t/go-pretty/v6` | Table formatting |
| `github.com/charmbracelet/bubbletea` | TUI framework (interactive mode) |
