package rss

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRegisteredSites(t *testing.T) {
	db, err := openSiteDB(filepath.Join(t.TempDir(), "sites.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer db.close()
	if err := db.seed(context.Background(), defaultSites); err != nil {
		t.Fatal(err)
	}
	sites, err := db.list(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(sites); err != nil {
		t.Fatal(err)
	}
	if len(sites) != len(defaultSites) {
		t.Fatalf("sites = %#v", sites)
	}
	for i := range defaultSites {
		if sites[i] != defaultSites[i] {
			t.Fatalf("sites[%d] = %#v", i, sites[i])
		}
	}
}

func TestValidate(t *testing.T) {
	ok := []Site{{ID: "go-blog", Name: "Go", URL: "https://example.com/feed"}}
	if err := Validate(ok); err != nil {
		t.Fatal(err)
	}

	cases := [][]Site{
		nil,
		{{ID: "../etc", Name: "x", URL: "https://example.com/feed"}},
		{{ID: "a", Name: "x", URL: "https://example.com"}, {ID: "a", Name: "y", URL: "https://example.com"}},
		{{ID: "a", Name: " ", URL: "https://example.com"}},
		{{ID: "a", Name: "x", URL: "file:///tmp/feed.xml"}},
		{{ID: "a", Name: "x", URL: "javascript:alert(1)"}},
	}
	for _, sites := range cases {
		if err := Validate(sites); err == nil {
			t.Fatalf("want error for %#v", sites)
		}
	}
}
