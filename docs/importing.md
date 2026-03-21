# Importing Bookmarks

## Chrome Import

bm auto-discovers all Chrome profiles and imports their bookmarks.

```bash
bm import                    # all profiles (default)
bm import -p "Profile 1"    # specific profile by directory name
bm import -p user@example.com   # specific profile by email
bm import /path/to/Bookmarks    # specific file
```

### Profile Discovery

Chrome stores bookmarks in JSON files under its user data directory:

| OS | Path |
|----|------|
| Linux / WSL | `~/.config/google-chrome/<Profile>/Bookmarks` |
| macOS | `~/Library/Application Support/Google/Chrome/<Profile>/Bookmarks` |
| Windows | `%LOCALAPPDATA%\Google\Chrome\User Data\<Profile>\Bookmarks` |

bm reads Chrome's `Local State` file to get stable profile identities (Google account ID + email) so profiles are tracked correctly even if Chrome reassigns directory names.

### Composite Primary Key

Bookmarks are stored with the key `(url, folder_path, source)`:

- Same URL in different folders → separate entries
- Same URL from different Chrome profiles → separate entries
- Same URL in the same folder from the same profile → single entry

### Deduplication

Chrome sync can create duplicate bookmarks (same URL, same folder). bm deduplicates these by keeping the entry with the **latest `date_added`** timestamp.

### Fast Re-imports

On import, bm loads all existing bookmarks into memory and diffs against the incoming data. Only new or changed bookmarks are written, in a single SQLite transaction. This makes re-imports near-instant — even with thousands of bookmarks.

```
$ bm import
Found 1 Chrome profile(s):
  - user@example.com (Default) [chrome:gaia:123456789]

Imported from user@example.com (Default): 0 new, 0 updated, 2886 unchanged (total 2886)
```

### Managing Profiles

```bash
bm profile list    # show all profiles with bookmark/fetch/index counts
```
