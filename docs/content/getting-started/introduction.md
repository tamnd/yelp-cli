---
title: "Introduction"
description: "What yelp is, how it is put together, and which surfaces it reads."
weight: 10
---

`yelp` reads public Yelp data: a business search by term and place, one business
with its full detail, a business's reviews, a reviewer's public profile,
autocomplete suggestions, and the Yelp category taxonomy. It is a single binary.
It speaks to Yelp over plain HTTPS and shapes each surface into a clean record.
There is no login and nothing to run alongside it. The official Fusion API is
opt-in through a free key you supply in an environment variable.

## How it is built

- A **library package** (`yelp`) holds the two-plane HTTP client, the JSON-LD and
  web-state parsers, the Fusion client, and the typed data models. It paces
  requests, sends a browser User-Agent on the web plane because that is what a
  logged-out reader looks like, caches on disk, and retries the transient
  failures any public site throws under load.
- A **domain** (`yelp/domain.go`) declares each operation once on the
  [any-cli/kit](https://github.com/tamnd/any-cli) framework. That single
  declaration becomes a CLI command, an HTTP route, an MCP tool, and a
  resource-URI dereference. It is the one place you add to the tool.
- A thin **`cmd/yelp`** hands the assembled app to `kit.Run`, which builds the
  command tree and the serve and mcp surfaces.

## One operation, four surfaces

Because an operation is surface-neutral, the same `biz` you run on the command
line is also a route and a tool:

```bash
yelp biz garaje-san-francisco            # the command
yelp serve --addr :7777                  # GET /v1/biz/garaje-san-francisco
yelp mcp                                 # the biz tool, over stdio
ant get yelp://biz/garaje-san-francisco  # the URI dereference (via a host)
```

## Two planes

Yelp can be read two ways, and `yelp` is honest about the line between them.

The default **web plane** reads what a logged-out browser reads: a business page
and the schema.org JSON-LD island it embeds, the `review_feed` JSON endpoint, the
search page's embedded state, the `aselect` autocomplete JSON, and a reviewer's
public `/user_details` page. Yelp fronts the site with a PerimeterX bot manager
that classifies a request on IP reputation and TLS fingerprint and hard-walls
datacenter IPs, so these reads are best-effort: from a home or mobile network
they usually answer, from a datacenter they hit the wall and exit 4.

The opt-in **fusion plane** is Yelp's official [Fusion
API](https://docs.developer.yelp.com) at `api.yelp.com/v3`, addressed with a free
developer key. It answers from any network and carries the cleanest detail. Set
the key in the environment and `yelp` uses it automatically:

```bash
export YELP_API_KEY=...        # a Yelp developer key, free to create
yelp biz garaje-san-francisco  # now reads the Fusion API, no wall
```

The client uses the fusion plane when `YELP_API_KEY` is set and the web plane
otherwise. Force one with `--plane web|fusion|auto`. The key is read from the
environment only, never a flag, so it never lands in shell history or a process
list.

## What anonymous access reaches

This tool sorts the surfaces into what works regardless and what is gated, and
never pretends the line is elsewhere.

Works from any network, offline:

- `ref id`, `ref url` (pure string resolution, no request)

Reachable on the fusion plane from any network, best-effort on the web plane:

- `biz` (the `/biz/` JSON-LD island, or `businesses/{id}`)
- `search` (the search page state, or `businesses/search`)
- `reviews` (the `review_feed` endpoint, or `businesses/{id}/reviews`)
- `suggest` (the `aselect` autocomplete, or `autocomplete`)

Fusion plane only:

- `categories` (the `categories` endpoint; there is no clean web equivalent, so
  this needs `YELP_API_KEY`)

Web plane only:

- `user` (the public `/user_details` page; the Fusion API has no user endpoint)

From a datacenter the web-plane surfaces hit the bot wall and exit 4. The remedy
the tool names is to run from a residential or mobile network, or to set
`YELP_API_KEY` and use the Fusion API. It does not forge a TLS fingerprint and
does not rent or rotate IPs to get past the edge: it reads what a logged-out
browser reads, the way it reads it.

Records carry only fields a logged-out reader or a free Fusion key can fill.
There is no owner dashboard, no message thread, no private analytics, because none
of that is reachable without a signed-in account. A business shows its name,
rating and review count, price, categories, address and coordinates, hours,
transactions, attributes, claim and closure state, photos, and description; a
review carries the rating, author and a link to the author's profile, date, text,
and the useful/funny/cool counts; a user carries the public profile a visitor
sees. A field a surface does not show is left empty rather than guessed.

## Scope

`yelp` is a read-only client over data Yelp already serves publicly, on the web
plane, or through the official Fusion API on the fusion plane. It reads that data
and shapes it for you. That narrow scope keeps it a single small binary with no
database, no daemon, and no setup.

`yelp` is an independent tool and is not affiliated with Yelp.

Next: [install it](/getting-started/installation/), then take the
[quick start](/getting-started/quick-start/).
</content>
