package yelp

import (
	"context"
	"net/url"
	"strconv"
)

// suggest.go reads autocomplete suggestions for a typed prefix. The fusion plane
// calls GET /v3/autocomplete, which returns matching search terms, businesses,
// and categories. The web plane GETs the suggest/v2/aselect endpoint; it is
// best-effort behind the PerimeterX wall. A place or category suggestion carries
// the edge into a search; a business suggestion carries the edge straight to that
// business.

// Suggest returns autocomplete suggestions for a prefix, up to limit.
func (c *Client) Suggest(ctx context.Context, prefix string, limit int) ([]*Suggestion, error) {
	if c.usesFusion() {
		return c.fusionSuggest(ctx, prefix, limit)
	}
	return c.webSuggest(ctx, prefix, limit)
}

// fusionAutocompleteResp is the GET /v3/autocomplete shape.
type fusionAutocompleteResp struct {
	Terms []struct {
		Text string `json:"text"`
	} `json:"terms"`
	Businesses []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"businesses"`
	Categories []struct {
		Alias string `json:"alias"`
		Title string `json:"title"`
	} `json:"categories"`
}

func (c *Client) fusionSuggest(ctx context.Context, prefix string, limit int) ([]*Suggestion, error) {
	q := url.Values{}
	q.Set("text", prefix)
	if c.Locale != "" {
		q.Set("locale", c.Locale)
	}
	if c.Latitude != 0 || c.Longitude != 0 {
		q.Set("latitude", strconv.FormatFloat(c.Latitude, 'f', -1, 64))
		q.Set("longitude", strconv.FormatFloat(c.Longitude, 'f', -1, 64))
	}
	var resp fusionAutocompleteResp
	if err := c.fusionGet(ctx, "autocomplete", q, &resp); err != nil {
		return nil, err
	}
	var out []*Suggestion
	add := func(s *Suggestion) bool {
		out = append(out, s)
		return limit <= 0 || len(out) < limit
	}
	for _, t := range resp.Terms {
		if t.Text == "" {
			continue
		}
		if !add(&Suggestion{Query: prefix, Text: squish(t.Text), Kind: "place", SearchRef: squish(t.Text)}) {
			return out, nil
		}
	}
	for _, b := range resp.Businesses {
		if b.Name == "" {
			continue
		}
		if !add(&Suggestion{Query: prefix, Text: squish(b.Name), Kind: "business", Alias: b.ID, Business: b.ID}) {
			return out, nil
		}
	}
	for _, cat := range resp.Categories {
		if cat.Title == "" {
			continue
		}
		if !add(&Suggestion{Query: prefix, Text: squish(cat.Title), Kind: "category", Alias: cat.Alias, SearchRef: squish(cat.Title)}) {
			return out, nil
		}
	}
	return out, nil
}

// webAselectResp is the suggest/v2/aselect shape: grouped suggestion lists, each
// entry carrying a title and, for a business, a redirect_url that names the alias.
type webAselectResp struct {
	Response []struct {
		Suggestions []struct {
			Title       string `json:"title"`
			Query       string `json:"query"`
			RedirectURL string `json:"redirect_url"`
		} `json:"suggestions"`
	} `json:"response"`
}

func (c *Client) webSuggest(ctx context.Context, prefix string, limit int) ([]*Suggestion, error) {
	q := url.Values{}
	q.Set("prefix", prefix)
	if c.Location != "" {
		q.Set("loc", c.Location)
	}
	var resp webAselectResp
	if err := c.getJSON(ctx, c.BaseURL+"/search/suggest/v2/aselect?"+q.Encode(), &resp); err != nil {
		return nil, err
	}
	var out []*Suggestion
	seen := map[string]bool{}
	for _, group := range resp.Response {
		for _, s := range group.Suggestions {
			text := squish(s.Title)
			if text == "" {
				text = squish(s.Query)
			}
			if text == "" || seen[text] {
				continue
			}
			seen[text] = true
			sg := &Suggestion{Query: prefix, Text: text}
			if alias := bizAliasFromURL(s.RedirectURL); alias != "" {
				sg.Kind = "business"
				sg.Alias = alias
				sg.Business = alias
			} else {
				sg.Kind = "place"
				sg.SearchRef = text
			}
			out = append(out, sg)
			if limit > 0 && len(out) >= limit {
				return out, nil
			}
		}
	}
	return out, nil
}

// bizAliasFromURL pulls a business alias out of a /biz/<alias> redirect url, or
// "" when the url does not name one.
func bizAliasFromURL(u string) string {
	r := Classify(u)
	if r.Kind == "biz" {
		return r.ID
	}
	return ""
}
