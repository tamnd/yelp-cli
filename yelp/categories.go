package yelp

import (
	"context"
	"net/url"
	"strings"
)

// categories.go reads Yelp's category taxonomy: the aliases a search filters on,
// each with its title and parents. This is a Fusion-backed surface; the web site
// has no clean category JSON, so on the web plane it reports ErrNeedKey rather
// than guessing. The optional locale narrows the list to categories valid in that
// country.

// Categories returns the category taxonomy, up to limit, optionally narrowed to
// the locale's country.
func (c *Client) Categories(ctx context.Context, limit int) ([]*Category, error) {
	if !c.usesFusion() {
		if err := c.requireFusion(); err != nil {
			return nil, err
		}
	}
	return c.fusionCategories(ctx, limit)
}

// fusionCategoriesResp is the GET /v3/categories shape.
type fusionCategoriesResp struct {
	Categories []struct {
		Alias            string   `json:"alias"`
		Title            string   `json:"title"`
		ParentAliases    []string `json:"parent_aliases"`
		CountryWhitelist []string `json:"country_whitelist"`
		CountryBlacklist []string `json:"country_blacklist"`
	} `json:"categories"`
}

func (c *Client) fusionCategories(ctx context.Context, limit int) ([]*Category, error) {
	q := url.Values{}
	if c.Locale != "" {
		q.Set("locale", c.Locale)
	}
	var resp fusionCategoriesResp
	if err := c.fusionGet(ctx, "categories", q, &resp); err != nil {
		return nil, err
	}
	country := localeCountry(c.Locale)
	var out []*Category
	for _, cat := range resp.Categories {
		if cat.Alias == "" {
			continue
		}
		if !categoryInCountry(country, cat.CountryWhitelist, cat.CountryBlacklist) {
			continue
		}
		out = append(out, &Category{
			Alias:     cat.Alias,
			Title:     squish(cat.Title),
			Parents:   cat.ParentAliases,
			SearchRef: cat.Alias,
		})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// categoryInCountry reports whether a category applies in a country. An empty
// country (no locale narrowing) keeps everything; otherwise a whitelist must
// include it and a blacklist must not.
func categoryInCountry(country string, whitelist, blacklist []string) bool {
	if country == "" {
		return true
	}
	for _, c := range blacklist {
		if strings.EqualFold(c, country) {
			return false
		}
	}
	if len(whitelist) == 0 {
		return true
	}
	for _, c := range whitelist {
		if strings.EqualFold(c, country) {
			return true
		}
	}
	return false
}

// localeCountry returns the country part of a locale ("en_US" -> "US"), or "".
func localeCountry(locale string) string {
	if i := strings.IndexByte(locale, '_'); i >= 0 && i+1 < len(locale) {
		return locale[i+1:]
	}
	return ""
}
