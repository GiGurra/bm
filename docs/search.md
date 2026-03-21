# Search

bm supports two search modes: full-text search (default) and semantic search.

## Text Search

```bash
bm search "golang tutorials"
bm search -n 20 "rust"          # limit results
bm search -p "Default" "go"     # filter by profile
```

Text search uses SQLite FTS5 which indexes bookmark titles, URLs, and fetched page content. It supports standard FTS5 query syntax (AND, OR, NOT, phrase matching).

For queries containing special characters (dots, colons, etc.), bm falls back to a case-insensitive `LIKE` search on title and URL.

## Semantic Search

```bash
bm search -s "how to learn programming"
bm search -s "good sci-fi recommendations"
```

Semantic search finds bookmarks by **meaning** rather than exact keywords. It requires:

1. Fetched content (`bm fetch`)
2. Generated embeddings (`bm index`) — see [Indexing](indexing.md)
3. [Ollama](https://ollama.com) running locally

### How It Works

1. Your query is embedded into a vector via Ollama
2. Cosine similarity is computed against all stored bookmark embeddings
3. Results are ranked by similarity score

This means `"learn to code"` will match pages about programming tutorials even if they never use the exact phrase.

See [Indexing](indexing.md) for details on building the embedding index, choosing models, and age filtering.
