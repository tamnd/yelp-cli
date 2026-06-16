package yelp

import (
	"context"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// biz.go reads one business by its alias. The fusion plane calls
// GET /v3/businesses/{alias}, which answers from any network and carries the full
// detail. The web plane GETs /biz/<alias> and reads the schema.org JSON-LD island
// the page embeds; it is best-effort behind the PerimeterX wall. Either way the
// record carries the reviews_ref and category_ref edges so a crawl reaching a
// business expands to its reviews and a same-category search.

// Business returns one business by alias (or a /biz/<alias> reference).
func (c *Client) Business(ctx context.Context, ref string) (*Business, error) {
	alias := bizAlias(ref)
	if alias == "" {
		return nil, ErrNotFound
	}
	if c.usesFusion() {
		return c.fusionBusiness(ctx, alias)
	}
	return c.webBusiness(ctx, alias)
}

// fusionBusinessResp is the GET /v3/businesses/{id} shape.
type fusionBusinessResp struct {
	ID           string   `json:"id"`
	Alias        string   `json:"alias"`
	Name         string   `json:"name"`
	ImageURL     string   `json:"image_url"`
	IsClaimed    bool     `json:"is_claimed"`
	IsClosed     bool     `json:"is_closed"`
	URL          string   `json:"url"`
	Phone        string   `json:"phone"`
	DisplayPhone string   `json:"display_phone"`
	ReviewCount  int      `json:"review_count"`
	Rating       float64  `json:"rating"`
	Price        string   `json:"price"`
	Distance     float64  `json:"distance"` // meters from the search center, present on a search result
	Photos       []string `json:"photos"`
	Transactions []string `json:"transactions"`
	Categories   []struct {
		Alias string `json:"alias"`
		Title string `json:"title"`
	} `json:"categories"`
	Coordinates struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"coordinates"`
	Location struct {
		Address1       string   `json:"address1"`
		City           string   `json:"city"`
		ZipCode        string   `json:"zip_code"`
		Country        string   `json:"country"`
		State          string   `json:"state"`
		DisplayAddress []string `json:"display_address"`
	} `json:"location"`
	Hours []struct {
		Open []struct {
			IsOvernight bool   `json:"is_overnight"`
			Start       string `json:"start"`
			End         string `json:"end"`
			Day         int    `json:"day"`
		} `json:"open"`
		HoursType string `json:"hours_type"`
		IsOpenNow bool   `json:"is_open_now"`
	} `json:"hours"`
	Attributes map[string]any `json:"attributes"`
}

func (c *Client) fusionBusiness(ctx context.Context, alias string) (*Business, error) {
	var r fusionBusinessResp
	if err := c.fusionGet(ctx, "businesses/"+url.PathEscape(alias), nil, &r); err != nil {
		return nil, err
	}
	return r.toBusiness(alias), nil
}

func (r fusionBusinessResp) toBusiness(alias string) *Business {
	if r.Alias != "" {
		alias = r.Alias
	}
	b := &Business{
		Alias:          alias,
		ID:             r.ID,
		Name:           squish(r.Name),
		Rating:         r.Rating,
		ReviewCount:    r.ReviewCount,
		Price:          r.Price,
		Phone:          r.Phone,
		DisplayPhone:   r.DisplayPhone,
		Street:         r.Location.Address1,
		City:           r.Location.City,
		State:          r.Location.State,
		Zip:            r.Location.ZipCode,
		Country:        r.Location.Country,
		DisplayAddress: r.Location.DisplayAddress,
		Lat:            r.Coordinates.Latitude,
		Lng:            r.Coordinates.Longitude,
		Distance:       r.Distance,
		Transactions:   r.Transactions,
		IsClaimed:      r.IsClaimed,
		IsClosed:       r.IsClosed,
		Image:          r.ImageURL,
		Photos:         r.Photos,
		URL:            bizURL(alias, r.URL),
	}
	for _, cat := range r.Categories {
		if cat.Title != "" {
			b.Categories = append(b.Categories, cat.Title)
		}
		if cat.Alias != "" {
			b.CategoryAliases = append(b.CategoryAliases, cat.Alias)
		}
	}
	for _, h := range r.Hours {
		if h.IsOpenNow {
			b.OpenNow = true
		}
		for _, o := range h.Open {
			b.Hours = append(b.Hours, fusionHourLine(o.Day, o.Start, o.End))
		}
	}
	b.Attributes = flattenAttributes(r.Attributes)
	applyEdges(b)
	return b
}

func (c *Client) webBusiness(ctx context.Context, alias string) (*Business, error) {
	body, err := c.get(ctx, c.BaseURL+"/biz/"+url.PathEscape(alias))
	if err != nil {
		return nil, err
	}
	ld, ok := firstBusinessLD(body)
	if !ok {
		// The page loaded but carried no business JSON-LD: treat it as the wall,
		// since a real biz page always embeds one.
		return nil, ErrBlocked
	}
	b := &Business{
		Alias:   alias,
		Name:    squish(ld.Name),
		Phone:   ld.Telephone,
		Price:   dollars(ld.PriceRange),
		Street:  ld.Address.StreetAddress,
		City:    ld.Address.AddressLocality,
		State:   ld.Address.AddressRegion,
		Zip:     ld.Address.PostalCode,
		Country: ld.Address.AddressCountry,
		About:   squish(ld.Description),
		URL:     bizURL(alias, ""),
	}
	if v, err := ld.AggregateRating.RatingValue.Float64(); err == nil {
		b.Rating = v
	}
	if v, err := ld.AggregateRating.ReviewCount.Int64(); err == nil {
		b.ReviewCount = int(v)
	}
	if v, err := ld.Geo.Latitude.Float64(); err == nil {
		b.Lat = v
	}
	if v, err := ld.Geo.Longitude.Float64(); err == nil {
		b.Lng = v
	}
	if b.Street != "" {
		b.DisplayAddress = []string{b.Street}
	}
	b.Image = firstString(ldImages(ld.Image))
	b.Photos = ldImages(ld.Image)
	b.Hours = jsonLDHours(ld)
	applyEdges(b)
	return b, nil
}

// applyEdges sets the outbound graph edges on a business: its reviews, and a
// search by its first category.
func applyEdges(b *Business) {
	b.ReviewsRef = b.Alias
	if len(b.CategoryAliases) > 0 {
		b.CategoryRef = b.CategoryAliases[0]
	}
}

// flattenAttributes turns the Fusion attributes map into a stable, sorted list a
// reader and a filter can use. A boolean flag that is on becomes its bare key
// ("BusinessAcceptsCreditCards"); a valued attribute becomes "key=value"
// ("RestaurantsPriceRange2=2", "WiFi=free"); a nested group (BusinessParking,
// Ambience) becomes one "group.sub" or "group.sub=value" entry per set member.
// Off flags, empty values, and null entries are dropped, since they carry no
// fact. Map iteration is unordered, so the result is sorted to keep the record
// stable across reads.
func flattenAttributes(attrs map[string]any) []string {
	var out []string
	var add func(prefix string, v any)
	add = func(prefix string, v any) {
		switch t := v.(type) {
		case bool:
			if t {
				out = append(out, prefix)
			}
		case string:
			if s := strings.TrimSpace(t); s != "" && s != "none" {
				out = append(out, prefix+"="+s)
			}
		case float64:
			out = append(out, prefix+"="+strconv.FormatFloat(t, 'f', -1, 64))
		case map[string]any:
			for k, sv := range t {
				add(prefix+"."+k, sv)
			}
		}
	}
	for k, v := range attrs {
		add(k, v)
	}
	sort.Strings(out)
	return out
}

// jsonLDHours turns the openingHoursSpecification (or the simpler openingHours
// strings) into "Mon 09:00-17:00" lines.
func jsonLDHours(ld *jsonLD) []string {
	var out []string
	for _, s := range ld.OpeningHoursSpecification {
		days := ldDays(s.DayOfWeek)
		for _, d := range days {
			out = append(out, d+" "+s.Opens+"-"+s.Closes)
		}
	}
	if len(out) == 0 {
		out = append(out, ld.OpeningHours...)
	}
	return out
}

// fusionHourLine turns a Fusion hour block into "Mon 0900-1700".
func fusionHourLine(day int, start, end string) string {
	names := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	d := "?"
	if day >= 0 && day < len(names) {
		d = names[day]
	}
	return d + " " + start + "-" + end
}

func firstString(ss []string) string {
	if len(ss) > 0 {
		return ss[0]
	}
	return ""
}

// bizURL builds the canonical /biz/<alias> URL, preferring the API-supplied URL
// when present (it carries Yelp's tracking-free canonical form).
func bizURL(alias, apiURL string) string {
	if apiURL != "" {
		if i := strings.IndexByte(apiURL, '?'); i >= 0 {
			return apiURL[:i]
		}
		return apiURL
	}
	return BaseURL + "/biz/" + alias
}
