# yelp

A command line for [Yelp](https://www.yelp.com). One binary that resolves any
Yelp reference offline, searches businesses by term and place, opens a business
with its full detail, reads a business's reviews, shows a reviewer's public
profile, completes a prefix into autocomplete suggestions, and lists the Yelp
category taxonomy. No login, nothing to run alongside it. The official Fusion API
is opt-in through a free key you supply in an environment variable.

```
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

On a terminal the table header and JSON values are colorized; piped to a file or
another program the output drops to plain text so it parses cleanly. Use
`--color always` to keep color through a pipe, or `--color never` to drop it.

Full documentation: [yelp-cli.tamnd.com](https://yelp-cli.tamnd.com).

## Two planes

Yelp can be read two ways, and `yelp` is honest about the line between them.

The default **web plane** reads what a logged-out browser reads: a business page
and the schema.org JSON-LD island it embeds, the `review_feed` JSON endpoint, the
search page's embedded state, the autocomplete JSON, and a reviewer's public
profile. Yelp fronts the site with a PerimeterX bot manager that classifies a
request on IP reputation and TLS fingerprint and hard-walls datacenter IPs, so
these reads are best-effort: from a home or mobile network they usually answer,
from a datacenter they hit the wall and exit 4.

The opt-in **fusion plane** is Yelp's official [Fusion
API](https://docs.developer.yelp.com) at `api.yelp.com/v3`, addressed with a free
developer key. It answers from any network and carries the cleanest detail. Set
the key in the environment and `yelp` uses it automatically:

```sh
export YELP_API_KEY=...        # a Yelp developer key, free to create
yelp biz garaje-san-francisco  # now reads the Fusion API, no wall
```

The client uses the fusion plane when `YELP_API_KEY` is set and the web plane
otherwise. Force one with `--plane web|fusion|auto`. The key is read from the
environment only, never a flag, so it never lands in shell history or a process
list.

## Install

```sh
go install github.com/tamnd/yelp-cli/cmd/yelp@latest
```

Or grab a prebuilt binary from the [releases page](https://github.com/tamnd/yelp-cli/releases).
The binary is pure Go with no runtime dependencies. You can also run the
container image:

```sh
docker run --rm ghcr.io/tamnd/yelp:latest --help
```

Build from source:

```sh
git clone https://github.com/tamnd/yelp-cli
cd yelp-cli
make build      # produces ./bin/yelp
```

## Quick start

```sh
yelp suggest "coffee"                       # autocomplete a prefix (best-effort on web)
yelp search "tacos" "San Francisco, CA"     # business search by term and place
yelp biz garaje-san-francisco               # one business by alias
yelp reviews garaje-san-francisco           # a business's reviews
yelp user <user-id>                         # a reviewer's public profile (web plane only)
yelp categories                             # the Yelp category taxonomy (needs YELP_API_KEY)
yelp category coffee                        # one category by alias (needs YELP_API_KEY)
```

Most commands accept a bare alias, a `/biz/` path, or a full Yelp URL wherever
they take a business reference. The `ref` commands resolve those offline, with no
network call:

```sh
yelp ref id "https://www.yelp.com/biz/garaje-san-francisco" -o json
yelp ref url biz garaje-san-francisco
```

Scope a search or autocomplete to a place with the trailing location argument or
`--location`, and order results with `--sort best_match|rating|review_count|distance`
and `--price 1,2,3,4`:

```sh
yelp search "ramen" --location "Oakland, CA" --sort rating --price 1,2 -n 10
```

The fusion plane carries more search filters: `--radius <meters>` from the center,
`--categories <alias,alias>`, `--attributes <alias,alias>`, and `--open-now`. A
fusion search needs a place, so pass a location argument or `--location` (or a
latitude and longitude); without one it exits 2 rather than guessing.

```sh
yelp search "coffee" "Oakland, CA" --radius 1500 --open-now --attributes wheelchair_accessible
```

## How it works

On the web plane, a business page renders server-side and embeds a schema.org
`application/ld+json` island; `yelp` GETs `/biz/<alias>` and reads that island for
the rating, address, hours, and contact detail. Reviews come from the page's
`review_feed` JSON endpoint, search reads the embedded state the search page
ships, autocomplete is the lighter `aselect` JSON endpoint, and a user profile is
lifted from the public `/user_details` page. It paces and caches requests, retries
the transient failures, and sends a browser user-agent because that is what a
logged-out reader looks like.

On the fusion plane, the same commands call the matching `api.yelp.com/v3`
endpoint (`businesses/{id}`, `businesses/search`, `businesses/{id}/reviews`,
`autocomplete`, `categories`, `categories/{alias}`) with the key in an
`Authorization: Bearer` header.

Prices, hours, and dates are read in whatever locale Yelp serves; set `--locale`
(default `en_US`) to ask for another.

## What anonymous access reaches

The web plane sits behind PerimeterX, which classifies a request before the
application sees it. This tool sorts the surfaces into what works regardless and
what is gated, and never pretends the line is elsewhere.

Works from any network, offline:

- `ref id`, `ref url` (pure string resolution, no request)

Reachable on the fusion plane from any network, best-effort on the web plane:

- `biz` (the `/biz/` JSON-LD island, or `businesses/{id}`)
- `search` (the search page state, or `businesses/search`)
- `reviews` (the `review_feed` endpoint, or `businesses/{id}/reviews`)
- `suggest` (the `aselect` autocomplete, or `autocomplete`)

Fusion plane only:

- `categories` and `category` (the `categories` endpoints; there is no clean web
  equivalent, so these need `YELP_API_KEY`)

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
rating and review count, price, categories, address and coordinates, distance
from a search center, hours, transactions, attributes, claim and closure state,
photos, and description; a review carries the rating, author and a link to the
author's profile, date, text, and the useful/funny/cool counts; a user carries
the public profile a visitor sees; a category carries its title, parents, and the
edges into a search and up to its parent. A field a surface does not show is left
empty rather than guessed.

When something is genuinely missing the exit code says which, so a script can tell
the cases apart:

| Exit | Meaning |
| --- | --- |
| 0 | ok |
| 2 | usage error |
| 3 | no results (the resource is genuinely empty) |
| 4 | need auth: the bot wall, a missing `YELP_API_KEY`, or a rejected key |
| 5 | rate limited (raise `--retries`, or the Fusion daily quota) |
| 6 | not found (unknown alias or user id, bad reference) |
| 8 | network error |

## Commands

| Command | What it does |
| --- | --- |
| `search <term> [location]` | Business search by term and place |
| `biz <alias>` | One business by alias |
| `reviews <alias>` | A business's reviews |
| `user <id>` | A reviewer's public profile (web plane only) |
| `suggest <prefix>` | Autocomplete suggestions for a prefix |
| `categories` | The Yelp category taxonomy (needs `YELP_API_KEY`) |
| `category <alias>` | One category by alias (needs `YELP_API_KEY`) |
| `ref id <ref>` | Classify a reference into its (kind, id), offline |
| `ref url <kind> <id>` | Build the canonical URL for a (kind, id), offline |
| `serve` | Serve the same operations over HTTP as NDJSON |
| `mcp` | Serve the same operations to an agent over MCP |
| `version` | Print version, commit, and build date |

A business is addressed by its alias, like `garaje-san-francisco`, or a `/biz/`
URL; a user by the id in a `/user_details?userid=` URL. Run `yelp <command>
--help` for the full flag list on any command.

## Output

Every command shares one output contract. The default adapts to where output
goes, a table on a terminal and JSONL in a pipe, so the same command reads well by
hand and parses cleanly downstream.

```sh
yelp search "tacos" "San Francisco, CA" -n 4 --fields name,rating,review_count,price
```

Pick the format with `-o table|json|jsonl|csv|tsv|url|raw`, choose columns with
`--fields a,b,c`, render a custom line with `--template`, drop the header with
`--no-header`, and cap results with `-n/--limit`. The `url` format prints just the
canonical URL of each record, which is handy for piping into another tool.

## Recipes

Resolve a pile of pasted links to their (kind, id) offline:

```sh
yelp ref id "https://www.yelp.com/biz/garaje-san-francisco" -o json | jq '{kind, id}'
```

A search as JSON, piped to jq (best-effort on web, reliable with a key):

```sh
yelp search "ramen" "Oakland, CA" -o json | jq '{alias, name, rating, review_count}'
```

The canonical URLs of a search, one per line:

```sh
yelp search "tacos" "San Francisco, CA" -n 50 -o url
```

Tee a business's reviews into a local SQLite store, keyed by each review id:

```sh
yelp reviews garaje-san-francisco --db yelp.db
```

## Serve it

The same operations are available over HTTP and as an MCP tool set for agents,
with no extra code:

```sh
yelp serve --addr :7777    # GET /v1/... returns NDJSON
yelp mcp                   # speak MCP over stdio
```

## Use it as a resource-URI driver

`yelp` registers a `yelp` domain the way a program registers a database driver
with `database/sql`. A host enables it with one blank import:

```go
import _ "github.com/tamnd/yelp-cli/yelp"
```

Then [ant](https://github.com/tamnd/ant) (or any program that links the package)
dereferences `yelp://` URIs without knowing anything about Yelp:

```sh
ant get yelp://biz/<alias>        # fetch a business
ant cat yelp://biz/<alias>        # just the description body
ant ls  yelp://reviews/<alias>    # a business's reviews, each addressable
ant get yelp://user/<id>          # fetch a reviewer's profile
ant get yelp://category/<alias>   # fetch one category, with its parent edge
ant url yelp://biz/<alias>        # the live https URL
```

Records carry explicit edges that close into one connected graph, so a host can
breadth-first crawl it and write it to disk: a suggestion fans out into a place
search, straight to a business, or into the category that names it; a search card
links to its full business; a business links to its reviews and a same-category
search; a review links back to its business and on to the reviewer's profile; a
category fans into a search and climbs to its parent. The business graph and the
category taxonomy are both fully connected, so a crawl started anywhere reaches
the rest of the reachable site. The one leaf is the user: the Fusion API has no
user endpoint and the web profile shows no clean reviews feed to a logged-out
reader, so a reviewer is a leaf rather than a fabricated edge. `ant export <uri>
--follow N` walks those edges. See the
[resource-URI guide](https://yelp-cli.tamnd.com/guides/resource-uris/) for the
full edge map.

## Development

```
cmd/yelp/   thin main: hands cli.NewApp to kit.Run
cli/        assembles the kit App from the yelp domain
yelp/       the library: the two-plane HTTP client, the JSON-LD and web-state
            parsers, the Fusion client, data models, and domain.go (the driver)
docs/       tago documentation site
```

```sh
make build      # ./bin/yelp
make test       # go test ./...
make vet        # go vet ./...
```

Every read command is declared once as a kit operation in `yelp/domain.go`. That
single declaration becomes the CLI subcommand, the HTTP route, and the MCP tool,
so the three surfaces never drift.

## Releasing

Push a version tag and GitHub Actions runs GoReleaser, which builds the archives,
Linux packages, the multi-arch GHCR image, checksums, SBOMs, and a cosign
signature:

```sh
git tag v0.1.0
git push --tags
```

The Homebrew and Scoop steps self-disable until their tokens exist, so the first
release works with no extra secrets.

## License

`yelp` is an independent tool and is not affiliated with Yelp. Apache-2.0, see
[LICENSE](LICENSE).
</content>
</invoke>
