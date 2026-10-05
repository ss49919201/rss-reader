package sitedb

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/ss49919201/rss-reader/internal/rss"
)

// fakeR2 は GET と条件付き PUT だけを実装した S3 互換サーバー。
type fakeR2 struct {
	mu        sync.Mutex
	objects   map[string][]byte
	puts      int
	beforePut func()
}

func (f *fakeR2) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		f.mu.Lock()
		data, ok := f.objects[r.URL.Path]
		f.mu.Unlock()
		if !ok {
			s3Error(w, http.StatusNotFound, "NoSuchKey")
			return
		}
		w.Header().Set("ETag", etagOf(data))
		_, _ = w.Write(data)
	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			s3Error(w, http.StatusBadRequest, "BadRequest")
			return
		}
		if hook := f.takeBeforePut(); hook != nil {
			hook()
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		current, exists := f.objects[r.URL.Path]
		if r.Header.Get("If-None-Match") == "*" && exists {
			s3Error(w, http.StatusPreconditionFailed, "PreconditionFailed")
			return
		}
		if m := r.Header.Get("If-Match"); m != "" && (!exists || m != etagOf(current)) {
			s3Error(w, http.StatusPreconditionFailed, "PreconditionFailed")
			return
		}
		f.objects[r.URL.Path] = body
		f.puts++
		w.Header().Set("ETag", etagOf(body))
	default:
		s3Error(w, http.StatusMethodNotAllowed, "MethodNotAllowed")
	}
}

func (f *fakeR2) takeBeforePut() func() {
	f.mu.Lock()
	defer f.mu.Unlock()
	hook := f.beforePut
	f.beforePut = nil
	return hook
}

func (f *fakeR2) putCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.puts
}

func (f *fakeR2) object(path string) ([]byte, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.objects[path]
	return data, ok
}

func etagOf(data []byte) string {
	sum := md5.Sum(data)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

func s3Error(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>`+code+`</Code><Message>`+code+`</Message></Error>`)
}

func newTestStore(t *testing.T) (*Store, *fakeR2) {
	t.Helper()
	fake := &fakeR2{objects: map[string][]byte{}}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	store := New(Config{
		AccessKeyID:     "test",
		SecretAccessKey: "test",
		Bucket:          "test-bucket",
		Key:             DefaultObjectKey,
		Endpoint:        srv.URL,
	})
	return store, fake
}

const objectPath = "/test-bucket/sites.db"

func TestDefaultSitesAreValid(t *testing.T) {
	if err := rss.Validate(defaultSites); err != nil {
		t.Fatal(err)
	}
}

func TestLoadSeedsMissingDatabase(t *testing.T) {
	store, fake := newTestStore(t)
	ctx := context.Background()

	sites, err := store.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sites, defaultSites) {
		t.Fatalf("sites = %#v", sites)
	}
	data, ok := fake.object(objectPath)
	if !ok {
		t.Fatalf("database was not uploaded to %s", objectPath)
	}
	if !strings.HasPrefix(string(data), "SQLite format 3\x00") {
		t.Fatal("uploaded object is not a SQLite database")
	}

	if _, err := store.Load(ctx); err != nil {
		t.Fatal(err)
	}
	if fake.putCount() != 1 {
		t.Fatalf("puts = %d, want 1", fake.putCount())
	}
}

func TestUploadedObjectIsReadableSQLite(t *testing.T) {
	store, fake := newTestStore(t)
	if _, err := store.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	data, _ := fake.object(objectPath)
	path := filepath.Join(t.TempDir(), "sites.db")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM sites`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != len(defaultSites) {
		t.Fatalf("rows = %d", n)
	}
}

func TestAddAndRemove(t *testing.T) {
	store, fake := newTestStore(t)
	ctx := context.Background()

	example := rss.Site{ID: "example", Name: "Example", URL: "https://example.com/feed.xml"}
	if err := store.Add(ctx, example); err != nil {
		t.Fatal(err)
	}
	sites, err := store.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := append(append([]rss.Site{}, defaultSites...), example)
	if !reflect.DeepEqual(sites, want) {
		t.Fatalf("sites = %#v", sites)
	}

	puts := fake.putCount()
	for _, bad := range []rss.Site{
		example,
		{ID: "../x", Name: "x", URL: "https://example.com"},
		{ID: "x", Name: "x", URL: "javascript:alert(1)"},
		{ID: "x", Name: " ", URL: "https://example.com"},
	} {
		if err := store.Add(ctx, bad); err == nil {
			t.Fatalf("want error for %#v", bad)
		}
	}
	if fake.putCount() != puts {
		t.Fatal("rejected add was uploaded")
	}

	if err := store.Remove(ctx, "zenn"); err != nil {
		t.Fatal(err)
	}
	if err := store.Remove(ctx, "zenn"); err == nil {
		t.Fatal("want error for missing site")
	}
	sites, err = store.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, site := range sites {
		if site.ID == "zenn" {
			t.Fatal("zenn was not removed")
		}
	}
	if len(sites) != len(defaultSites) {
		t.Fatalf("sites = %d", len(sites))
	}
}

func TestAddRetriesOnConcurrentWrite(t *testing.T) {
	store, fake := newTestStore(t)
	ctx := context.Background()
	if _, err := store.Load(ctx); err != nil {
		t.Fatal(err)
	}

	other := New(Config{AccessKeyID: "test", SecretAccessKey: "test", Bucket: "test-bucket", Key: DefaultObjectKey, Endpoint: *store.client.Options().BaseEndpoint})
	fake.beforePut = func() {
		if err := other.Add(ctx, rss.Site{ID: "first", Name: "First", URL: "https://example.com/1"}); err != nil {
			t.Error(err)
		}
	}
	if err := store.Add(ctx, rss.Site{ID: "second", Name: "Second", URL: "https://example.com/2"}); err != nil {
		t.Fatal(err)
	}

	sites, err := store.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(sites))
	for _, site := range sites {
		ids = append(ids, site.ID)
	}
	got := strings.Join(ids, ",")
	if !strings.HasSuffix(got, ",first,second") {
		t.Fatalf("ids = %s", got)
	}
}

func TestConfigFromEnv(t *testing.T) {
	for _, name := range []string{"R2_ACCOUNT_ID", "R2_ACCESS_KEY_ID", "R2_SECRET_ACCESS_KEY", "R2_BUCKET", "R2_OBJECT_KEY", "R2_ENDPOINT"} {
		t.Setenv(name, "")
	}
	_, err := ConfigFromEnv()
	if err == nil {
		t.Fatal("want error")
	}
	for _, name := range []string{"R2_ACCOUNT_ID", "R2_ACCESS_KEY_ID", "R2_SECRET_ACCESS_KEY", "R2_BUCKET"} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("error does not mention %s: %v", name, err)
		}
	}

	t.Setenv("R2_ACCOUNT_ID", "abc123")
	t.Setenv("R2_ACCESS_KEY_ID", "key")
	t.Setenv("R2_SECRET_ACCESS_KEY", "secret")
	t.Setenv("R2_BUCKET", "my-bucket")
	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Key != "sites.db" || cfg.Bucket != "my-bucket" {
		t.Fatalf("cfg = %#v", cfg)
	}
	store := New(cfg)
	if store.Location() != "r2://my-bucket/sites.db" {
		t.Fatalf("location = %s", store.Location())
	}
	if got := *store.client.Options().BaseEndpoint; got != "https://abc123.r2.cloudflarestorage.com" {
		t.Fatalf("endpoint = %s", got)
	}
}
