package yelp

import "time"

// hostname is the Yelp host the web plane builds page URLs from and the host the
// URI driver in domain.go claims.
const hostname = "www.yelp.com"

// BaseURL is the root every web page and web JSON endpoint is built from.
const BaseURL = "https://" + hostname

// fusionBase is the root of Yelp's official Fusion API. The opt-in plane talks
// here with a bearer key; it is unwalled and reliable, unlike the web plane.
const fusionBase = "https://api.yelp.com"

// The two data planes. The web plane reads what a logged-out browser reads (the
// biz page JSON-LD island and embedded app state, the review_feed JSON endpoint,
// the autocomplete JSON, the search page) and sits behind Yelp's PerimeterX bot
// wall, which hard-blocks datacenter IPs (see errors.go). The fusion plane is the
// official api.yelp.com/v3 read API, addressed by a free developer key the
// operator supplies in YELP_API_KEY; it answers from any network. The client
// picks a plane per planeAuto unless --plane forces one.
const (
	planeAuto   = "auto"   // fusion when a key is set, else web
	planeWeb    = "web"    // force the anonymous web plane
	planeFusion = "fusion" // force the Fusion API plane (needs a key)
)

// DefaultUserAgent is sent with every web-plane request. Yelp serves its public
// pages to a normal browser, so a browser User-Agent is what keeps a logged-out
// reader looking like one. The fusion plane ignores it. Override with
// --user-agent.
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// Defaults for the polite client.
const (
	// DefaultDelay is the minimum gap between requests. Yelp's edge is touchy on
	// the web plane, so a one-second pace reads steadily without leaning on it;
	// the fusion plane has its own daily quota the operator owns.
	DefaultDelay    = 1 * time.Second
	DefaultRetries  = 3
	DefaultTimeout  = 30 * time.Second
	DefaultCacheTTL = 24 * time.Hour

	// defaultLocale localizes prices, hours, and review dates. Yelp calls it the
	// locale (language_COUNTRY); the fusion plane sends it as the locale param and
	// the web plane as Accept-Language.
	defaultLocale = "en_US"

	// defaultLimit is the page size for a search or a review list; 20 matches the
	// web grid and is the fusion search default.
	defaultLimit = 20
)

// Config carries the knobs the client reads. It is built from the kit framework
// config in ClientFromConfig, so a --rate or --timeout on the command line and
// the same value resolved by a host both land here.
type Config struct {
	UserAgent string
	Locale    string

	// Plane is one of planeAuto, planeWeb, or planeFusion.
	Plane string

	// APIKey is the Fusion developer key, read only from the YELP_API_KEY
	// environment variable. Empty leaves the fusion plane unavailable, so an
	// auto run uses the web plane. It is never a flag, so a key never lands in
	// shell history.
	APIKey string

	// Location scopes a search and an autocomplete to a place ("Oakland, CA"). A
	// search may instead be scoped by Latitude and Longitude.
	Location  string
	Latitude  float64
	Longitude float64

	// Sort orders a search: best_match, rating, review_count, or distance.
	Sort string
	// Price filters a search: any of 1,2,3,4 ("$" to "$$$$"), comma-joined.
	Price string
	// Radius narrows a fusion search to this many meters from the center (max
	// 40000); zero leaves it to Yelp.
	Radius int
	// CategoryFilter filters a fusion search to one or more category aliases,
	// comma-joined ("coffee,bakeries").
	CategoryFilter string
	// Attributes filters a fusion search to businesses carrying every listed
	// attribute, comma-joined ("hot_and_new,wheelchair_accessible").
	Attributes string
	// OpenNow filters a fusion search to businesses open at request time.
	OpenNow bool

	// Delay is the minimum gap between requests. Zero means no pacing.
	Delay   time.Duration
	Retries int
	Timeout time.Duration

	// BaseURL and FusionBase are the plane roots. Empty uses the public hosts;
	// tests point both at one httptest server.
	BaseURL    string
	FusionBase string

	// CacheDir is where responses are cached. Empty or NoCache disables it.
	CacheDir string
	CacheTTL time.Duration
	NoCache  bool
	// Refresh fetches fresh copies and rewrites the cache, ignoring any hit.
	Refresh bool
}

// DefaultConfig returns the baseline: a browser User-Agent, the en_US locale, the
// auto plane, a one-second pace, three retries, a 30s timeout, and a one-day
// cache.
func DefaultConfig() Config {
	return Config{
		UserAgent:  DefaultUserAgent,
		Locale:     defaultLocale,
		Plane:      planeAuto,
		Delay:      DefaultDelay,
		Retries:    DefaultRetries,
		Timeout:    DefaultTimeout,
		BaseURL:    BaseURL,
		FusionBase: fusionBase,
		CacheTTL:   DefaultCacheTTL,
	}
}
