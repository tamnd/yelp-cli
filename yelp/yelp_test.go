package yelp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestClient builds a client whose web and fusion planes both point at one
// httptest server, with pacing off so tests run fast.
func newTestClient(t *testing.T, h http.Handler, plane, key string) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.FusionBase = srv.URL
	cfg.Delay = 0
	cfg.Plane = plane
	cfg.APIKey = key
	cfg.NoCache = true
	return NewClient(cfg)
}

func TestClassify(t *testing.T) {
	cases := []struct {
		in   string
		kind string
		id   string
	}{
		{"https://www.yelp.com/biz/molinari-delicatessen-san-francisco", "biz", "molinari-delicatessen-san-francisco"},
		{"/biz/molinari-delicatessen-san-francisco", "biz", "molinari-delicatessen-san-francisco"},
		{"molinari-delicatessen-san-francisco", "biz", "molinari-delicatessen-san-francisco"},
		{"https://www.yelp.com/user_details?userid=abc123", "user", "abc123"},
		{"/user/xyz789", "user", "xyz789"},
		{"https://www.yelp.com/search?cflt=coffee&find_loc=SF", "category", "coffee"}, // cflt parameter
		{"abcdefghij0123456789AB", "biz", "abcdefghij0123456789AB"},                   // 22-char encid
		{"", "unknown", ""},
		{"Not An Alias!", "unknown", ""},
	}
	for _, c := range cases {
		r := Classify(c.in)
		if r.Kind != c.kind || (c.kind != "unknown" && r.ID != c.id) {
			t.Errorf("Classify(%q) = (%q,%q), want (%q,%q)", c.in, r.Kind, r.ID, c.kind, c.id)
		}
	}
}

func TestURLFor(t *testing.T) {
	if got := URLFor("biz", "joe-coffee"); got != BaseURL+"/biz/joe-coffee" {
		t.Errorf("biz url = %q", got)
	}
	if got := URLFor("user", "u1"); got != BaseURL+"/user_details?userid=u1" {
		t.Errorf("user url = %q", got)
	}
	if got := URLFor("category", "coffee"); got != BaseURL+"/search?cflt=coffee" {
		t.Errorf("category url = %q", got)
	}
	if got := URLFor("nope", "x"); got != "" {
		t.Errorf("unknown kind should be empty, got %q", got)
	}
}

func TestPlaneSelection(t *testing.T) {
	cases := []struct {
		plane string
		key   string
		want  bool
	}{
		{planeAuto, "", false},
		{planeAuto, "k", true},
		{planeWeb, "k", false},
		{planeFusion, "", true},
	}
	for _, c := range cases {
		cfg := DefaultConfig()
		cfg.Plane = c.plane
		cfg.APIKey = c.key
		if got := NewClient(cfg).usesFusion(); got != c.want {
			t.Errorf("plane=%q key=%q usesFusion=%v want %v", c.plane, c.key, got, c.want)
		}
	}
}

const fusionBizJSON = `{
  "id": "encid123","alias": "molinari-deli","name": "Molinari Delicatessen",
  "image_url": "https://img/1.jpg","is_claimed": true,"is_closed": false,
  "url": "https://www.yelp.com/biz/molinari-deli?adjust=1",
  "phone": "+14154212337","display_phone": "(415) 421-2337",
  "review_count": 1200,"rating": 4.5,"price": "$$",
  "photos": ["https://img/1.jpg","https://img/2.jpg"],
  "transactions": ["delivery","pickup"],
  "categories": [{"alias":"delis","title":"Delis"},{"alias":"italian","title":"Italian"}],
  "coordinates": {"latitude": 37.79,"longitude": -122.4},
  "location": {"address1":"373 Columbus Ave","city":"San Francisco","zip_code":"94133","country":"US","state":"CA","display_address":["373 Columbus Ave","San Francisco, CA 94133"]},
  "hours": [{"open":[{"is_overnight":false,"start":"0900","end":"1700","day":0}],"hours_type":"REGULAR","is_open_now":true}],
  "attributes": {"BusinessAcceptsCreditCards": true, "RestaurantsDelivery": false}
}`

