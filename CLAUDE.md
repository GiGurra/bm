# bm

CLI tool for creating a searchable database of browser bookmarks with both text (FTS5) and semantic (Ollama embeddings) search.

## Build & Test

```bash
go build ./...
go test ./...
go vet ./...
go install .
```

## Architecture

**Entry point:** `main.go` — builds cobra command tree.

**Commands (under `cmd/`):**

| Command   | Purpose                                                  |
|-----------|----------------------------------------------------------|
| `import`  | Import bookmarks from Chrome (auto-discovers all profiles) |
| `fetch`   | Fetch page content (HTML → text) for bookmarked URLs     |
| `index`   | Generate embeddings via Ollama for semantic search       |
| `search`  | Text search (FTS5) or semantic search (`-s` flag)        |
| `list`    | List/filter bookmarks                                    |
| `sync`    | Run import + index in sequence (--fetch to also fetch)   |
| `stats`   | Show database statistics (per-profile, per-year, fetch status) |

**Packages (under `pkg/`):**

| Package   | Purpose                                                  |
|-----------|----------------------------------------------------------|
| `config`  | Settings from `~/.bm/settings.json`. Profile resolution (CLI > env > config > all) |
| `db`      | SQLite storage (bookmarks, FTS5, embeddings). WAL mode, pure-Go via `modernc.org/sqlite` |
| `chrome`  | Chrome bookmark file parser. Multi-profile discovery     |
| `fetcher` | HTTP page fetch + HTML text extraction                   |
| `ollama`  | Ollama embedding client, cosine similarity, vector encoding |

**Data location:** `~/.bm/bm.sqlite`

## Configuration

**Config file:** `~/.bm/settings.json`

```json
{
  "profiles": [
    {"email": "user@gmail.com"},
    {"gaia_id": "12345678"}
  ]
}
```

**Profile resolution priority:** `--profile` flag > `BM_PROFILE` env var > config file > default (all profiles)

- Profiles are identified by `email` (Google account) or `gaia_id` (stable Google account ID) — not directory names like "Default" which Chrome can reassign.
- `--profile all` or `BM_PROFILE=all` forces all profiles, overriding config.

## Future Ideas

- **`date_last_used` from Chrome**: Chrome's Bookmarks JSON has a `date_last_used` field (WebKit timestamp, `"0"` = never visited). Could be imported and used to find "bookmarked but never visited" pages — e.g. searching for unwatched anime series. Would need: new column in DB, parse in `chrome.go`, display in list/watch, and potentially a filter flag like `--unvisited`.

## Key Dependencies

- `github.com/GiGurra/boa` — typed params wrapper for cobra
- `github.com/spf13/cobra` — CLI framework
- `modernc.org/sqlite` — pure-Go SQLite
- `golang.org/x/net/html` — HTML parsing
