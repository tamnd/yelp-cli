package yelp

import (
	"context"
	"net/url"
	"strconv"
)

// reviews.go reads a business's reviews. The fusion plane calls
// GET /v3/businesses/{id}/reviews, which answers from any network but returns
// only three review excerpts per business (a documented Fusion limit). The web
// plane GETs the review_feed JSON endpoint, which pages the full set; it is
// best-effort behind the PerimeterX wall. Each review links back to its business
// and on to the reviewer's profile.

// Reviews returns reviews of a business by alias, up to limit.
func (c *Client) Reviews(ctx context.Context, ref string, limit int) ([]*Review, error) {
	alias := bizAlias(ref)
	if alias == "" {
		return nil, ErrNotFound
	}
	if c.usesFusion() {
		return c.fusionReviews(ctx, alias, limit)
	}
	return c.webReviews(ctx, alias, limit)
}

// fusionReviewsResp is the GET /v3/businesses/{id}/reviews shape.
type fusionReviewsResp struct {
	Reviews []struct {
		ID          string `json:"id"`
		URL         string `json:"url"`
		Text        string `json:"text"`
		Rating      int    `json:"rating"`
		TimeCreated string `json:"time_created"`
		User        struct {
			ID         string `json:"id"`
			ProfileURL string `json:"profile_url"`
			ImageURL   string `json:"image_url"`
			Name       string `json:"name"`
		} `json:"user"`
	} `json:"reviews"`
	Total int `json:"total"`
}

func (c *Client) fusionReviews(ctx context.Context, alias string, limit int) ([]*Review, error) {
	q := url.Values{}
	if c.Locale != "" {
		q.Set("locale", c.Locale)
	}
	if c.Sort != "" {
		q.Set("sort_by", c.Sort)
	}
	var resp fusionReviewsResp
	if err := c.fusionGet(ctx, "businesses/"+url.PathEscape(alias)+"/reviews", q, &resp); err != nil {
		return nil, err
	}
	var out []*Review
	for _, r := range resp.Reviews {
		rv := &Review{
			ID:          r.ID,
			Rating:      r.Rating,
			Author:      squish(r.User.Name),
			AuthorID:    userIDFromURL(r.User.ProfileURL, r.User.ID),
			AuthorImage: r.User.ImageURL,
			Date:        r.TimeCreated,
			Text:        squish(r.Text),
			URL:         r.URL,
			Business:    alias,
		}
		out = append(out, rv)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// webReviewFeed is the /biz/<alias>/review_feed JSON shape.
type webReviewFeed struct {
	Reviews []struct {
		ID      string `json:"id"`
		Rating  int    `json:"rating"`
		Comment struct {
			Text     string `json:"text"`
			Language string `json:"language"`
		} `json:"comment"`
		LocalizedDate string `json:"localizedDate"`
		User          struct {
			EncID             string `json:"encid"`
			MarkupDisplayName string `json:"markupDisplayName"`
			DisplayLocation   string `json:"displayLocation"`
			Src               string `json:"src"`
			Link              string `json:"link"`
		} `json:"user"`
		Feedback struct {
			Counts struct {
				Useful int `json:"useful"`
				Funny  int `json:"funny"`
				Cool   int `json:"cool"`
			} `json:"counts"`
		} `json:"feedback"`
		Photos []struct {
			Src string `json:"src"`
		} `json:"photos"`
	} `json:"reviews"`
	Pagination struct {
		TotalResults   int `json:"totalResults"`
		ResultsPerPage int `json:"resultsPerPage"`
	} `json:"pagination"`
}

func (c *Client) webReviews(ctx context.Context, alias string, limit int) ([]*Review, error) {
	var out []*Review
	seen := map[string]bool{}
	for start := 0; ; start += 10 {
		q := url.Values{}
		q.Set("rl", localeLang(c.Locale))
		q.Set("sort_by", "relevance_desc")
		q.Set("start", strconv.Itoa(start))
		var feed webReviewFeed
		if err := c.getJSON(ctx, c.BaseURL+"/biz/"+url.PathEscape(alias)+"/review_feed?"+q.Encode(), &feed); err != nil {
			if len(out) > 0 {
				return out, nil
			}
			return nil, err
		}
		if len(feed.Reviews) == 0 {
			break
		}
		for _, r := range feed.Reviews {
			if r.ID == "" || seen[r.ID] {
				continue
			}
			seen[r.ID] = true
			rv := &Review{
				ID:             r.ID,
				Rating:         r.Rating,
				Author:         stripHTML(r.User.MarkupDisplayName),
				AuthorID:       userIDFromURL(r.User.Link, r.User.EncID),
				AuthorImage:    r.User.Src,
				AuthorLocation: squish(r.User.DisplayLocation),
				Date:           r.LocalizedDate,
				Text:           squish(r.Comment.Text),
				Language:       r.Comment.Language,
				Useful:         r.Feedback.Counts.Useful,
				Funny:          r.Feedback.Counts.Funny,
				Cool:           r.Feedback.Counts.Cool,
				Business:       alias,
			}
			for _, p := range r.Photos {
				if p.Src != "" {
					rv.Photos = append(rv.Photos, p.Src)
				}
			}
			out = append(out, rv)
			if limit > 0 && len(out) >= limit {
				return out, nil
			}
		}
		total := feed.Pagination.TotalResults
		if total > 0 && len(seen) >= total {
			break
		}
	}
	return out, nil
}

// localeLang reduces a locale ("en_US") to the language ("en") the review_feed rl
// parameter wants.
func localeLang(locale string) string {
	if locale == "" {
		return "en"
	}
	if i := indexUnderscore(locale); i > 0 {
		return locale[:i]
	}
	return locale
}

func indexUnderscore(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '_' {
			return i
		}
	}
	return -1
}
