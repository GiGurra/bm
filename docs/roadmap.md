# Roadmap

Planned features and directions for bm. These are ideas under consideration — not commitments or timelines.

## Bookmark Cleanup

Commands for identifying and removing old, broken, or unwanted bookmarks:

- **Dead link detection** — flag bookmarks that return 404, DNS failures, or other permanent errors
- **Duplicate detection** — find identical or near-identical bookmarks across folders
- **Age-based cleanup** — bulk remove bookmarks older than a threshold
- **Interactive review** — TUI for reviewing and deleting bookmarks one by one

## Chrome Sync-Back

Write changes back to Chrome's bookmark files, so deletions and edits in bm are reflected in the browser:

- **Delete from Chrome** — remove bookmarks from Chrome profiles after deleting them in bm
- **Two-way sync** — keep bm and Chrome in sync, not just one-way import

## Reorganization

Commands for moving and restructuring bookmarks across folders:

- **Move between folders** — `bm move <url> --to "Folder/Subfolder"`
- **Bulk reorganization** — move bookmarks matching a search or filter to a new folder
- **LLM-assisted cleanup** — let a language model suggest folder structures or categorize unfiled bookmarks based on content and titles
