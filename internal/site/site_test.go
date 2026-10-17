package site

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidate(t *testing.T) {
	ok := []Site{{ID: "go-blog", Name: "Go", URL: "https://example.com/feed"}}
	if err := Validate(ok); err != nil {
		t.Fatal(err)
	}

	cases := [][]Site{
		nil,
		{{ID: "../etc", Name: "x", URL: "https://example.com/feed"}},
		{{ID: "a", Name: "x", URL: "https://example.com"}, {ID: "a", Name: "y", URL: "https://example.com"}},
		{{ID: "a", Name: "", URL: "https://example.com"}},
		{{ID: "a", Name: "x", URL: "file:///tmp/feed.xml"}},
		{{ID: "a", Name: "x", URL: "javascript:alert(1)"}},
	}
	for _, sites := range cases {
		if err := Validate(sites); err == nil {
			t.Fatalf("want error for %#v", sites)
		}
	}
}

func TestLoadSitesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "feeds.json")
	if err := os.WriteFile(path, []byte(`[{"id":"demo","name":"Demo","url":"https://example.com/rss"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	sites, err := LoadSitesFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 || sites[0].ID != "demo" {
		t.Fatalf("sites=%#v", sites)
	}
}
