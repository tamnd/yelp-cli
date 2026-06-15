---
title: "Troubleshooting"
description: "The handful of things that trip people up, and how to fix each one."
weight: 40
---

Most of these come down to network reality or how Yelp serves its data, not a
bug. The exit code tells the cases apart.

## Exit codes

| Exit | Meaning |
| --- | --- |
| 0 | ok |
| 2 | usage error |
| 3 | no results (the resource is genuinely empty) |
| 4 | need auth: the bot wall, a missing `YELP_API_KEY`, or a rejected key |
| 5 | rate limited (raise `--retries`, or the Fusion daily quota) |
| 6 | not found (unknown alias or user id, bad reference) |
| 8 | network error |

## A web-plane read exits 4

The web plane sits behind Yelp's PerimeterX bot manager, which classifies a
request on IP reputation and TLS fingerprint and hard-walls datacenter IPs. From a
home or mobile network the web surfaces usually answer; from a datacenter or CI
they hit the wall and exit 4. This tool does not forge a TLS fingerprint and does
not rent or rotate IPs to get past the edge. The two remedies it names are to run
from a residential or mobile network, or to set `YELP_API_KEY` and use the Fusion
API, which answers from any network:

```bash
export YELP_API_KEY=...     # a free Yelp developer key
yelp biz garaje-san-francisco
```

## A command says it needs a key

`categories` is fusion plane only (there is no clean web equivalent), and
`--plane fusion` needs a key. Both exit 4 when `YELP_API_KEY` is unset. Set the
key, or drop `--plane fusion` to let the web plane try. A key that is set but
rejected (a 401 from a missing, malformed, or revoked key) also exits 4; check the
key value.

## Requests start returning 429

`yelp` already paces requests and retries the transient failures, but a hard limit
still means backing off. Raise the delay between requests with `--rate` (for
example `--rate 1s`) and retry later. On the fusion plane a sustained 429 is the
Fusion daily quota; slow down or raise the quota in the developer portal.

## Nothing is found for something you expected

The public surface is not the whole site, and the web plane only carries what a
page actually shows. Check that the alias is spelled the way Yelp uses it (the
slug in `/biz/<alias>`), try a broader search term or a different location, and
remember that the web `reviews` feed pages while the fusion plane returns only
three review excerpts per business.

## The binary is not on your PATH

`go install` puts the binary in `$(go env GOPATH)/bin` (usually `~/go/bin`), and
a release archive leaves it wherever you unpacked it. If your shell cannot find
`yelp`, add that directory to your `PATH`. See
[installation](/getting-started/installation/).

## Seeing what yelp actually did

When something behaves unexpectedly, `-v` adds per-request detail so you can see
the URLs it hit and the responses it got. That is usually enough to tell a rate
limit apart from a genuinely empty result.
