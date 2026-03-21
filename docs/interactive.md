# Interactive Browser

The interactive mode provides a TUI for browsing, searching, and opening bookmarks.

## Usage

```bash
bm list -w                # launch interactive browser
bm list -w -p "Default"   # filter by profile
```

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `/` | Start text search |
| `s` | Start semantic search |
| `↑`/`↓` or `j`/`k` | Navigate (exits search input but keeps filter) |
| `Enter` | Open selected bookmark in browser |
| `1`-`5` | Sort by column |
| `Esc` | Clear search / exit mode |
| `q` | Quit |

## Features

- **Search as you type** — results update live with debouncing (150ms for text, 250ms for semantic)
- **Column sorting** — press number keys to sort by different columns
- **Dual search** — switch between text (`/`) and semantic (`s`) search
- **URL opening** — press Enter to open the selected bookmark in your default browser
- **Profile filtering** — use `-p` to scope to a specific Chrome profile

## Non-interactive Mode

Without `-w`, `bm list` outputs a table:

```bash
bm list                  # list bookmarks (newest first)
bm list -f "recipes"     # filter by folder substring
bm list -n 100           # limit results
bm list -p "Default"     # filter by profile
```
