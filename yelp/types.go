package yelp

// This file holds the exported records the commands emit. Their json tags name
// the fields a reader sees, kit:"id" marks the key the record store upserts on,
// kit:"body" marks the long-text field `yelp cat` and the Markdown export print,
// and table:",truncate" keeps wide free text from blowing up a terminal table.
// Each record carries only fields a logged-out reader (web plane) or a free
// Fusion key (fusion plane) can actually fill: no private dashboards, no owner
// analytics, no messages, because none of that is reachable without a signed-in
// account. There is no Rank column; emit order is the rank. Several records are
// emitted by more than one surface, and omitempty carries the gaps.
//
// The kit:"link" edges connect the records into one graph a host walks for
// breadth-first crawls, and they are what lets a crawl reconstruct the public
// site from a single seed. A resolver edge (business, user, category) names a
// bare field and points at one record; a collection edge carries the parent id
// under a <name>_ref field and points at a list authority. Following all of them
// closes the loop:
//
//	suggestion --search_ref--> search ----> business --reviews_ref--> reviews
//	suggestion --business----> business     business --category_ref-> search
//	suggestion --category----> category                                  |
//	category   --search_ref--> search                                    |
//	category   --parent------> category (up the taxonomy)                |
//	review --business--> business   review --author_id--> user           |
//	                                                                      v
//	                                                  review --business--> business
//
// so a suggestion fans out into a place search, straight to a business, or into
// the category that names it; a search card walks through to the full business;
// a business reaches its reviews and a same-category search; a review reaches
// back to its business and on to the reviewer's profile; a category fans into a
// search and climbs to its parent. The business graph (search, biz, reviews) and
// the taxonomy (categories, category) are both fully connected, so a crawl from
// any seed reaches the rest of the reachable public site. The one node without an
// outward edge is the user: the Fusion API has no user endpoint and the web
// profile exposes no clean reviews feed to a logged-out reader, so a reviewer is
// a leaf, not a fabricated edge.

// Business is a Yelp business, emitted by search (as a card) and by biz (full
// detail). The id is the alias, the human slug in /biz/<alias>, so the same
// record is addressed the same way from a card and from a direct read. ID holds
// Yelp's opaque business id (the 22-char encid) when a plane carries it.
type Business struct {
	Alias           string   `json:"alias" kit:"id"`         // the /biz/<alias> slug
	ID              string   `json:"id,omitempty" table:"-"` // Yelp's opaque encid, when present
	Name            string   `json:"name,omitempty" table:",truncate"`
	Rating          float64  `json:"rating,omitempty"`
	ReviewCount     int      `json:"review_count,omitempty"`
	Price           string   `json:"price,omitempty"`           // "$" to "$$$$"
	Phone           string   `json:"phone,omitempty" table:"-"` // E.164
	DisplayPhone    string   `json:"display_phone,omitempty" table:"-"`
	Categories      []string `json:"categories,omitempty" table:"-"`       // human titles, e.g. "Mexican"
	CategoryAliases []string `json:"category_aliases,omitempty" table:"-"` // category slugs, e.g. "mexican"
	Street          string   `json:"street,omitempty" table:"-"`
	City            string   `json:"city,omitempty"`
	State           string   `json:"state,omitempty"`
	Zip             string   `json:"zip,omitempty" table:"-"`
	Country         string   `json:"country,omitempty" table:"-"`
	DisplayAddress  []string `json:"display_address,omitempty" table:"-"` // the address as printed on the page
	Lat             float64  `json:"lat,omitempty" table:"-"`
	Lng             float64  `json:"lng,omitempty" table:"-"`
	Distance        float64  `json:"distance,omitempty" table:"-"` // meters from the search center, on a fusion search
	Hours           []string `json:"hours,omitempty" table:"-"`    // "Mon 9:00-17:00", localized
	OpenNow         bool     `json:"open_now,omitempty" table:"-"`
	Transactions    []string `json:"transactions,omitempty" table:"-"` // delivery, pickup, restaurant_reservation
	Attributes      []string `json:"attributes,omitempty" table:"-"`   // a flag like "BusinessAcceptsCreditCards", or "key=value" for a valued one
	Neighborhoods   []string `json:"neighborhoods,omitempty" table:"-"`
	IsClaimed       bool     `json:"is_claimed,omitempty" table:"-"`
	IsClosed        bool     `json:"is_closed,omitempty" table:"-"` // permanently closed
	About           string   `json:"about,omitempty" table:",truncate" kit:"body"`
	Image           string   `json:"image,omitempty" table:",truncate"`
	Photos          []string `json:"photos,omitempty" table:"-"`
	URL             string   `json:"url"`
	ReviewsRef      string   `json:"reviews_ref,omitempty" table:"-" kit:"link,kind=yelp/reviews"` // edge to this business's reviews (= alias)
	CategoryRef     string   `json:"category_ref,omitempty" table:"-" kit:"link,kind=yelp/search"` // edge to a same-category search (= first category alias)
}

