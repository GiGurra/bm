# Commands

## import

Import bookmarks from Chrome. Auto-discovers all profiles by default.

```bash
bm import                    # all profiles
bm import -p "Profile 1"    # specific profile
bm import /path/to/Bookmarks # specific file
```

Uses bulk in-memory diffing — only writes changed bookmarks in a single transaction. Re-imports are near-instant.

Duplicate bookmarks (same URL in same folder, from Chrome sync conflicts) are deduplicated, keeping the entry with the latest `date_added`.

## fetch

Fetch and extract text content from bookmarked pages.

```bash
bm fetch                 # fetch unfetched bookmarks (max 1 year old)
bm fetch -a              # re-fetch all
bm fetch --max-age 6m    # only bookmarks from last 6 months
bm fetch --max-age 0     # no age limit
bm fetch -p gigurra      # specific profile only
bm fetch -n 100          # limit to 100 bookmarks
bm fetch -d 1000         # 1 second delay between fetches
```

HTTP errors (404, 403, timeouts, etc.) are recorded so failed URLs aren't retried on subsequent runs.

### Age format

`--max-age` accepts: `Nd` (days), `Nw` (weeks), `Nm` (months), `Ny` (years). Default: `1y`.

## index

Build semantic search index using local Ollama embeddings.

```bash
bm index                       # index bookmarks (max 1 year old)
bm index --max-age 0           # index all bookmarks
bm index --max-age 6m          # last 6 months
bm index --reindex             # force re-index everything
bm index --model nomic-embed-text  # use a different model
```

Default model: `qwen3-embedding:0.6b`. Requires [Ollama](https://ollama.ai) running locally.

## search

Search bookmarks by text or meaning.

```bash
bm search "golang"           # FTS5 text search
bm search -s "learn coding"  # semantic search (requires index)
bm search -n 20 "rust"       # limit results
bm search -p gigurra "go"    # filter by profile
```

Text search uses SQLite FTS5 with a LIKE fallback for special characters. Semantic search uses cosine similarity against Ollama embeddings.

## list

List and browse bookmarks.

```bash
bm list                  # list bookmarks (newest first)
bm list -w               # interactive TUI browser
bm list -f "recipes"     # filter by folder
bm list -p gigurra       # filter by profile
bm list -n 100           # limit results
```

### Interactive mode keys

| Key | Action |
|-----|--------|
| `/` | Text search |
| `s` | Semantic search |
| `↑`/`↓` or `j`/`k` | Navigate |
| `Enter` | Open URL in browser |
| `1`-`5` | Sort by column |
| `Esc` | Clear search |
| `q` | Quit |

## sync

Run import + fetch + index in sequence.

```bash
bm sync                          # full pipeline
bm sync --model nomic-embed-text # with custom model
```

## stats

Show database statistics: bookmarks by year, fetch status breakdown, and per-profile counts.

```bash
bm stats
```

## profile

Manage browser profiles.

```bash
bm profile list    # show profiles with stats
```

## clear

Clear data from the database.

```bash
bm clear all          # everything
bm clear contents     # fetched content only
bm clear embeddings   # embeddings only
```
