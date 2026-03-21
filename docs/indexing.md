# Indexing

Indexing generates vector embeddings for your bookmarks using [Ollama](https://ollama.com), enabling semantic search. This is a local operation — no data leaves your machine.

## Usage

```bash
bm index                          # index recent bookmarks (default: max 1 year old)
bm index --max-age 6m             # only bookmarks from last 6 months
bm index --max-age 0              # index everything regardless of age
bm index --reindex                # force re-index all bookmarks
bm index --model custom-model     # use a different embedding model
bm index -p "Default"             # limit to a specific profile
```

## Age Filtering

By default, only bookmarks from the last year are indexed. This keeps indexing fast and avoids spending time on old bookmarks that may be less relevant.

`--max-age` accepts the same formats as `bm fetch`:

| Format | Meaning |
|--------|---------|
| `Nd` | N days |
| `Nw` | N weeks |
| `Nm` | N months (30 days) |
| `Ny` | N years (365 days) |
| `0` | No limit |

## Prerequisites

Before indexing, you need:

1. **Fetched content** — run `bm fetch` first so there's page text to embed
2. **Ollama running** — `ollama serve` or `brew services start ollama`
3. **An embedding model pulled** — `ollama pull qwen3-embedding:0.6b`

## How It Works

Each bookmark is split into chunks and sent to Ollama for embedding:

- **Chunk 0**: metadata (title + URL + folder path)
- **Chunk 1+**: page content in ~24,000 character slices

The resulting vectors are stored in the database alongside the bookmarks.

## Choosing a Model

Default model: `qwen3-embedding:0.6b`

Any Ollama embedding model works. Override with `--model` or the `BM_EMBED_MODEL` environment variable.

| Model | Size | Speed | Notes |
|-------|------|-------|-------|
| `qwen3-embedding:0.6b` | Small | Fast | Good for most use cases |
| `nomic-embed-text` | Medium | Medium | Good general purpose |
| `mxbai-embed-large` | Large | Slower | Higher quality |

!!! warning "Use the same model for indexing and searching"
    The embedding model used during `bm index` must match the one used during `bm search -s`. Vectors from different models are incompatible — cosine similarity between them produces meaningless results. If you switch models, re-index with `bm index --reindex`.
