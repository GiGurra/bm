# bm - Bookmark Manager

CLI tool for creating a searchable database of browser bookmarks with both text (FTS5) and semantic (Ollama embeddings) search.

## Features

- **Chrome Import** - Auto-discovers all Chrome profiles, imports bookmarks with stable identity tracking
- **Full-Text Search** - SQLite FTS5 for instant text search across titles, URLs, and page content
- **Semantic Search** - Find bookmarks by meaning using local Ollama embeddings
- **Page Fetching** - Downloads and extracts text content from bookmarked pages
- **Statistics** - Breakdown by year, fetch status, and profile
- **Interactive Mode** - TUI browser with search, sorting, and filtering

## Installation

### bm

```bash
go install github.com/GiGurra/bm@latest
```

### Ollama (required for semantic search)

Semantic search requires [Ollama](https://ollama.com) running locally with an embedding model.

```bash
# Install Ollama
brew install ollama          # macOS / Linux (Homebrew)
# or download from https://ollama.com/download

# Start the Ollama server
ollama serve                 # or: brew services start ollama

# Pull the default embedding model
ollama pull qwen3-embedding:0.6b
```

Text search (`bm search`) and the interactive browser (`bm list -w`) work without Ollama. Only semantic search (`bm search -s`, `bm index`) requires it.

## Quick Start

```bash
# Import bookmarks from Chrome
bm import

# Fetch page content and build search index
bm fetch
bm index

# Or do all three at once
bm sync

# Search
bm search "golang tutorials"
bm search -s "how to learn programming"  # semantic search

# Interactive browser
bm list -w

# Stats
bm stats
```

## How It Works

```
Chrome Bookmarks JSON
        │
        ▼
   bm import ──► SQLite DB (~/.bm/bm.sqlite)
                      │
                 bm fetch ──► page content stored in DB
                      │
                 bm index ──► Ollama embeddings stored in DB
                      │
              bm search / bm list -w
```

1. **Import** reads Chrome's bookmark files and stores them in SQLite with a composite key `(url, folder_path, source)` — duplicate URLs in different folders are preserved
2. **Fetch** downloads page content and extracts text from HTML
3. **Index** generates vector embeddings via Ollama for semantic search
4. **Search** queries either FTS5 (text) or cosine similarity (semantic)

## Data Storage

All data is stored locally in `~/.bm/bm.sqlite` using WAL mode for performance. The database uses a pure-Go SQLite driver (`modernc.org/sqlite`) — no CGO required.
