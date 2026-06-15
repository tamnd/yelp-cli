---
title: "yelp"
description: "A command line for Yelp."
heroTitle: "yelp, from the command line"
heroLead: "A command line for Yelp. One pure-Go binary, no login, output that pipes into the rest of your tools, and a resource-URI driver other programs can address. The official Fusion API is opt-in through a free key."
heroPrimaryURL: "/getting-started/quick-start/"
heroPrimaryText: "Get started"
---

`yelp` reads public Yelp data over plain HTTPS, shapes it into clean records, and
gets out of your way.

```bash
yelp search "tacos" "San Francisco, CA"     # business search by term and place
yelp biz garaje-san-francisco               # one business by alias
yelp reviews garaje-san-francisco           # a business's reviews
yelp serve --addr :7777                     # the same operations over HTTP
```

There is no login and nothing to run alongside it. Output adapts to where it
goes: an aligned table on your terminal, JSONL the moment you pipe it somewhere.

## Two planes

The default web plane reads what a logged-out browser reads, behind Yelp's
PerimeterX bot wall, so those reads are best-effort. The opt-in fusion plane is
Yelp's official Fusion API, addressed with a free developer key you set in
`YELP_API_KEY`; it answers from any network. See the
[introduction](/getting-started/introduction/) for the full split.

## Two ways to use it

- **As a command** for reading Yelp by hand or in a script. Start with the
  [quick start](/getting-started/quick-start/).
- **As a resource-URI driver** so a host like
  [ant](https://github.com/tamnd/ant) can address Yelp as `yelp://` URIs and
  follow edges across sites. See [resource URIs](/guides/resource-uris/).

Both are the same code: one operation, declared once, is a CLI command, an HTTP
route, an MCP tool, and a URI dereference.

## Where to go next

- New here? Read the [introduction](/getting-started/introduction/), then the
  [quick start](/getting-started/quick-start/).
- Installing? See [installation](/getting-started/installation/).
- Doing a specific job? The [guides](/guides/) are task-first.
- Need every flag? The [CLI reference](/reference/cli/) is the full surface.
</content>
