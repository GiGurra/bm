# Statistics

The `bm stats` command shows an overview of your bookmark database.

## Usage

```bash
bm stats
```

## Output

### Bookmarks by Year

Shows how many bookmarks were created each year, along with how many have been fetched, had errors, or been indexed.

```
Bookmarks: 2886 total, 450 fetched

┌──────┬───────┬─────────┬────────┬─────────┐
│ YEAR │ TOTAL │ FETCHED │ ERRORS │ INDEXED │
├──────┼───────┼─────────┼────────┼─────────┤
│ 2012 │   587 │      42 │     89 │       0 │
│ 2013 │   277 │      31 │     55 │       0 │
│ ...  │   ... │     ... │    ... │     ... │
│ 2025 │   169 │     120 │     12 │     115 │
├──────┼───────┼─────────┼────────┼─────────┤
│ TOTAL│  2886 │     450 │    312 │     129 │
└──────┴───────┴─────────┴────────┴─────────┘
```

### Fetch Status Breakdown

Shows counts per fetch status category — useful for understanding how much content is available.

### Per-Profile Stats

Shows bookmark counts grouped by Chrome profile, including fetched, errors, and indexed counts.
