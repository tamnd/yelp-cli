// Package cli assembles the yelp command tree from the yelp
// domain on top of the any-cli/kit framework.
package cli

import (
	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/yelp-cli/yelp"
)

// Build metadata, set via -ldflags at release time.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// builder holds the domain-global flags while the app is assembled, then folds
// them onto the resolved config in finalize, using the exact keys
// ClientFromConfig reads.
type builder struct {
	userAgent  string
	locale     string
	location   string
	sort       string
	price      string
	radius     string
	categories string
	attributes string
	openNow    bool
	plane      string
	cacheTTL   string
	refresh    bool
}

// NewApp assembles the kit application from the yelp domain. The domain's
// Register installs the client factory and every operation, so the binary and a
// host (ant, which blank-imports the package) share one source of truth. This
// package adds the domain-global flags and the version command; kit.Run turns the
// App into the CLI, plus the serve and mcp surfaces and the typed-error-to-exit-
// code mapping.
//
// To add a command, declare it in yelp/domain.go with kit.Handle and it appears
// here automatically. Reach for app.AddCommand only for a verb that does not fit
// the emit-records shape, the way version does below.
func NewApp() *kit.App {
	b := &builder{}
	id := yelp.Identity()
	id.Version = Version

	app := kit.New(id, kit.WithDefaults(yelp.Defaults))
	app.GlobalFlags(b.globals)
	app.Finalize(b.finalize)

	yelp.Domain{}.Register(app)
	app.AddCommand(newVersionCmd())
	return app
}

func (b *builder) globals(f *kit.FlagSet) {
	f.StringVar(&b.userAgent, "user-agent", yelp.DefaultUserAgent, "User-Agent sent with each web-plane request")
	f.StringVar(&b.locale, "locale", "", "locale for prices, hours, and dates (default en_US)")
	f.StringVar(&b.location, "location", "", "place to scope a search or autocomplete to, e.g. \"Oakland, CA\"")
	f.StringVar(&b.sort, "sort", "", "search order: best_match, rating, review_count, or distance")
	f.StringVar(&b.price, "price", "", "search price filter: any of 1,2,3,4 comma-joined")
	f.StringVar(&b.radius, "radius", "", "fusion search radius in meters from the center (max 40000)")
	f.StringVar(&b.categories, "categories", "", "fusion search category filter: one or more aliases comma-joined")
	f.StringVar(&b.attributes, "attributes", "", "fusion search attribute filter: one or more aliases comma-joined")
	f.BoolVar(&b.openNow, "open-now", false, "fusion search filter: only businesses open now")
	f.StringVar(&b.plane, "plane", "", "data plane: auto, web, or fusion (default auto; fusion needs YELP_API_KEY)")
	f.StringVar(&b.cacheTTL, "cache-ttl", yelp.DefaultCacheTTL.String(), "how long a cached response stays fresh")
	f.BoolVar(&b.refresh, "refresh", false, "fetch fresh copies and rewrite the cache, ignoring any hit")
}

func (b *builder) finalize(c *kit.Config) {
	if c.Extra == nil {
		c.Extra = map[string]string{}
	}
	set := func(k, v string) {
		if v != "" {
			c.Extra[k] = v
		}
	}
	set("user-agent", b.userAgent)
	set("locale", b.locale)
	set("location", b.location)
	set("sort", b.sort)
	set("price", b.price)
	set("radius", b.radius)
	set("categories", b.categories)
	set("attributes", b.attributes)
	set("plane", b.plane)
	set("cache-ttl", b.cacheTTL)
	if b.openNow {
		c.Extra["open-now"] = "true"
	}
	if b.refresh {
		c.Extra["refresh"] = "true"
	}
}
