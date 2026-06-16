package yelp

import (
	"context"
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes yelp as a kit Domain: a driver that a multi-domain host (ant)
// enables with a single blank import,
//
//	import _ "github.com/tamnd/yelp-cli/yelp"
//
// exactly as a database/sql program enables a driver with `import _
// "github.com/lib/pq"`. The init below registers it; the host then dereferences
// yelp:// URIs by routing to the operations Register installs. The same Domain
// also builds the standalone yelp binary (see cli.NewApp), so the binary and a
// host share one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the yelp driver. It carries no state; the per-run client is built by
// the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against, and
// the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme:   "yelp",
		Hosts:    []string{hostname, "yelp.com"},
		Identity: Identity(),
	}
}

// Identity is the fixed description of the yelp CLI, shared by the domain and the
// standalone composition root so help and version read the same everywhere.
func Identity() kit.Identity {
	return kit.Identity{
		Binary: "yelp",
		Short:  "Read public Yelp businesses, reviews, users, suggestions, and categories into structured records",
		Long: `yelp reads public Yelp data over two planes. The default web
plane reads what a logged-out browser reads (a business page, its
reviews, search, and autocomplete) and sits behind Yelp's PerimeterX bot
wall, which hard-walls datacenter IPs, so those reads are best-effort and
a wall returns exit 4. The opt-in fusion plane is Yelp's official Fusion
API at api.yelp.com/v3, addressed with a free developer key you supply in
the YELP_API_KEY environment variable; it answers from any network. The
client uses fusion when a key is set and the web plane otherwise, or the
plane --plane forces. It returns records as a table, JSON, JSONL, CSV,
TSV, or URLs, and serves the same operations over HTTP and MCP.

yelp is an independent tool and is not affiliated with Yelp.`,
		Site: BaseURL,
		Repo: "https://github.com/tamnd/yelp-cli",
	}
}

// Register installs the client factory and every operation onto app. A resolver
// op (Single) names its own record type and answers `ant get`; a List op
// enumerates a parent resource's members and answers `ant ls`. Each list op names
// its own collection authority, distinct from the biz and user resolvers, so
// yelp://search/<term>, yelp://reviews/<alias>, yelp://categories, and
// yelp://suggest/<prefix> each reach the right op rather than shadowing one
// another.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)
	app.CommandGroup("read", "Read public Yelp data")
	app.CommandGroup("ref", "Resolve references to ids and URLs (offline)")

	kit.Handle(app, kit.OpMeta{
		Name: "search", Group: "read", List: true,
		Summary: "Search businesses by term and place",
		URIType: "search",
		Args: []kit.Arg{
			{Name: "term", Help: "what to search for"},
			{Name: "location", Help: "a place to search in", Optional: true},
		},
	}, search)

	kit.Handle(app, kit.OpMeta{
		Name: "biz", Group: "read", Single: true,
		Summary: "Show one business by alias",
		URIType: "biz", Resolver: true,
		Args: []kit.Arg{{Name: "alias", Help: "business alias or /biz/ URL"}},
	}, getBiz)

	kit.Handle(app, kit.OpMeta{
		Name: "reviews", Group: "read", List: true,
		Summary: "List a business's reviews",
		URIType: "reviews",
		Args:    []kit.Arg{{Name: "alias", Help: "business alias or /biz/ URL"}},
	}, reviews)

	kit.Handle(app, kit.OpMeta{
		Name: "user", Group: "read", Single: true,
		Summary: "Show a reviewer's public profile (web plane only)",
		URIType: "user", Resolver: true,
		Args: []kit.Arg{{Name: "id", Help: "user id or /user_details URL"}},
	}, getUser)

	kit.Handle(app, kit.OpMeta{
		Name: "suggest", Group: "read", List: true,
		Summary: "Autocomplete suggestions for a prefix",
		URIType: "suggest",
		Args:    []kit.Arg{{Name: "prefix", Help: "the typed prefix"}},
	}, suggest)

	kit.Handle(app, kit.OpMeta{
		Name: "categories", Group: "read", List: true,
		Summary: "List the Yelp category taxonomy (needs YELP_API_KEY)",
		URIType: "categories",
	}, categories)

	kit.Handle(app, kit.OpMeta{
		Name: "category", Group: "read", Single: true,
		Summary: "Show one category by alias (needs YELP_API_KEY)",
		URIType: "category", Resolver: true,
		Args: []kit.Arg{{Name: "alias", Help: "category alias, e.g. \"coffee\""}},
	}, getCategory)

	// Reference tools (offline).
	kit.Handle(app, kit.OpMeta{
		Name: "id", Parent: "ref", Single: true,
		Summary: "Classify a reference into its (kind, id)",
		Args:    []kit.Arg{{Name: "ref", Help: "any Yelp URL, path, alias, or id"}},
	}, classifyRef)

	kit.Handle(app, kit.OpMeta{
		Name: "url", Parent: "ref", Single: true,
		Summary: "Build the canonical URL for a (kind, id)",
		Args: []kit.Arg{
			{Name: "kind", Help: "biz, user, or category"},
			{Name: "id", Help: "the id for that kind"},
		},
	}, buildURL)
}

