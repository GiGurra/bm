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

```bash
go install github.com/GiGurra/bm@latest
```

Requires [Ollama](https://ollama.ai) for semantic search (optional).

## Quick Start

```bash
# Import bookmarks from Chrome
bm import

# Fetch page content
bm fetch

# Build semantic search index
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

## Data Storage

All data is stored locally in `~/.bm/bm.sqlite` using WAL mode for performance. The database uses a pure-Go SQLite driver (`modernc.org/sqlite`) — no CGO required.
