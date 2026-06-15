package yelp

import (
	"errors"
	"testing"

	"github.com/tamnd/any-cli/kit"
)

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "yelp" {
		t.Errorf("scheme = %q", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Identity.Binary != "yelp" {
		t.Errorf("bad info: %+v", info)
	}
}

func TestDomainClassifyLocate(t *testing.T) {
	d := Domain{}
	kind, id, err := d.Classify("https://www.yelp.com/biz/joe-coffee")
	if err != nil || kind != "biz" || id != "joe-coffee" {
		t.Fatalf("Classify = (%q,%q,%v)", kind, id, err)
	}
	u, err := d.Locate(kind, id)
	if err != nil || u != BaseURL+"/biz/joe-coffee" {
		t.Errorf("Locate = (%q,%v)", u, err)
	}
	if _, _, err := d.Classify("!!!"); err == nil {
		t.Error("Classify of junk should error")
	}
	if _, err := d.Locate("nope", "x"); err == nil {
		t.Error("Locate of unknown kind should error")
	}
}

func TestClientFromConfig(t *testing.T) {
	t.Setenv("YELP_API_KEY", "envkey")
	cfg := kit.Config{
		Extra: map[string]string{
			"plane":    "fusion",
			"locale":   "fr_FR",
			"location": "Paris",
			"sort":     "rating",
			"price":    "1,2",
		},
	}
	c := ClientFromConfig(cfg)
	if c.apiKey != "envkey" {
		t.Errorf("key not read from env: %q", c.apiKey)
	}
	if c.Plane != "fusion" || c.Locale != "fr_FR" || c.Location != "Paris" || c.Sort != "rating" || c.Price != "1,2" {
		t.Errorf("bad config mapping: %+v", c)
	}
	if !c.usesFusion() {
		t.Error("forced fusion should use fusion")
	}
}

func TestMapErr(t *testing.T) {
	cases := []error{ErrNotFound, ErrRateLimited, ErrBlocked, ErrNeedKey, ErrKeyRejected}
	for _, e := range cases {
		if got := mapErr(e); got == nil {
			t.Errorf("mapErr(%v) returned nil", e)
		}
	}
	if mapErr(nil) != nil {
		t.Error("mapErr(nil) should be nil")
	}
	if !errors.Is(ErrNeedKey, ErrNeedKey) {
		t.Error("sanity")
	}
}
