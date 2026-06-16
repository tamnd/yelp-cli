// Package yelp is the library behind the yelp command line: a two-plane HTTP
// client for Yelp's public data, and the typed records every command emits.
//
// There are two ways into Yelp's public data, and this client speaks both. The
// web plane reads what a logged-out browser reads: a biz page embeds its detail
// as a schema.org JSON-LD island, reviews come from the review_feed JSON
// endpoint, autocomplete from the suggest endpoint, search from the search page.
// Yelp fronts the web site with a PerimeterX bot wall that classifies a request
// on IP reputation and TLS fingerprint and hard-walls datacenter IPs, so the web
// plane is best-effort: a wall returns ErrBlocked (exit 4). The fusion plane is
// Yelp's official Fusion API at api.yelp.com/v3, addressed with a bearer key the
// operator supplies in YELP_API_KEY (a free developer key); it answers from any
// network. The client uses fusion when a key is present and web otherwise, or the
// plane --plane forces. This client does not forge a TLS fingerprint and does not
// rent or rotate IPs to get past the web edge: it reads what a logged-out browser
// reads, the way it reads it, and offers the fusion plane as the honest reliable
// path. Each surface lives in its own file (search.go, biz.go, reviews.go,
// user.go, suggest.go, categories.go) with both plane implementations and the
// record mapping; this file holds the shared client and fusion.go the bearer
// transport.
package yelp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Client talks to Yelp over both planes. It paces requests, retries the
// transient failures, detects the bot wall, and caches response bodies on disk
// keyed by the request.
type Client struct {
	HTTP           *http.Client
	BaseURL        string // web plane root
	FusionBase     string // fusion plane root
	UserAgent      string
	Locale         string
	Plane          string
	Location       string
	Latitude       float64
	Longitude      float64
	Sort           string
	Price          string
	Radius         int
	CategoryFilter string
	Attributes     string
	OpenNow        bool
	Delay          time.Duration
	Retries        int

	apiKey string // the Fusion bearer key, from YELP_API_KEY; empty leaves fusion unavailable

	cache   *cache
	refresh bool

	mu   sync.Mutex
	last time.Time
}

// NewClient builds a client from cfg.
func NewClient(cfg Config) *Client {
	c := &Client{
		HTTP:           &http.Client{Timeout: cfg.Timeout},
		BaseURL:        cfg.BaseURL,
		FusionBase:     cfg.FusionBase,
		UserAgent:      cfg.UserAgent,
		Locale:         cfg.Locale,
		Plane:          cfg.Plane,
		Location:       cfg.Location,
		Latitude:       cfg.Latitude,
		Longitude:      cfg.Longitude,
		Sort:           cfg.Sort,
		Price:          cfg.Price,
		Radius:         cfg.Radius,
		CategoryFilter: cfg.CategoryFilter,
		Attributes:     cfg.Attributes,
		OpenNow:        cfg.OpenNow,
		Delay:          cfg.Delay,
		Retries:        cfg.Retries,
		apiKey:         cfg.APIKey,
		refresh:        cfg.Refresh,
	}
	if c.BaseURL == "" {
		c.BaseURL = BaseURL
	}
	if c.FusionBase == "" {
		c.FusionBase = fusionBase
	}
	if c.UserAgent == "" {
		c.UserAgent = DefaultUserAgent
	}
	if c.Locale == "" {
		c.Locale = defaultLocale
	}
	if c.Plane == "" {
		c.Plane = planeAuto
	}
	if !cfg.NoCache {
		c.cache = newCache(cfg.CacheDir, cfg.CacheTTL)
	}
	return c
}

// usesFusion reports whether a call should take the fusion plane. Auto picks
// fusion only when a key is set; web and fusion force the choice.
func (c *Client) usesFusion() bool {
	switch c.Plane {
	case planeFusion:
		return true
	case planeWeb:
		return false
	default:
		return c.apiKey != ""
	}
}

// requireFusion returns the reason the fusion plane is unavailable for a call
// that needs it, or nil.
func (c *Client) requireFusion() error {
	if c.apiKey == "" {
		return ErrNeedKey
	}
	return nil
}

// get fetches a web-plane URL and returns the response body: paced, retried,
// cached, and wall-checked.
func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	ck := "web:" + url
	if !c.refresh {
		if b, ok := c.cache.get(ck); ok {
			return b, nil
		}
	}
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, http.MethodGet, url, nil, nil)
		if err == nil {
			c.cache.put(ck, body)
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, lastErr
}

// do performs one request and returns the body. retry reports whether the
// failure is worth another attempt. header, when non-nil, is applied to the
// request; reqBody, when non-nil, is the request payload.
func (c *Client) do(ctx context.Context, method, url string, header http.Header, reqBody []byte) (body []byte, retry bool, err error) {
	c.pace()
	var rdr io.Reader
	if reqBody != nil {
		rdr = bytes.NewReader(reqBody)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json")
	req.Header.Set("Accept-Language", localeToAcceptLanguage(c.Locale))
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Set(k, v)
		}
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		// A connection reset mid-handshake is how the edge sometimes drops a
		// datacenter request; treat a transport error as retryable.
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusOK:
		// fall through to read and check the body
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, true, ErrRateLimited
	case resp.StatusCode == http.StatusUnauthorized:
		// A 401 on a fusion request is a rejected key; on the web plane it reads
		// as the wall.
		if header != nil && header.Get("Authorization") != "" {
			return nil, false, ErrKeyRejected
		}
		return nil, false, ErrBlocked
	case resp.StatusCode == http.StatusForbidden:
		return nil, false, ErrBlocked
	case resp.StatusCode == http.StatusNotFound, resp.StatusCode == http.StatusGone:
		return nil, false, ErrNotFound
	case resp.StatusCode >= 500:
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	default:
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	if isChallenge(b) {
		return nil, false, ErrBlocked
	}
	return b, false, nil
}

// isChallenge reports whether a 200 body is in fact the PerimeterX bot wall,
// which the edge sometimes serves with a 200 status, or an empty body where a
// real page would be large. The markers are the PerimeterX captcha host and the
// block-page text.
func isChallenge(b []byte) bool {
	if len(bytes.TrimSpace(b)) == 0 {
		return true
	}
	return bytes.Contains(b, []byte("px-captcha")) ||
		bytes.Contains(b, []byte("captcha.px-cloud.net")) ||
		bytes.Contains(b, []byte("_pxhd")) ||
		bytes.Contains(b, []byte("Please verify you are a human"))
}

// pace blocks until at least Delay has passed since the previous request.
func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Delay <= 0 {
		c.last = time.Now()
		return
	}
	if wait := c.Delay - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// localeToAcceptLanguage turns a Yelp locale ("en_US") into an Accept-Language
// header value ("en-US,en;q=0.9").
func localeToAcceptLanguage(locale string) string {
	if locale == "" {
		return "en-US,en;q=0.9"
	}
	tag := strings.ReplaceAll(locale, "_", "-")
	lang := tag
	if i := strings.IndexByte(tag, '-'); i > 0 {
		lang = tag[:i]
	}
	return tag + "," + lang + ";q=0.9"
}

// ClearCache removes the on-disk cache.
func (c *Client) ClearCache() error { return c.cache.clear() }
