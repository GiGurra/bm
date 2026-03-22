# Architecture

## Project Structure

Entry point is `main.go` which builds the cobra command tree. Commands live under `cmd/`, library packages under `pkg/`.

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
