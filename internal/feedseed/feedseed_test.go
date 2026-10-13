package feedseed

import (
	"path/filepath"
	"testing"
)

func TestLoadSitesFile(t *testing.T) {
	path := filepath.Join("..", "..", "feeds", "feeds.json")
	sites, err := LoadSitesFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) < 1 {
		t.Fatal("expected feeds")
	}
}
