# Content Fetching (Beta)

!!! note "Optional"
    Content fetching is **not required**. Both text search and semantic search work without it — bookmark titles, URLs, and folder paths are always indexed. Fetching adds page body text for deeper search results.

!!! warning "Beta"
    This feature is considered beta. It works, but search result quality with fetched content has not been extensively validated yet.

!!! info "Limitations"
    Pages requiring authentication (login walls, paywalls, etc.) cannot be fetched — bm makes plain HTTP GET requests without cookies or session tokens.

## Usage

```bash
bm fetch                 # fetch unfetched bookmarks (default: max 1 year old)
bm fetch -a              # re-fetch all bookmarks
bm fetch --max-age 6m    # only bookmarks from last 6 months
bm fetch --max-age 0     # no age limit
bm fetch -p "Default"      # filter by profile
bm fetch -n 100          # limit to 100 bookmarks
bm fetch -d 1000         # 1 second delay between fetches
```

## Age Filtering

By default, only bookmarks from the last year are fetched. This avoids wasting time on old bookmarks that may no longer exist.

`--max-age` accepts:

| Format | Meaning |
|--------|---------|
| `Nd` | N days |
| `Nw` | N weeks |
| `Nm` | N months (30 days) |
| `Ny` | N years (365 days) |
| `0` | No limit |

The age is determined from Chrome's original `date_added` timestamp, falling back to the bm import timestamp.

## Error Handling

HTTP errors are classified and recorded so failed URLs aren't retried:

| Status | Meaning |
|--------|---------|
| `ok` | Successfully fetched |
| `error:404` | Page not found |
| `error:403` | Forbidden |
| `error:401` | Unauthorized |
| `error:410` | Gone |
| `error:5xx` | Server error |
| `error:timeout` | Request timed out |
| `error:dns` | DNS resolution failed |
| `error:tls` | TLS/certificate error |
| `error:not-html` | Response wasn't HTML |
| `error:empty` | Page had no extractable text |

Use `bm fetch -a` to retry all bookmarks including previously failed ones.

## How It Works

1. For each URL, bm makes an HTTP GET request
2. The response is parsed as HTML using `golang.org/x/net/html`
3. Text content is extracted (script/style tags are stripped)
4. The text is stored in the `content_text` column alongside the bookmark
5. FTS5 triggers automatically update the full-text search index
