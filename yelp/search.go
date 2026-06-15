package yelp

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// search.go reads a business search for a term and a location. The fusion plane
// calls GET /v3/businesses/search, which answers from any network and pages the
// offset. The web plane GETs the search page and reads the embedded result JSON;
// it is best-effort behind the PerimeterX wall. Every card links straight through
// to its full business, so a host can crawl a search result onward.

// Search returns businesses matching term in a location, up to limit.
func (c *Client) Search(ctx context.Context, term, location string, limit int) ([]*Business, error) {
	if location == "" {
		location = c.Location
	}
	if c.usesFusion() {
		return c.fusionSearch(ctx, term, location, limit)
	}
	return c.webSearch(ctx, term, location, limit)
}

// fusionSearchResp is the GET /v3/businesses/search shape.
type fusionSearchResp struct {
	Businesses []fusionBusinessResp `json:"businesses"`
	Total      int                  `json:"total"`
}

func (c *Client) fusionSearch(ctx context.Context, term, location string, limit int) ([]*Business, error) {
	if limit <= 0 {
		limit = defaultLimit
	}
	var out []*Business
	seen := map[string]bool{}
	for offset := 0; offset < limit; offset += 50 {
		page := 50
		if rem := limit - offset; rem < page {
			page = rem
		}
		q := url.Values{}
		if term != "" {
			q.Set("term", term)
		}
		if location != "" {
			q.Set("location", location)
		} else if c.Latitude != 0 || c.Longitude != 0 {
			q.Set("latitude", strconv.FormatFloat(c.Latitude, 'f', -1, 64))
			q.Set("longitude", strconv.FormatFloat(c.Longitude, 'f', -1, 64))
		}
		if c.Sort != "" {
			q.Set("sort_by", c.Sort)
		}
		if c.Price != "" {
			q.Set("price", c.Price)
		}
		if c.Locale != "" {
			q.Set("locale", c.Locale)
		}
		q.Set("limit", strconv.Itoa(page))
		q.Set("offset", strconv.Itoa(offset))

		var resp fusionSearchResp
		if err := c.fusionGet(ctx, "businesses/search", q, &resp); err != nil {
			if len(out) > 0 {
				return out, nil
			}
			return nil, err
		}
		if len(resp.Businesses) == 0 {
			break
		}
		for i := range resp.Businesses {
			b := resp.Businesses[i].toBusiness(resp.Businesses[i].Alias)
			if b.Alias == "" || seen[b.Alias] {
				continue
			}
			seen[b.Alias] = true
			out = append(out, b)
			if len(out) >= limit {
				return out, nil
			}
		}
		if len(out) >= resp.Total {
			break
		}
	}
	return out, nil
}

// webSearchResp is the slice of the search page's embedded state this reads: the
// app stores its results as a list of components, each card holding a
// searchResultBusiness with the alias and the summary fields.
type webSearchResp struct {
	SearchPageProps struct {
		MainContentComponentsListProps []struct {
			SearchResultBusiness *struct {
				Alias       string  `json:"alias"`
				Name        string  `json:"name"`
				Rating      float64 `json:"rating"`
				ReviewCount int     `json:"reviewCount"`
				PriceRange  string  `json:"priceRange"`
				Categories  []struct {
					Alias string `json:"alias"`
					Title string `json:"title"`
				} `json:"categories"`
				Neighborhoods []string `json:"neighborhoods"`
				PhotoSrc      string   `json:"photoSrc"`
			} `json:"searchResultBusiness"`
		} `json:"mainContentComponentsListProps"`
	} `json:"searchPageProps"`
}

func (c *Client) webSearch(ctx context.Context, term, location string, limit int) ([]*Business, error) {
	q := url.Values{}
	if term != "" {
		q.Set("find_desc", term)
	}
	if location != "" {
		q.Set("find_loc", location)
	}
	body, err := c.get(ctx, c.BaseURL+"/search?"+q.Encode())
	if err != nil {
		return nil, err
	}
	state, ok := extractWebState(body)
	if !ok {
		return nil, ErrBlocked
	}
	var resp webSearchResp
	if json.Unmarshal(state, &resp) != nil {
		return nil, ErrBlocked
	}
	var out []*Business
	seen := map[string]bool{}
	for _, comp := range resp.SearchPageProps.MainContentComponentsListProps {
		sb := comp.SearchResultBusiness
		if sb == nil || sb.Alias == "" || seen[sb.Alias] {
			continue
		}
		seen[sb.Alias] = true
		b := &Business{
			Alias:         sb.Alias,
			Name:          squish(sb.Name),
			Rating:        sb.Rating,
			ReviewCount:   sb.ReviewCount,
			Price:         dollars(sb.PriceRange),
			Neighborhoods: sb.Neighborhoods,
			Image:         sb.PhotoSrc,
			URL:           BaseURL + "/biz/" + sb.Alias,
		}
		for _, cat := range sb.Categories {
			if cat.Title != "" {
				b.Categories = append(b.Categories, cat.Title)
			}
			if cat.Alias != "" {
				b.CategoryAliases = append(b.CategoryAliases, cat.Alias)
			}
		}
		applyEdges(b)
		out = append(out, b)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}