func TestFusionBusiness(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer KEY" {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"error":{"code":"TOKEN_INVALID"}}`))
			return
		}
		if r.URL.Path != "/v3/businesses/molinari-deli" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(fusionBizJSON))
	})
	c := newTestClient(t, h, planeFusion, "KEY")
	b, err := c.Business(context.Background(), "molinari-deli")
	if err != nil {
		t.Fatal(err)
	}
	if b.Name != "Molinari Delicatessen" || b.Rating != 4.5 || b.ReviewCount != 1200 {
		t.Errorf("bad business: %+v", b)
	}
	if b.ID != "encid123" || b.Price != "$$" || b.City != "San Francisco" {
		t.Errorf("bad fields: %+v", b)
	}
	if len(b.Categories) != 2 || b.CategoryAliases[0] != "delis" {
		t.Errorf("bad categories: %+v", b.Categories)
	}
	if b.ReviewsRef != "molinari-deli" || b.CategoryRef != "delis" {
		t.Errorf("bad edges: reviews=%q category=%q", b.ReviewsRef, b.CategoryRef)
	}
	if !b.OpenNow || len(b.Hours) != 1 {
		t.Errorf("bad hours: open=%v %v", b.OpenNow, b.Hours)
	}
	// Only the true attribute survives.
	if len(b.Attributes) != 1 || b.Attributes[0] != "BusinessAcceptsCreditCards" {
		t.Errorf("bad attributes: %v", b.Attributes)
	}
	// The canonical URL drops the tracking query.
	if strings.Contains(b.URL, "?") {
		t.Errorf("url kept query: %q", b.URL)
	}
}

func TestFlattenAttributes(t *testing.T) {
	attrs := map[string]any{
		"BusinessAcceptsCreditCards": true,
		"RestaurantsDelivery":        false,
		"WiFi":                       "free",
		"NoiseLevel":                 "none", // a "none" value is dropped
		"BusinessParking":            map[string]any{"garage": true, "lot": false},
	}
	got := flattenAttributes(attrs)
	want := []string{
		"BusinessAcceptsCreditCards",
		"BusinessParking.garage",
		"WiFi=free",
	}
	if len(got) != len(want) {
		t.Fatalf("flattenAttributes = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("flattenAttributes[%d] = %q, want %q (full %v)", i, got[i], want[i], got)
		}
	}
}

func TestFusionSearchFilters(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("radius") != "1500" || q.Get("categories") != "coffee" ||
			q.Get("attributes") != "hot_and_new" || q.Get("open_now") != "true" {
			t.Errorf("filters missing from query %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"total":1,"businesses":[` + fusionBizJSON + `]}`))
	})
	c := newTestClient(t, h, planeFusion, "KEY")
	c.Radius = 1500
	c.CategoryFilter = "coffee"
	c.Attributes = "hot_and_new"
	c.OpenNow = true
	if _, err := c.Search(context.Background(), "espresso", "Oakland", 5); err != nil {
		t.Fatal(err)
	}
}

func TestFusionSearchNeedsLocation(t *testing.T) {
	c := newTestClient(t, http.NotFoundHandler(), planeFusion, "KEY")
	_, err := c.Search(context.Background(), "tacos", "", 5)
	if !errors.Is(err, ErrNeedLocation) {
		t.Errorf("want ErrNeedLocation, got %v", err)
	}
}

func TestFusionSearchDistance(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"total":1,"businesses":[{"alias":"a","name":"A","distance":1234.5}]}`))
	})
	c := newTestClient(t, h, planeFusion, "KEY")
	bs, err := c.Search(context.Background(), "x", "Oakland", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(bs) != 1 || bs[0].Distance != 1234.5 {
		t.Errorf("distance not carried: %+v", bs)
	}
}

func TestFusionCategory(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/categories/coffee" {
			t.Errorf("path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"category":{"alias":"coffee","title":"Coffee & Tea","parent_aliases":["food"]}}`))
	})
	c := newTestClient(t, h, planeFusion, "KEY")
	cat, err := c.Category(context.Background(), "coffee")
	if err != nil {
		t.Fatal(err)
	}
	if cat.Alias != "coffee" || cat.Title != "Coffee & Tea" {
		t.Errorf("bad category: %+v", cat)
	}
	if cat.SearchRef != "coffee" || cat.ParentRef != "food" {
		t.Errorf("bad category edges: search=%q parent=%q", cat.SearchRef, cat.ParentRef)
	}
}

