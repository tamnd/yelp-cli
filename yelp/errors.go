package yelp

import "errors"

// The library reports its outcomes as a few sentinel errors. domain.go's mapErr
// translates each into the kit error kind that carries the matching exit code, so
// the standalone binary and a host agree on what a wall, a throttle, a missing
// key, and a miss mean.
var (
	// ErrNotFound is a missing entity: an unknown business alias or user id. Yelp
	// serves these as a 404 on the web plane and as a not-found envelope on the
	// fusion plane. Exit code 6.
	ErrNotFound = errors.New("not found")

	// ErrRateLimited is a sustained HTTP 429 after the client's own retries. On
	// the fusion plane it is the daily quota; slow down or raise the quota. Exit
	// code 5.
	ErrRateLimited = errors.New("rate limited")

	// ErrBlocked is Yelp's PerimeterX bot wall on the web plane: the edge
	// classifies a request on IP reputation and TLS fingerprint and answers with a
	// 403 or a challenge body before the application sees it, and hard-walls
	// datacenter IPs. The data is public and needs no account, so the message
	// names the real remedies: a residential or mobile connection, or the
	// reliable fusion plane via a free YELP_API_KEY. This tool does not forge a
	// TLS fingerprint or rent an IP to get past the edge. Exit code 4.
	ErrBlocked = errors.New("blocked by Yelp's bot wall (the data is public and needs no account; " +
		"retry from a residential or mobile connection, or set YELP_API_KEY to use the Fusion API)")

	// ErrNeedKey is the fusion plane without a key: either --plane fusion was
	// asked for with no YELP_API_KEY set, or a fusion-only surface (categories)
	// was reached on the web plane. The remedy is a free developer key in
	// YELP_API_KEY. Exit code 4.
	ErrNeedKey = errors.New("this needs a Yelp Fusion API key (set YELP_API_KEY; a developer key is free)")

	// ErrKeyRejected is a fusion request the API refused: a 401 from a missing,
	// malformed, or revoked key. Exit code 4.
	ErrKeyRejected = errors.New("the Yelp Fusion API key was rejected (check YELP_API_KEY)")

	// ErrNeedLocation is a fusion search with neither a place nor a coordinate to
	// scope it. The Fusion search endpoint requires one or the other, so the
	// client refuses the call rather than letting the API reject it as a bad
	// request. The remedy is a --location or a latitude and longitude. Exit code 2.
	ErrNeedLocation = errors.New("a fusion search needs a place: pass a location argument or set --location (or a latitude and longitude)")
)