// newClient builds the client from the host-resolved config, so a host and the
// standalone binary pace and identify themselves the same way.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	return ClientFromConfig(cfg), nil
}

// ClientFromConfig maps the framework config onto a yelp.Config and returns a
// client. The Fusion key is read only from the YELP_API_KEY environment variable,
// never a flag, so a key never lands in shell history; everything else comes from
// the resolved flags.
func ClientFromConfig(cfg kit.Config) *Client {
	yc := DefaultConfig()
	if cfg.Rate > 0 {
		yc.Delay = cfg.Rate
	}
	if cfg.Retries >= 0 {
		yc.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		yc.Timeout = cfg.Timeout
	}
	if ua := cfg.Extra["user-agent"]; ua != "" {
		yc.UserAgent = ua
	} else if cfg.UserAgent != "" {
		yc.UserAgent = cfg.UserAgent
	}
	if v := cfg.Extra["locale"]; v != "" {
		yc.Locale = v
	}
	if v := cfg.Extra["location"]; v != "" {
		yc.Location = v
	}
	if v := cfg.Extra["sort"]; v != "" {
		yc.Sort = v
	}
	if v := cfg.Extra["price"]; v != "" {
		yc.Price = v
	}
	if v := cfg.Extra["radius"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			yc.Radius = n
		}
	}
	if v := cfg.Extra["categories"]; v != "" {
		yc.CategoryFilter = v
	}
	if v := cfg.Extra["attributes"]; v != "" {
		yc.Attributes = v
	}
	yc.OpenNow = cfg.Extra["open-now"] == "true"
	if v := cfg.Extra["plane"]; v != "" {
		yc.Plane = v
	}
	// The Fusion key is environment-only. YELP_API_KEY is read directly so it is
	// honored whether the binary or a host built the config.
	yc.APIKey = os.Getenv("YELP_API_KEY")
	yc.CacheDir = cfg.CacheDir
	yc.NoCache = cfg.NoCache
	if ttl := cfg.Extra["cache-ttl"]; ttl != "" {
		if d, err := time.ParseDuration(ttl); err == nil {
			yc.CacheTTL = d
		}
	}
	yc.Refresh = cfg.Extra["refresh"] == "true"
	return NewClient(yc)
}

// Defaults seeds the framework baseline with yelp's own values, so an unset
// --rate or --timeout uses the yelp default rather than the generic kit one.
func Defaults(c *kit.Config) {
	def := DefaultConfig()
	c.Rate = def.Delay
	c.Retries = def.Retries
	c.Timeout = def.Timeout
	c.UserAgent = def.UserAgent
}

// Classify turns any accepted input into the canonical (type, id), so `ant
// resolve` and `ant url` touch no network.
func (Domain) Classify(input string) (uriType, id string, err error) {
	r := Classify(input)
	if r.Kind == "unknown" {
		return "", "", errs.Usage("unrecognized yelp reference: %q", input)
	}
	return r.Kind, r.ID, nil
}

// Locate is the inverse: the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	u := URLFor(uriType, id)
	if u == "" {
		return "", errs.Usage("yelp has no resource type %q", uriType)
	}
	return u, nil
}

// mapErr translates a library error into a kit error so the exit code matches the
// rest of the fleet: a missing entity reads as "not found" (exit 6), a throttle
// as "rate limited" (exit 5), and the bot wall, a missing key, or a rejected key
// as "need auth" (exit 4).
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return errs.NotFound("%s", err.Error())
	case errors.Is(err, ErrRateLimited):
		return errs.RateLimited("%s", err.Error())
	case errors.Is(err, ErrNeedLocation):
		return errs.Usage("%s", err.Error())
	case errors.Is(err, ErrBlocked), errors.Is(err, ErrNeedKey), errors.Is(err, ErrKeyRejected):
		return errs.NeedAuth("%s", err.Error())
	default:
		return err
	}
}