func TestCategoryNeedKeyOnWeb(t *testing.T) {
	c := newTestClient(t, http.NotFoundHandler(), planeWeb, "")
	_, err := c.Category(context.Background(), "coffee")
	if !errors.Is(err, ErrNeedKey) {
		t.Errorf("want ErrNeedKey, got %v", err)
	}
}

func TestFusionKeyRejected(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"code":"TOKEN_INVALID"}}`))
	})
	c := newTestClient(t, h, planeFusion, "BAD")
	_, err := c.Business(context.Background(), "x")
	if !errors.Is(err, ErrKeyRejected) {
		t.Errorf("want ErrKeyRejected, got %v", err)
	}
}

func TestFusionSearch(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/businesses/search" {
			t.Errorf("path %q", r.URL.Path)
		}
		if r.URL.Query().Get("term") != "pizza" || r.URL.Query().Get("location") != "Oakland" {
			t.Errorf("bad query %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"total":1,"businesses":[` + fusionBizJSON + `]}`))
	})
	c := newTestClient(t, h, planeFusion, "KEY")
	bs, err := c.Search(context.Background(), "pizza", "Oakland", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(bs) != 1 || bs[0].Alias != "molinari-deli" {
		t.Errorf("bad search: %+v", bs)
	}
}

func TestFusionReviews(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/businesses/joe/reviews" {
			t.Errorf("path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"total":1,"reviews":[{"id":"rev1","rating":5,"text":"Great","time_created":"2024-01-02 10:00:00","url":"https://www.yelp.com/biz/joe?hrid=rev1","user":{"id":"u1","name":"Jane","profile_url":"https://www.yelp.com/user_details?userid=u1","image_url":"https://img/u1.jpg"}}]}`))
	})
	c := newTestClient(t, h, planeFusion, "KEY")
	rs, err := c.Reviews(context.Background(), "joe", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 || rs[0].Author != "Jane" || rs[0].AuthorID != "u1" || rs[0].Business != "joe" {
		t.Errorf("bad review: %+v", rs[0])
	}
}

func TestFusionCategories(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"categories":[
		  {"alias":"delis","title":"Delis","parent_aliases":["food"],"country_whitelist":[]},
		  {"alias":"poutineries","title":"Poutineries","parent_aliases":["food"],"country_whitelist":["CA"]}
		]}`))
	})
	c := newTestClient(t, h, planeFusion, "KEY") // locale en_US -> country US
	cs, err := c.Categories(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	// Poutineries is CA-only, so a US locale drops it.
	if len(cs) != 1 || cs[0].Alias != "delis" || cs[0].SearchRef != "delis" {
		t.Errorf("country filter failed: %+v", cs)
	}
	if cs[0].ParentRef != "food" {
		t.Errorf("missing parent edge: %q", cs[0].ParentRef)
	}
}

func TestCategoriesNeedKeyOnWeb(t *testing.T) {
	c := newTestClient(t, http.NotFoundHandler(), planeWeb, "")
	_, err := c.Categories(context.Background(), 0)
	if !errors.Is(err, ErrNeedKey) {
		t.Errorf("want ErrNeedKey, got %v", err)
	}
}

func TestFusionSuggest(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"terms":[{"text":"pizza"}],"businesses":[{"id":"b1","name":"Pizza Place"}],"categories":[{"alias":"pizza","title":"Pizza"}]}`))
	})
	c := newTestClient(t, h, planeFusion, "KEY")
	ss, err := c.Suggest(context.Background(), "piz", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(ss) != 3 {
		t.Fatalf("want 3 suggestions, got %d", len(ss))
	}
	if ss[0].Kind != "place" || ss[0].SearchRef != "pizza" {
		t.Errorf("bad term suggestion: %+v", ss[0])
	}
	if ss[1].Kind != "business" || ss[1].Business != "b1" {
		t.Errorf("bad business suggestion: %+v", ss[1])
	}
	if ss[2].Kind != "category" || ss[2].Alias != "pizza" || ss[2].Category != "pizza" {
		t.Errorf("bad category suggestion: %+v", ss[2])
	}
}

