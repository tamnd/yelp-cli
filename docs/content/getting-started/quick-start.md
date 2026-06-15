---
title: "Quick start"
description: "Run your first searches and reads with yelp."
weight: 30
---

Once `yelp` is on your `PATH`, resolve a reference offline (this always works,
from any network):

```bash
yelp ref id "https://www.yelp.com/biz/garaje-san-francisco" -o json
```

```json
[
  {
    "input": "https://www.yelp.com/biz/garaje-san-francisco",
    "kind": "biz",
    "id": "garaje-san-francisco",
    "url": "https://www.yelp.com/biz/garaje-san-francisco"
  }
]
```

## Read live data

The live reads work best-effort on the web plane (a home or mobile network
usually answers; a datacenter hits the bot wall and exits 4) and reliably on the
fusion plane:

```bash
yelp search "tacos" "San Francisco, CA"     # business search by term and place
yelp biz garaje-san-francisco               # one business by alias
yelp reviews garaje-san-francisco           # a business's reviews
yelp suggest "coffee"                       # autocomplete a prefix
yelp user <user-id>                         # a reviewer's public profile
```

A business is addressed by its alias, like `garaje-san-francisco`, or a `/biz/`
URL. Scope a search to a place with the trailing location argument or
`--location`, and order it with `--sort` and `--price`:

```bash
yelp search "ramen" --location "Oakland, CA" --sort rating --price 1,2 -n 10
```

## Use the Fusion API for reliable reads

The web plane is best-effort behind the bot wall. For reads that answer from any
network, including a datacenter or CI, get a free Yelp developer key and set it in
the environment:

```bash
export YELP_API_KEY=...                      # free at the Yelp developer portal
yelp biz garaje-san-francisco                # now reads the Fusion API
yelp categories                              # the category taxonomy (fusion only)
```

`yelp` uses the fusion plane automatically when the key is set. Force a plane with
`--plane web|fusion|auto`. The key is read from the environment only, never a
flag.

## Shape the output

The same flags work on every command:

```bash
yelp search "tacos" "San Francisco, CA" --fields name,rating,review_count,price
yelp biz garaje-san-francisco --template '{{.Name}} ({{.Rating}})'
yelp search "tacos" "San Francisco, CA" -o jsonl | jq .alias
```

`-o` takes `table`, `json`, `jsonl`, `csv`, `tsv`, `url`, or `raw`. Left to
`auto`, it prints a table to a terminal and JSONL into a pipe, so the same command
reads well by hand and parses cleanly downstream. See
[output formats](/reference/output/) for the full contract.

## Serve it instead

The same operations are available over HTTP and to agents over MCP:

```bash
yelp serve --addr :7777 &
curl -s 'localhost:7777/v1/biz/garaje-san-francisco'   # NDJSON, one record per line
yelp mcp                                                # MCP over stdio
```

## Where to go next

- The [guides](/guides/) cover the common jobs, including using `yelp` as a
  resource-URI driver for a crawling host.
- The [CLI reference](/reference/cli/) is the full command and flag surface.
- [Troubleshooting](/reference/troubleshooting/) explains the exit codes and the
  bot wall.
</content>
