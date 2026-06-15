---
title: "Resource URIs"
description: "Use yelp as a database/sql-style driver so a host program can address Yelp as yelp:// URIs."
weight: 20
---

`yelp` is a command line, but the `yelp` Go package is also a small driver that
makes Yelp addressable as a resource URI. A host program registers it the way a
program registers a database driver with `database/sql`, then dereferences
`yelp://` URIs without knowing anything about how Yelp is fetched.

The host that does this today is [ant](https://github.com/tamnd/ant), a single
binary that puts one URI namespace over a family of site tools. The examples
below use `ant`; any program that links the package gets the same behavior.

## Mounting the driver

A host enables the driver with one blank import, exactly like
`import _ "github.com/lib/pq"`:

```go
import _ "github.com/tamnd/yelp-cli/yelp"
```

The package's `init` registers a domain with the scheme `yelp` for the hosts
`www.yelp.com` and `yelp.com`. The standalone `yelp` binary does not change.

## Addressing records

A URI is `scheme://authority/id`. The resolver types are:

| URI                   | What it is                            |
| --------------------- | ------------------------------------- |
| `yelp://biz/<alias>`  | one business, keyed by its alias      |
| `yelp://user/<id>`    | a reviewer's public profile           |

```bash
ant get yelp://biz/garaje-san-francisco   # the business record
ant cat yelp://biz/garaje-san-francisco   # just the description body
ant get yelp://user/<id>                  # a reviewer's profile
ant url yelp://biz/garaje-san-francisco   # the live https URL
ant resolve https://www.yelp.com/biz/garaje-san-francisco  # a pasted link, back to its URI
```

`biz` is best-effort on the web plane (a datacenter may hit Yelp's bot wall and
report need-auth) and reliable on the fusion plane when `YELP_API_KEY` is set.
`user` is web plane only, since the Fusion API has no user endpoint. See
[what anonymous access reaches](/getting-started/introduction/#what-anonymous-access-reaches).

## Collections

`ls` lists the members of a collection. Each list operation has its own
authority, so they never shadow one another:

| URI                          | What it lists                       |
| ---------------------------- | ----------------------------------- |
| `yelp://search/<term>`       | businesses matching a term          |
| `yelp://reviews/<alias>`     | a business's reviews                |
| `yelp://suggest/<prefix>`    | autocomplete suggestions            |
| `yelp://categories`          | the Yelp category taxonomy          |

```bash
ant ls yelp://search/tacos                  # businesses matching a term
ant ls yelp://reviews/garaje-san-francisco  # the business's reviews
ant ls yelp://suggest/coffee                # autocomplete a prefix
```

Scope a search to a place through the host's query options, the same as
`--location` on the command line.

## Walking the graph

Every record carries explicit edges to the records it points at, so a host can
breadth-first crawl the site and write it to disk without scraping URLs out of
free text. A resolver edge names a bare field and points at one record; a
collection edge carries the parent id under a `<name>_ref` field and points at a
list authority. The edges are:

| From         | Field          | Edge to                  |
| ------------ | -------------- | ------------------------ |
| `Suggestion` | `search_ref`   | `yelp://search/<text>`   |
| `Suggestion` | `business`     | `yelp://biz/<alias>`     |
| `Business`   | `reviews_ref`  | `yelp://reviews/<alias>` |
| `Business`   | `category_ref` | `yelp://search/<alias>`  |
| `Review`     | `business`     | `yelp://biz/<alias>`     |
| `Review`     | `author_id`    | `yelp://user/<id>`       |
| `Category`   | `search_ref`   | `yelp://search/<alias>`  |

The edges close into one connected graph. A suggestion fans out into a place
search and, for a business suggestion, straight to that business; a search card
walks through to its full business; a business reaches its reviews and a
same-category search; a review reaches back to its business and on to the
reviewer's own profile; a category fans into a search. No node is left without an
outward edge, so a crawl started anywhere reaches the rest of the reachable site.
Starting from any node, `--follow` walks these edges:

```bash
ant export yelp://search/tacos --follow 2 --to ./data  # each business, then its reviews and a same-category search
ant get yelp://biz/garaje-san-francisco
ant cat yelp://biz/garaje-san-francisco       # the description body
ant url yelp://biz/garaje-san-francisco
```

Each record is written under its minted URI with its edges intact, so the saved
set reconstructs the slice of the site that was reached: the search results, the
full business behind each card, its reviews, and the profile of each reviewer.

These edge fields stay out of the table and CSV views (they would be noise in a
terminal) but are always present in the JSON and JSONL a host reads.

## Why this is the same code

The driver and the binary share one definition per operation. A resolver op
answers both `yelp biz` on the command line and `ant get yelp://biz/...` through a
host, from the same handler and the same client. There is no second
implementation to keep in step.
</content>
