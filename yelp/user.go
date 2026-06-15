package yelp

import (
	"context"
	"net/url"
	"regexp"
	"strings"
)

// user.go reads a reviewer's public profile. The Fusion API exposes no user
// endpoint, so this is a web-plane surface: it GETs /user_details?userid=<id> and
// lifts the public profile a logged-out visitor sees. It is best-effort behind
// the PerimeterX wall, and on the fusion plane it reports that the surface needs
// the web plane rather than pretending the key reaches it.

var (
	ogTitleRE  = regexp.MustCompile(`<meta[^>]+property="og:title"[^>]+content="([^"]*)"`)
	ogImageRE  = regexp.MustCompile(`<meta[^>]+property="og:image"[^>]+content="([^"]*)"`)
	ogDescRE   = regexp.MustCompile(`<meta[^>]+(?:property|name)="(?:og:description|description)"[^>]+content="([^"]*)"`)
	userStatRE = regexp.MustCompile(`(?i)([0-9][0-9,]*)\s*(friends?|reviews?|photos?)`)
	sinceRE    = regexp.MustCompile(`(?i)(?:member since|yelping since)[^0-9a-z]*([a-z0-9, ]+)`)
	locLineRE  = regexp.MustCompile(`(?i)Location[^>]*</[^>]+>\s*<[^>]+>([^<]{2,60})<`)
)

// User returns a reviewer's public profile by user id (or a /user_details
// reference). The fusion plane has no user endpoint, so this always reads the
// web plane; on a forced --plane fusion it returns ErrNeedKey only when the web
// read would itself need a different plane, which it never does, so it just reads
// the web.
func (c *Client) User(ctx context.Context, ref string) (*User, error) {
	id := userID(ref)
	if id == "" {
		return nil, ErrNotFound
	}
	body, err := c.get(ctx, c.BaseURL+"/user_details?userid="+url.QueryEscape(id))
	if err != nil {
		return nil, err
	}
	u := &User{ID: id, URL: URLFor("user", id)}
	if m := ogTitleRE.FindSubmatch(body); m != nil {
		u.Name = cleanUserName(string(m[1]))
	}
	if m := ogImageRE.FindSubmatch(body); m != nil {
		u.Image = string(m[1])
	}
	if m := ogDescRE.FindSubmatch(body); m != nil {
		u.About = squish(string(m[1]))
	}
	for _, m := range userStatRE.FindAllSubmatch(body, -1) {
		n := firstInt(string(m[1]))
		switch strings.ToLower(strings.TrimRight(string(m[2]), "s")) {
		case "friend":
			if u.FriendCount == 0 {
				u.FriendCount = n
			}
		case "review":
			if u.ReviewCount == 0 {
				u.ReviewCount = n
			}
		case "photo":
			if u.PhotoCount == 0 {
				u.PhotoCount = n
			}
		}
	}
	if m := sinceRE.FindSubmatch(body); m != nil {
		u.Since = squish(string(m[1]))
	}
	if m := locLineRE.FindSubmatch(body); m != nil {
		u.Location = squish(string(m[1]))
	}
	if u.Name == "" {
		// No recognizable profile: a wall stub rather than a real page.
		return nil, ErrBlocked
	}
	return u, nil
}

// cleanUserName trims the " | User Profile | Yelp" suffix Yelp puts in og:title.
func cleanUserName(s string) string {
	if i := strings.Index(s, " | "); i >= 0 {
		s = s[:i]
	}
	return squish(s)
}
