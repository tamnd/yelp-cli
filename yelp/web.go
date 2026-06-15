package yelp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

// getJSON GETs a web-plane URL and unmarshals the JSON body into out. The
// underlying get already paces, caches, and checks for the bot wall.
func (c *Client) getJSON(ctx context.Context, u string, out any) error {
	body, err := c.get(ctx, u)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode %s: %w", u, err)
	}
	return nil
}

// web.go holds the helpers the web plane shares: pulling the schema.org JSON-LD
// island a biz page embeds, and the loose JSON-LD shape the biz parser reads from
// it. A biz page renders server-side and carries one or more
// <script type="application/ld+json"> blocks; the business one is a
// Restaurant/LocalBusiness object with the public detail.

// jsonLD is the loose schema.org shape a biz page embeds. It decodes only the
// fields Business needs; a page carries more under the same block.
type jsonLD struct {
	Type        string          `json:"@type"`
	Name        string          `json:"name"`
	Image       json.RawMessage `json:"image"` // a string or an array of strings
	Telephone   string          `json:"telephone"`
	PriceRange  string          `json:"priceRange"`
	Description string          `json:"description"`
	Address     struct {
		StreetAddress   string `json:"streetAddress"`
		AddressLocality string `json:"addressLocality"`
		AddressRegion   string `json:"addressRegion"`
		PostalCode      string `json:"postalCode"`
		AddressCountry  string `json:"addressCountry"`
	} `json:"address"`
	AggregateRating struct {
		RatingValue json.Number `json:"ratingValue"`
		ReviewCount json.Number `json:"reviewCount"`
	} `json:"aggregateRating"`
	Geo struct {
		Latitude  json.Number `json:"latitude"`
		Longitude json.Number `json:"longitude"`
	} `json:"geo"`
	OpeningHours              []string `json:"openingHours"`
	OpeningHoursSpecification []struct {
		DayOfWeek json.RawMessage `json:"dayOfWeek"` // string or array
		Opens     string          `json:"opens"`
		Closes    string          `json:"closes"`
	} `json:"openingHoursSpecification"`
}

// extractWebState returns the embedded application-state JSON the search and
// other pages render inline, picked as the first application/json script block
// that carries the searchPageProps marker. Yelp wraps these blobs in an HTML
// comment (<!-- ... -->) inside the script, so the wrapper is stripped and the
// HTML entities are unescaped before the JSON is handed back. ok is false when no
// such block is present (a wall stub, or an unexpected layout).
func extractWebState(body []byte) ([]byte, bool) {
	marker := []byte("searchPageProps")
	for _, b := range jsonScriptBlocks(body) {
		if bytes.Contains(b, marker) {
			return b, true
		}
	}
	return nil, false
}

// jsonScriptBlocks returns every application/json script block in an HTML body,
// with the HTML-comment wrapper stripped and entities unescaped.
func jsonScriptBlocks(body []byte) [][]byte {
	var out [][]byte
	marker := []byte(`application/json`)
	rest := body
	for {
		i := bytes.Index(rest, marker)
		if i < 0 {
			break
		}
		start := bytes.IndexByte(rest[i:], '>')
		if start < 0 {
			break
		}
		start += i + 1
		end := bytes.Index(rest[start:], []byte("</script>"))
		if end < 0 {
			break
		}
		raw := bytes.TrimSpace(rest[start : start+end])
		raw = bytes.TrimPrefix(raw, []byte("<!--"))
		raw = bytes.TrimSuffix(raw, []byte("-->"))
		out = append(out, htmlUnescape(bytes.TrimSpace(raw)))
		rest = rest[start+end:]
	}
	return out
}

// htmlUnescape reverses the few entities Yelp escapes in an embedded JSON blob.
func htmlUnescape(b []byte) []byte {
	repl := [][2]string{
		{"&quot;", `"`}, {"&#34;", `"`},
		{"&amp;", "&"}, {"&#38;", "&"},
		{"&lt;", "<"}, {"&#60;", "<"},
		{"&gt;", ">"}, {"&#62;", ">"},
		{"&#39;", "'"}, {"&#x27;", "'"},
	}
	for _, r := range repl {
		b = bytes.ReplaceAll(b, []byte(r[0]), []byte(r[1]))
	}
	return b
}

// jsonLDBlocks returns every application/ld+json block in an HTML body.
func jsonLDBlocks(body []byte) [][]byte {
	var out [][]byte
	marker := []byte(`application/ld+json`)
	rest := body
	for {
		i := bytes.Index(rest, marker)
		if i < 0 {
			break
		}
		start := bytes.IndexByte(rest[i:], '>')
		if start < 0 {
			break
		}
		start += i + 1
		end := bytes.Index(rest[start:], []byte("</script>"))
		if end < 0 {
			break
		}
		out = append(out, bytes.TrimSpace(rest[start:start+end]))
		rest = rest[start+end:]
	}
	return out
}

// firstBusinessLD returns the first JSON-LD block whose @type names a business.
// JSON-LD may be a single object or an array (an @graph), so each block is tried
// both ways.
func firstBusinessLD(body []byte) (*jsonLD, bool) {
	for _, b := range jsonLDBlocks(body) {
		if ld, ok := pickBusinessLD(b); ok {
			return ld, true
		}
	}
	return nil, false
}

func pickBusinessLD(b []byte) (*jsonLD, bool) {
	// A bare object.
	var one jsonLD
	if json.Unmarshal(b, &one) == nil && isBusinessType(one.Type) {
		return &one, true
	}
	// An array of objects.
	var many []jsonLD
	if json.Unmarshal(b, &many) == nil {
		for i := range many {
			if isBusinessType(many[i].Type) {
				return &many[i], true
			}
		}
	}
	// An @graph wrapper.
	var graph struct {
		Graph []jsonLD `json:"@graph"`
	}
	if json.Unmarshal(b, &graph) == nil {
		for i := range graph.Graph {
			if isBusinessType(graph.Graph[i].Type) {
				return &graph.Graph[i], true
			}
		}
	}
	return nil, false
}

// isBusinessType reports whether a schema.org @type is a business Yelp lists.
// Yelp tags biz pages with Restaurant most often, and LocalBusiness or a more
// specific subtype otherwise; any non-empty type that is not a Review or a
// BreadcrumbList is treated as the business object.
func isBusinessType(t string) bool {
	switch t {
	case "", "Review", "BreadcrumbList", "WebSite", "Organization", "ListItem", "AggregateRating":
		return false
	default:
		return true
	}
}

// ldImages reads the JSON-LD image field, which Yelp serves as either a single
// URL string or an array of URL strings.
func ldImages(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var one string
	if json.Unmarshal(raw, &one) == nil && one != "" {
		return []string{one}
	}
	var many []string
	if json.Unmarshal(raw, &many) == nil {
		var out []string
		for _, s := range many {
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// ldDays reads a dayOfWeek field, which is a single day string or an array.
func ldDays(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var one string
	if json.Unmarshal(raw, &one) == nil && one != "" {
		return []string{shortDay(one)}
	}
	var many []string
	if json.Unmarshal(raw, &many) == nil {
		var out []string
		for _, s := range many {
			if s != "" {
				out = append(out, shortDay(s))
			}
		}
		return out
	}
	return nil
}

// shortDay turns a schema.org day ("https://schema.org/Monday" or "Monday")
// into a three-letter label.
func shortDay(s string) string {
	if i := lastSlash(s); i >= 0 {
		s = s[i+1:]
	}
	if len(s) >= 3 {
		return s[:3]
	}
	return s
}

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}
