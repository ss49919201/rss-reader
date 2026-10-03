package rss

import "testing"

func TestRegisteredSites(t *testing.T) {
	if err := Validate(Sites); err != nil {
		t.Fatal(err)
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
