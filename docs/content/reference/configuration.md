---
title: "Configuration"
description: "Environment variables, defaults, and the data directory."
weight: 20
---

`yelp` needs almost no configuration: it runs anonymously against public data out
of the box. The one credential it reads is the optional `YELP_API_KEY`, which
turns on the Fusion API plane. The settings below let you tune the plane,
politeness, and storage.

## The Fusion API key

`YELP_API_KEY` holds a free Yelp developer key. When it is set, `yelp` uses the
Fusion API plane, which answers from any network; without it, `yelp` uses the web
plane, which is best-effort behind Yelp's bot wall. The key is read from the
environment only, never a flag, so it never lands in shell history or a process
list.

```bash
export YELP_API_KEY=...     # turns on the fusion plane
yelp biz garaje-san-francisco --plane web   # force the web plane for one run
```

`--plane auto` (the default) picks fusion when the key is set and web otherwise.
`--plane fusion` without a key, or `categories` on the web plane, exits 4.

## Defaults

| Setting | Default | Flag |
|---|---|---|
| Requests | paced and retried on 429/5xx | `--rate`, `--retries` |
| Per-request timeout | 30s | `--timeout` |
| On-disk cache | under the data directory | `--no-cache` to bypass |

## The data directory

Caches and any record store live under one data directory, chosen in this order:

1. `--data-dir`
2. `YELP_DATA_DIR`
3. `$XDG_DATA_HOME/yelp`
4. `~/.local/share/yelp`

## Environment variables

Every flag has an environment fallback, prefixed `YELP_` in
upper case with dashes as underscores. For example:

```bash
export YELP_RATE=1s        # same as --rate 1s
export YELP_DATA_DIR=~/data/yelp
```

Flags win over environment variables, which win over the built-in defaults.

## Sending records to a store

`--db` tees every emitted record into a store as a side effect of reading, so a
session fills a local database without a separate import step:

```bash
yelp reviews garaje-san-francisco --db out.db        # SQLite file
yelp search "tacos" "San Francisco, CA" --db 'postgres://...'
```
