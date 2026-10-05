package rss

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
)

type memStore struct {
	mu      sync.Mutex
	objs    map[string][]byte
	puts    int
	failPut bool
}

func newMemStore() *memStore {
	return &memStore{objs: map[string][]byte{}}
}

func (m *memStore) loc(bucket, key string) string {
	return bucket + "/" + key
}

func (m *memStore) Get(ctx context.Context, bucket, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.objs[m.loc(bucket, key)]
	if !ok {
		return nil, errNotFound
	}
	return bytes.Clone(data), nil
}

func (m *memStore) Put(ctx context.Context, bucket, key string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failPut {
		return errors.New("put failed")
	}
	m.puts++
	m.objs[m.loc(bucket, key)] = bytes.Clone(data)
	return nil
}

func TestLoadSitesSeedsDefaultsOnce(t *testing.T) {
	ctx := context.Background()
	store := newMemStore()
	sites, err := LoadSites(ctx, store, DefaultBucket, SitesObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != len(defaultSites) {
		t.Fatalf("len = %d", len(sites))
	}
	for i := range defaultSites {
		if sites[i] != defaultSites[i] {
			t.Fatalf("sites[%d] = %#v", i, sites[i])
		}
	}
	if store.puts != 1 {
		t.Fatalf("puts = %d", store.puts)
	}
	blob, err := store.Get(ctx, DefaultBucket, SitesObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(blob, []byte("SQLite format 3\x00")) {
		t.Fatalf("prefix = %q", blob[:min(16, len(blob))])
	}
	if !bytes.Contains(blob, []byte("https://go.dev/blog/feed.atom")) {
		t.Fatal("database missing default feed")
	}

	if _, err := LoadSites(ctx, store, DefaultBucket, SitesObjectKey); err != nil {
		t.Fatal(err)
	}
	if store.puts != 1 {
		t.Fatalf("read path wrote again: puts = %d", store.puts)
	}
}

func TestAddAndRemoveSites(t *testing.T) {
	ctx := context.Background()
	store := newMemStore()
	extra := Site{ID: "extra", Name: "Extra", URL: "https://example.com/feed.xml"}
	if err := AddSite(ctx, store, DefaultBucket, SitesObjectKey, extra); err != nil {
		t.Fatal(err)
	}
	sites, err := LoadSites(ctx, store, DefaultBucket, SitesObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != len(defaultSites)+1 || sites[len(sites)-1] != extra {
		t.Fatalf("sites = %#v", sites)
	}

	if err := AddSite(ctx, store, DefaultBucket, SitesObjectKey, Site{
		ID: "go-blog", Name: "Go Blog Updated", URL: "https://example.com/go.xml",
	}); err != nil {
		t.Fatal(err)
	}
	sites, err = LoadSites(ctx, store, DefaultBucket, SitesObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	if sites[0].ID != "go-blog" || sites[0].Name != "Go Blog Updated" || sites[0].URL != "https://example.com/go.xml" {
		t.Fatalf("updated = %#v", sites[0])
	}
	if sites[1].ID != "zenn" || sites[len(sites)-1].ID != "extra" {
		t.Fatalf("order = %#v", sites)
	}

	if err := RemoveSite(ctx, store, DefaultBucket, SitesObjectKey, "extra"); err != nil {
		t.Fatal(err)
	}
	sites, err = LoadSites(ctx, store, DefaultBucket, SitesObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != len(defaultSites) {
		t.Fatalf("len = %d %#v", len(sites), sites)
	}
	for _, site := range sites {
		if site.ID == "extra" {
			t.Fatal("extra still registered")
		}
	}
}

func TestAddRejectsInvalidSiteWithoutUpload(t *testing.T) {
	ctx := context.Background()
	store := newMemStore()
	err := AddSite(ctx, store, DefaultBucket, SitesObjectKey, Site{ID: "../x", Name: "x", URL: "https://example.com/feed"})
	if err == nil {
		t.Fatal("expected error")
	}
	if store.puts != 0 {
		t.Fatalf("puts = %d", store.puts)
	}
	if _, err := store.Get(ctx, DefaultBucket, SitesObjectKey); !errors.Is(err, errNotFound) {
		t.Fatal(err)
	}
}

func TestRemoveUnknownDoesNotUpload(t *testing.T) {
	ctx := context.Background()
	store := newMemStore()
	err := RemoveSite(ctx, store, DefaultBucket, SitesObjectKey, "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if store.puts != 0 {
		t.Fatalf("puts = %d", store.puts)
	}
}

func TestRemoveSeededSiteUploadsRemaining(t *testing.T) {
	ctx := context.Background()
	store := newMemStore()
	if err := RemoveSite(ctx, store, DefaultBucket, SitesObjectKey, "zenn"); err != nil {
		t.Fatal(err)
	}
	sites, err := LoadSites(ctx, store, DefaultBucket, SitesObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != len(defaultSites)-1 {
		t.Fatalf("sites = %#v", sites)
	}
	for _, site := range sites {
		if site.ID == "zenn" {
			t.Fatal("zenn still registered")
		}
	}
	if store.puts != 1 {
		t.Fatalf("puts = %d", store.puts)
	}
}

func TestFailedUploadLeavesObjectMissing(t *testing.T) {
	ctx := context.Background()
	store := newMemStore()
	store.failPut = true
	if _, err := LoadSites(ctx, store, DefaultBucket, SitesObjectKey); err == nil {
		t.Fatal("expected error")
	}
	if _, err := store.Get(ctx, DefaultBucket, SitesObjectKey); !errors.Is(err, errNotFound) {
		t.Fatal(err)
	}
}

func TestCorruptObjectIsRejected(t *testing.T) {
	ctx := context.Background()
	store := newMemStore()
	if err := store.Put(ctx, DefaultBucket, SitesObjectKey, []byte("not a database")); err != nil {
		t.Fatal(err)
	}
	puts := store.puts
	_, err := LoadSites(ctx, store, DefaultBucket, SitesObjectKey)
	if err == nil {
		t.Fatal("expected error")
	}
	if store.puts != puts {
		t.Fatal("corrupt object was replaced")
	}
}

func TestRejectsBadLocation(t *testing.T) {
	ctx := context.Background()
	store := newMemStore()
	if _, err := LoadSites(ctx, store, "Bad_Bucket", SitesObjectKey); err == nil {
		t.Fatal("expected bucket error")
	}
	if _, err := LoadSites(ctx, store, DefaultBucket, "../sites.db"); err == nil {
		t.Fatal("expected key error")
	}
}
