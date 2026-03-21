# Statistics

The `bm stats` command shows an overview of your bookmark database.

## Usage

```bash
bm stats                          # all profiles
bm stats -p user@example.com      # filter by profile
```

## Output

### Profiles

Shows per-profile counts: how many bookmarks are in Chrome (after dedup), how many have been imported, fetched, and indexed.

```
Profiles:
┌──────────────────────────────┬───────────┬──────────┬─────────┬─────────┐
│ PROFILE                      │ IN CHROME │ IMPORTED │ FETCHED │ INDEXED │
├──────────────────────────────┼───────────┼──────────┼─────────┼─────────┤
│ user@example.com (Default)   │       138 │      138 │       0 │     138 │
│ other@gmail.com (Profile 1)  │      2888 │        0 │       0 │       0 │
├──────────────────────────────┼───────────┼──────────┼─────────┼─────────┤
│ TOTAL                        │      3026 │      138 │       0 │     138 │
└──────────────────────────────┴───────────┴──────────┴─────────┴─────────┘
```

### Bookmarks by Year

Shows counts per year, including how many are in Chrome vs imported/fetched/indexed.

```
By year:
┌───────┬───────────┬──────────┬─────────┬────────┬─────────┐
│ YEAR  │ IN CHROME │ IMPORTED │ FETCHED │ ERRORS │ INDEXED │
├───────┼───────────┼──────────┼─────────┼────────┼─────────┤
│ 2023  │       256 │        0 │       0 │      0 │       0 │
│ 2024  │       261 │       66 │       0 │      0 │      66 │
│ 2025  │       221 │       52 │       0 │      0 │      52 │
│ 2026  │        38 │       20 │       0 │      0 │      20 │
├───────┼───────────┼──────────┼─────────┼────────┼─────────┤
│ TOTAL │       776 │      138 │       0 │      0 │     138 │
└───────┴───────────┴──────────┴─────────┴────────┴─────────┘
```

### Fetch Status Breakdown

Shows counts per fetch status category — useful for understanding how much content is available.
