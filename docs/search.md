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
2. Generated embeddings (`bm index`)
3. [Ollama](https://ollama.ai) running locally

### How It Works

1. Your query is embedded into a vector via Ollama
2. Cosine similarity is computed against all stored bookmark embeddings
3. Results are ranked by similarity score

This means `"learn to code"` will match pages about programming tutorials even if they never use the exact phrase.

### Building the Index

```bash
bm index                          # index recent bookmarks (default: max 1 year old)
bm index --max-age 0              # index everything
bm index --reindex                # force re-index all
bm index --model custom-model     # use a different embedding model
```

Default model: `qwen3-embedding:0.6b`.

Each bookmark is split into chunks:

- **Chunk 0**: metadata (title + URL + folder path)
- **Chunk 1+**: page content in ~24,000 character slices

### Choosing a Model

Any Ollama embedding model works. Considerations:

| Model | Size | Speed | Quality |
|-------|------|-------|---------|
| `qwen3-embedding:0.6b` | Small | Fast | Good for most use cases |
| `nomic-embed-text` | Medium | Medium | Good general purpose |
| `mxbai-embed-large` | Large | Slower | Higher quality |

Override with `--model` or the `BM_EMBED_MODEL` environment variable.
