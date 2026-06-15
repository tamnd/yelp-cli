package yelp

import (
	"regexp"
	"strconv"
	"strings"
)

// html.go holds the small shared helpers the surfaces lean on. Yelp's primary
// modes are JSON (the Fusion API, the review_feed and autocomplete endpoints) and
// the JSON-LD island a biz page embeds, so most fields arrive already typed;
// these helpers clean the free text and read the few values that arrive as
// display strings (counts, ratings).

var (
	tagRE = regexp.MustCompile(`<[^>]+>`)
	intRE = regexp.MustCompile(`[0-9][0-9,]*`)
)

// stripHTML reduces an HTML fragment to its text, collapsing whitespace.
func stripHTML(s string) string {
	return squish(tagRE.ReplaceAllString(s, " "))
}

// squish collapses runs of whitespace into single spaces and trims the ends, so
// a value lifted from indented markup reads cleanly.
func squish(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// firstInt returns the first whole number in s (commas dropped), or 0.
func firstInt(s string) int {
	m := intRE.FindString(s)
	if m == "" {
		return 0
	}
	n, _ := strconv.Atoi(strings.ReplaceAll(m, ",", ""))
	return n
}

// dollars turns a Fusion or web price token into the "$"-string Yelp prints. A
// price already in dollar signs passes through; a "1".."4" maps to its run of
// signs.
func dollars(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "$") {
		return s
	}
	if n, err := strconv.Atoi(s); err == nil && n >= 1 && n <= 4 {
		return strings.Repeat("$", n)
	}
	return s
}
