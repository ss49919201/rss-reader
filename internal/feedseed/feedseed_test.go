package feedseed

import "testing"

func TestParseSitesJSON(t *testing.T) {
	data := []byte(`[{"id":"go-blog","name":"Go Blog","url":"https://go.dev/blog/feed.atom"}]`)
	sites, err := ParseSitesJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 || sites[0].ID != "go-blog" {
		t.Fatalf("sites = %#v", sites)
	}
}
