---
title: "CLI"
description: "Every command and subcommand, with the flags that matter."
weight: 10
---

```
yelp <command> [arguments] [flags]
```

Run `yelp <command> --help` for the full flag list on any command. This
page is the map; keep it in step with the real command tree as you add to it.

## Commands

| Command | What it does |
|---|---|
| `search <term> [location]` | Business search by term and place |
| `biz <alias>` | One business by alias |
| `reviews <alias>` | A business's reviews |
| `user <id>` | A reviewer's public profile (web plane only) |
| `suggest <prefix>` | Autocomplete suggestions for a prefix |
| `categories` | The Yelp category taxonomy (needs `YELP_API_KEY`) |
| `category <alias>` | One category by alias (needs `YELP_API_KEY`) |
| `ref id <ref>` | Classify a reference into its (kind, id), offline |
| `ref url <kind> <id>` | Build the canonical URL for a (kind, id), offline |
| `serve [--addr]` | Serve the operations over HTTP as NDJSON |
| `mcp` | Run as an MCP server over stdio |
| `version` | Print the version and exit |

A business is addressed by its alias, like `garaje-san-francisco`, or a `/biz/`
URL; a user by the id in a `/user_details?userid=` URL; a category by its alias,
like `coffee`.

## Plane and search flags

| Flag | Meaning |
|---|---|
| `--plane` | Data plane: `auto`, `web`, or `fusion` (default `auto`; `fusion` needs `YELP_API_KEY`) |
| `--location` | Place to scope a search or autocomplete to, e.g. `"Oakland, CA"` |
| `--sort` | Search order: `best_match`, `rating`, `review_count`, or `distance` |
| `--price` | Search price filter: any of `1,2,3,4` comma-joined |
| `--radius` | Fusion search radius in meters from the center (max 40000) |
| `--categories` | Fusion search category filter: one or more aliases comma-joined |
| `--attributes` | Fusion search attribute filter: one or more aliases comma-joined |
| `--open-now` | Fusion search filter: only businesses open now |
| `--locale` | Locale for prices, hours, and dates (default `en_US`) |
| `--user-agent` | User-Agent sent with each web-plane request |

The `--radius`, `--categories`, `--attributes`, and `--open-now` filters apply on
the fusion plane, where the search API documents them. A fusion search needs a
place: pass a location argument or `--location` (or a latitude and longitude), or
the command exits 2.

## Global flags

These are shared by every operation, so they work the same on every command.

| Flag | Meaning |
|---|---|
| `-o, --output` | Output format: `auto`, `table`, `json`, `jsonl`, `csv`, `tsv`, `url`, `raw` |
| `--fields` | Comma-separated columns to keep |
| `--template` | Go text/template applied per record |
| `--no-header` | Omit the header row in `table` and `csv` |
| `-n, --limit` | Stop after N records (0 means no limit) |
| `--rate` | Minimum delay between requests |
| `--retries` | Retry attempts on rate limit or 5xx |
| `--timeout` | Per-request timeout |
| `--cache-ttl` | How long a cached response stays fresh |
| `--refresh` | Fetch fresh copies and rewrite the cache, ignoring any hit |
| `--data-dir` | Override the data directory |
| `--no-cache` | Bypass on-disk caches |
| `--db` | Tee every record into a store (e.g. `out.db`, `postgres://...`) |
| `-v, --verbose` | Increase verbosity (repeatable) |
| `-q, --quiet` | Suppress progress output |
| `--color` | `auto`, `always`, or `never` |

See [output formats](/reference/output/) for what `-o`, `--fields`, and
`--template` produce, and [configuration](/reference/configuration/) for
environment variables and defaults.
