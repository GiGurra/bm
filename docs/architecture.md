# Architecture

## Project Structure

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

## Database

All data lives in `~/.bm/bm.sqlite` (WAL mode, pure-Go SQLite via `modernc.org/sqlite`).

### Schema

**bookmarks** — primary key: `(url, folder_path, source)`

| Column | Description |
|--------|-------------|
| `url` | Bookmark URL |
| `folder_path` | Chrome folder hierarchy |
| `source` | Stable source ID (e.g. `chrome:gaia:12345`) |
| `title` | Page title |
| `content_text` | Fetched page text |
| `fetch_status` | `""`, `"ok"`, `"error:404"`, etc. |
| `chrome_added_at` | Original Chrome bookmark timestamp |

**bookmarks_fts** — FTS5 virtual table synced via triggers, indexes `url`, `title`, `content_text`.

**bookmark_embeddings** — primary key: `(url, chunk_index)`

| Column | Description |
|--------|-------------|
| `embedding` | Float32 vector blob |
| `chunk_text` | Text that was embedded |
| `model` | Ollama model used |

**schema_version** — tracks migration state. Migrations are applied sequentially on startup.

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/GiGurra/boa` | Typed params wrapper for cobra |
| `github.com/spf13/cobra` | CLI framework |
| `modernc.org/sqlite` | Pure-Go SQLite (no CGO) |
| `golang.org/x/net/html` | HTML parsing |
| `github.com/jedib0t/go-pretty/v6` | Table formatting |
| `github.com/charmbracelet/bubbletea` | TUI framework |