// Review is one review of a business, emitted by reviews. Business is the edge
// back to the reviewed business; AuthorID, when the plane carries it, is the
// reviewer's user id, so a crawl can walk a review to the reviewer's profile.
// The fusion plane returns three review excerpts per business; the web review_feed
// returns the full paginated set.
type Review struct {
	ID             string   `json:"id" kit:"id"`
	Rating         int      `json:"rating,omitempty"`
	Author         string   `json:"author,omitempty"`
	AuthorID       string   `json:"author_id,omitempty" table:"-" kit:"link,kind=yelp/user"` // edge to the reviewer's profile
	AuthorImage    string   `json:"author_image,omitempty" table:"-"`
	AuthorLocation string   `json:"author_location,omitempty"` // the reviewer's stated home, when shown
	Date           string   `json:"date,omitempty"`            // the review date
	Text           string   `json:"text,omitempty" table:",truncate" kit:"body"`
	Language       string   `json:"language,omitempty" table:"-"`
	Useful         int      `json:"useful,omitempty" table:"-"`
	Funny          int      `json:"funny,omitempty" table:"-"`
	Cool           int      `json:"cool,omitempty" table:"-"`
	Photos         []string `json:"photos,omitempty" table:"-"`
	URL            string   `json:"url,omitempty" table:"-"`
	Business       string   `json:"business,omitempty" table:"-" kit:"link,kind=yelp/biz"` // edge back to the business (= alias)
}

// User is a reviewer's public profile, emitted by user. It is a web-plane surface
// (the Fusion API exposes no user endpoint), so it is best-effort behind the bot
// wall. It carries only the public profile a logged-out visitor sees.
type User struct {
	ID          string `json:"id" kit:"id"` // Yelp user id
	Name        string `json:"name,omitempty"`
	Location    string `json:"location,omitempty"`
	ReviewCount int    `json:"review_count,omitempty"`
	FriendCount int    `json:"friend_count,omitempty" table:"-"`
	PhotoCount  int    `json:"photo_count,omitempty" table:"-"`
	Since       string `json:"since,omitempty"` // "Member since ..."
	About       string `json:"about,omitempty" table:",truncate" kit:"body"`
	Image       string `json:"image,omitempty" table:",truncate"`
	URL         string `json:"url"`
}

// Suggestion is one autocomplete entry, emitted by suggest. Kind is "place",
// "business", or "category". A place suggestion carries SearchRef as the edge
// into a search; a business suggestion carries Business as the edge straight to
// that business; a category suggestion carries SearchRef scoped to the category
// and Category as the edge to the category record itself.
type Suggestion struct {
	Query     string  `json:"query"`           // the prefix that was queried
	Text      string  `json:"text" kit:"id"`   // the suggested text
	Kind      string  `json:"kind,omitempty"`  // place, business, category
	Alias     string  `json:"alias,omitempty"` // a business alias or a category alias, when known
	Lat       float64 `json:"lat,omitempty" table:"-"`
	Lng       float64 `json:"lng,omitempty" table:"-"`
	SearchRef string  `json:"search_ref,omitempty" table:"-" kit:"link,kind=yelp/search"` // edge into a search (= text)
	Business  string  `json:"business,omitempty" table:"-" kit:"link,kind=yelp/biz"`      // edge to a business (= alias), for a business suggestion
	Category  string  `json:"category,omitempty" table:"-" kit:"link,kind=yelp/category"` // edge to the category (= alias), for a category suggestion
}

// Category is one Yelp category, emitted by categories (the whole taxonomy) and
// by category (one alias). The id is the alias, the slug Yelp uses in the
// categories param. Parents are the alias's parent categories; SearchRef is the
// edge into a search by this category, and ParentRef climbs to the first parent
// so a crawl can walk the taxonomy tree in both directions.
type Category struct {
	Alias     string   `json:"alias" kit:"id"`
	Title     string   `json:"title,omitempty"`
	Parents   []string `json:"parents,omitempty" table:"-"`
	SearchRef string   `json:"search_ref,omitempty" table:"-" kit:"link,kind=yelp/search"`   // edge into a search by this category (= alias)
	ParentRef string   `json:"parent_ref,omitempty" table:"-" kit:"link,kind=yelp/category"` // edge up to the first parent category (= parent alias)
}

// Ref is the result of `yelp ref id`: the canonical (kind, id) a reference
// resolves to, plus the live URL, all without touching the network.
type Ref struct {
	Input string `json:"input"`
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	URL   string `json:"url"`
}
