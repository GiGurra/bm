# Configuration

bm stores settings in `~/.bm/settings.json`. The config file is optional — without it, bm uses all Chrome profiles by default.

## Managing settings

```bash
# Show current config
bm config list

# Add a profile (tab completion suggests available Chrome profiles)
bm config add-profile user@gmail.com

# Remove a profile
bm config remove-profile user@gmail.com
```

## Config file format

```json
{
  "profiles": [
    {"email": "user@gmail.com"},
    {"email": "work@company.com"}
  ]
}
```

Each profile entry can use either field:

| Field     | Description                          | Example              |
|-----------|--------------------------------------|----------------------|
| `email`   | Google account email                 | `"user@gmail.com"`   |
| `gaia_id` | Google account ID (stable internal)  | `"123456789012345"`  |

!!! tip
    Use `email` — it's what you see in Chrome and easy to remember. `gaia_id` is available as a stable fallback if needed.

!!! warning
    Don't use Chrome directory names like `"Default"` or `"Profile 1"` — Chrome can reassign these at any time.

## Profile resolution

When a command needs to know which Chrome profile(s) to use, this priority applies:

| Priority | Source              | Example                                   |
|----------|---------------------|--------------------------------------------|
| 1        | `--profile` flag    | `bm import --profile user@gmail.com`       |
| 2        | `BM_PROFILE` env    | `BM_PROFILE=user@gmail.com bm import`      |
| 3        | Config file         | `profiles` array in `settings.json`        |
| 4        | Default             | All Chrome profiles                        |

### Using `all`

If a config file limits profiles, you can override it to use all profiles:

```bash
bm import --profile all          # ignore config, use all profiles
BM_PROFILE=all bm sync           # same via env var
```

## Environment variables

| Variable        | Description                 | Default                      |
|-----------------|-----------------------------|------------------------------|
| `BM_PROFILE`    | Default Chrome profile      | (all profiles)               |
| `BM_EMBED_MODEL`| Ollama embedding model      | `qwen3-embedding:0.6b`      |
| `BM_OLLAMA_URL` | Ollama API base URL         | `http://localhost:11434`     |