const bizPageHTML = `<!doctype html><html><head>
<script type="application/ld+json">{"@type":"Restaurant","name":"Joe Coffee","telephone":"+14150000000","priceRange":"$$","description":"A cozy spot.","image":["https://img/a.jpg","https://img/b.jpg"],"address":{"streetAddress":"1 Main St","addressLocality":"San Francisco","addressRegion":"CA","postalCode":"94100","addressCountry":"US"},"aggregateRating":{"ratingValue":"4.0","reviewCount":"88"},"geo":{"latitude":"37.77","longitude":"-122.41"},"openingHoursSpecification":[{"dayOfWeek":"https://schema.org/Monday","opens":"08:00","closes":"17:00"}]}</script>
</head><body>Joe Coffee</body></html>`

func TestWebBusiness(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/biz/joe-coffee" {
			t.Errorf("path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(bizPageHTML))
	})
	c := newTestClient(t, h, planeWeb, "")
	b, err := c.Business(context.Background(), "joe-coffee")
	if err != nil {
		t.Fatal(err)
	}
	if b.Name != "Joe Coffee" || b.Rating != 4.0 || b.ReviewCount != 88 || b.Price != "$$" {
		t.Errorf("bad web business: %+v", b)
	}
	if b.City != "San Francisco" || b.State != "CA" || b.Lat != 37.77 {
		t.Errorf("bad address/geo: %+v", b)
	}
	if b.About != "A cozy spot." || len(b.Photos) != 2 {
		t.Errorf("bad about/photos: %+v", b)
	}
	if len(b.Hours) != 1 || b.Hours[0] != "Mon 08:00-17:00" {
		t.Errorf("bad hours: %v", b.Hours)
	}
	if b.ReviewsRef != "joe-coffee" {
		t.Errorf("missing reviews edge: %q", b.ReviewsRef)
	}
}

func TestWebReviews(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/biz/joe/review_feed") && r.URL.Path != "/biz/joe/review_feed" {
			t.Errorf("path %q", r.URL.Path)
		}
		if r.URL.Query().Get("start") == "0" {
			_, _ = w.Write([]byte(`{"pagination":{"totalResults":1},"reviews":[{"id":"r1","rating":4,"comment":{"text":"Nice","language":"en"},"localizedDate":"1/2/2024","user":{"encid":"u9","markupDisplayName":"<b>Sam</b>","displayLocation":"Reno, NV","src":"https://img/u9.jpg","link":"https://www.yelp.com/user_details?userid=u9"},"feedback":{"counts":{"useful":3,"funny":1,"cool":2}},"photos":[{"src":"https://img/p1.jpg"}]}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"pagination":{"totalResults":1},"reviews":[]}`))
	})
	c := newTestClient(t, h, planeWeb, "")
	rs, err := c.Reviews(context.Background(), "joe", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 {
		t.Fatalf("want 1 review, got %d", len(rs))
	}
	r := rs[0]
	if r.Author != "Sam" || r.AuthorID != "u9" || r.AuthorLocation != "Reno, NV" {
		t.Errorf("bad author fields: %+v", r)
	}
	if r.Useful != 3 || r.Funny != 1 || r.Cool != 2 || len(r.Photos) != 1 {
		t.Errorf("bad feedback/photos: %+v", r)
	}
	if r.Business != "joe" {
		t.Errorf("missing business edge: %q", r.Business)
	}
}

func TestWallDetection(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte("blocked"))
	})
	c := newTestClient(t, h, planeWeb, "")
	_, err := c.Business(context.Background(), "joe-coffee")
	if !errors.Is(err, ErrBlocked) {
		t.Errorf("want ErrBlocked, got %v", err)
	}
}

func TestChallengeBodyIsWall(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><div id="px-captcha"></div>Please verify you are a human</html>`))
	})
	c := newTestClient(t, h, planeWeb, "")
	_, err := c.Business(context.Background(), "joe-coffee")
	if !errors.Is(err, ErrBlocked) {
		t.Errorf("want ErrBlocked on challenge body, got %v", err)
	}
}

func TestNotFound(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})
	c := newTestClient(t, h, planeFusion, "KEY")
	_, err := c.Business(context.Background(), "ghost")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}
