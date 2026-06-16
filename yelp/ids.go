package yelp

import (
	"net/url"
	"strings"
)

// ids.go resolves a reference to a (kind, id) pair and builds canonical URLs, all
// offline. It backs `yelp ref id` and `yelp ref url`, and the Resolver the ant
// host calls to turn a yelp:// URI into the right command. Yelp addresses a
// business by a human alias (the /biz/<alias> slug), a user by an opaque id (the
// userid query parameter or the /user/<id> path), and a category by its alias
// (the cflt search parameter), so the resolver kinds are biz, user, and category.

// Classify reads a reference (a URL, a path, or a bare id/alias) and reports what
// it points at. Kind is one of biz, user, category, or unknown. A category is
// recognized only from a cflt parameter; a bare slug stays a business, since a
// business alias and a category alias share a shape and the business is the
// common case.
func Classify(ref string) Ref {
	in := strings.TrimSpace(ref)
	r := Ref{Input: in, Kind: "unknown"}
	if in == "" {
		return r
	}

	// A path or URL: read the leading segment.
	if looksLikePath(in) {
		segs := splitSegs(refPath(in))
		switch {
		case len(segs) >= 2 && segs[0] == "biz":
			r.Kind, r.ID = "biz", segs[1]
		case len(segs) >= 2 && segs[0] == "user":
			r.Kind, r.ID = "user", segs[1]
		case segs != nil && segs[0] == "user_details":
			if uid := queryParam(in, "userid"); uid != "" {
				r.Kind, r.ID = "user", uid
			}
		case segs != nil && segs[0] == "search":
			if cflt := queryParam(in, "cflt"); cflt != "" {
				r.Kind, r.ID = "category", cflt
			}
		}
		if r.Kind != "unknown" {
			r.URL = URLFor(r.Kind, r.ID)
			return r
		}
	}

	// A 22-character opaque token is a Yelp encid (a business or user id); without
	// more context it most often names a business.
	if isEncID(in) {
		r.Kind, r.ID = "biz", in
		r.URL = URLFor(r.Kind, r.ID)
		return r
	}

	// A bare slug is a business alias.
	if isAlias(in) {
		r.Kind, r.ID = "biz", in
		r.URL = URLFor(r.Kind, r.ID)
		return r
	}
	return r
}

// URLFor builds the canonical Yelp URL for a (kind, id) pair.
func URLFor(kind, id string) string {
	switch kind {
	case "biz":
		return BaseURL + "/biz/" + id
	case "user":
		return BaseURL + "/user_details?userid=" + url.QueryEscape(id)
	case "category":
		return BaseURL + "/search?cflt=" + url.QueryEscape(id)
	default:
		return ""
	}
}

// bizAlias reduces a reference to its business alias (or encid), so the surfaces
// accept a bare alias, a /biz/<alias> path, or a full URL.
func bizAlias(ref string) string {
	r := Classify(ref)
	if r.Kind == "biz" {
		return r.ID
	}
	return strings.Trim(ref, "/")
}

// userID reduces a reference to its user id.
func userID(ref string) string {
	r := Classify(ref)
	if r.Kind == "user" {
		return r.ID
	}
	return strings.Trim(ref, "/")
}

// userIDFromURL pulls the user id out of a profile URL (.../user_details?userid=X
// or .../user/X), falling back to fallback when the URL carries none.
func userIDFromURL(profileURL, fallback string) string {
	if profileURL != "" {
		if uid := queryParam(profileURL, "userid"); uid != "" {
			return uid
		}
		segs := splitSegs(refPath(profileURL))
		if len(segs) >= 2 && segs[0] == "user" {
			return segs[1]
		}
	}
	return fallback
}

// looksLikePath reports whether ref is a URL or a rooted path rather than a bare
// alias.
func looksLikePath(ref string) bool {
	return strings.Contains(ref, "://") || strings.HasPrefix(ref, "/") ||
		strings.HasPrefix(ref, "biz/") || strings.HasPrefix(ref, "user/") ||
		strings.HasPrefix(ref, "user_details")
}

// refPath reduces a reference to a site path: a full URL loses scheme, host, and
// query; a bare path is returned trimmed of its query.
func refPath(ref string) string {
	if i := strings.Index(ref, "://"); i >= 0 {
		rest := ref[i+3:]
		if s := strings.IndexByte(rest, '/'); s >= 0 {
			rest = rest[s:]
		} else {
			return "/"
		}
		ref = rest
	}
	if q := strings.IndexByte(ref, '?'); q >= 0 {
		ref = ref[:q]
	}
	if !strings.HasPrefix(ref, "/") {
		ref = "/" + ref
	}
	return ref
}

// queryParam returns the value of a query parameter in a URL or path, or "".
func queryParam(ref, key string) string {
	q := strings.IndexByte(ref, '?')
	if q < 0 {
		return ""
	}
	vals, err := url.ParseQuery(ref[q+1:])
	if err != nil {
		return ""
	}
	return vals.Get(key)
}

func splitSegs(path string) []string {
	var out []string
	for _, s := range strings.Split(path, "/") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// isEncID reports whether s is a Yelp opaque id: 22 url-safe base64 characters.
func isEncID(s string) bool {
	if len(s) != 22 {
		return false
	}
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

// isAlias reports whether s is a plausible business alias: lowercase letters,
// digits, and hyphens, the shape Yelp slugs use.
func isAlias(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-':
		default:
			return false
		}
	}
	return true
}
