package rss

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("R2_ACCOUNT_ID", "")
	t.Setenv("R2_ACCESS_KEY_ID", "")
	t.Setenv("R2_SECRET_ACCESS_KEY", "")
	t.Setenv("R2_BUCKET", "")
	t.Setenv("R2_OBJECT_KEY", "")
	t.Setenv("R2_ENDPOINT", "")
	if _, err := ConfigFromEnv(); err == nil || !strings.Contains(err.Error(), "R2_ACCOUNT_ID") {
		t.Fatal(err)
	}

	t.Setenv("R2_ACCOUNT_ID", "0123456789abcdef0123456789abcdef")
	t.Setenv("R2_ACCESS_KEY_ID", "testaccesskey")
	t.Setenv("R2_SECRET_ACCESS_KEY", "testsecretkey")
	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Bucket != DefaultBucket || cfg.ObjectKey != SitesObjectKey {
		t.Fatalf("cfg = %#v", cfg)
	}
	endpoint, err := cfg.endpoint()
	if err != nil {
		t.Fatal(err)
	}
	if endpoint != "https://0123456789abcdef0123456789abcdef.r2.cloudflarestorage.com" {
		t.Fatal(endpoint)
	}

	t.Setenv("R2_BUCKET", "custom-sites")
	t.Setenv("R2_OBJECT_KEY", "feeds/sites.db")
	t.Setenv("R2_ENDPOINT", "http://127.0.0.1:9")
	t.Setenv("R2_ACCOUNT_ID", "")
	cfg, err = ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Bucket != "custom-sites" || cfg.ObjectKey != "feeds/sites.db" || cfg.Endpoint != "http://127.0.0.1:9" {
		t.Fatalf("cfg = %#v", cfg)
	}

	t.Setenv("R2_ENDPOINT", "http://example.com")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("expected http endpoint error")
	}
}

type r2Fake struct {
	srv  *httptest.Server
	mu   sync.Mutex
	objs map[string][]byte
	reqs []string
	puts int
}

func newR2Fake(t *testing.T) *r2Fake {
	t.Helper()
	f := &r2Fake{objs: map[string][]byte{}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *r2Fake) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.reqs = append(f.reqs, r.Method+" "+r.Host+r.URL.RequestURI())
	f.mu.Unlock()
	if r.Header.Get("Authorization") == "" {
		http.Error(w, "missing auth", http.StatusForbidden)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	bucket, key, ok := strings.Cut(path, "/")
	if !ok || bucket == "" || key == "" {
		http.Error(w, "bad path "+r.URL.Path, http.StatusBadRequest)
		return
	}
	loc := bucket + "/" + key
	switch r.Method {
	case http.MethodGet:
		f.mu.Lock()
		data, found := f.objs[loc]
		f.mu.Unlock()
		if !found {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchKey</Code><Message>Not Found</Message></Error>`)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.sqlite3")
		w.Header().Set("ETag", `"sites"`)
		_, _ = w.Write(data)
	case http.MethodPut:
		data, err := io.ReadAll(io.LimitReader(r.Body, maxDBBytes+1))
		if err != nil || len(data) == 0 || len(data) > maxDBBytes {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.objs[loc] = bytes.Clone(data)
		f.puts++
		f.mu.Unlock()
		w.Header().Set("ETag", `"ok"`)
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "method "+r.Method, http.StatusMethodNotAllowed)
	}
}

func (f *r2Fake) client(t *testing.T) *R2 {
	t.Helper()
	t.Setenv("R2_ACCOUNT_ID", "0123456789abcdef0123456789abcdef")
	t.Setenv("R2_ACCESS_KEY_ID", "testaccesskey")
	t.Setenv("R2_SECRET_ACCESS_KEY", "testsecretkey")
	t.Setenv("R2_BUCKET", "")
	t.Setenv("R2_OBJECT_KEY", "")
	t.Setenv("R2_ENDPOINT", f.srv.URL)
	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewR2(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestR2ClientRoundTrip(t *testing.T) {
	fake := newR2Fake(t)
	client := fake.client(t)
	ctx := context.Background()

	_, err := client.Get(ctx, DefaultBucket, SitesObjectKey)
	if !errors.Is(err, errNotFound) {
		t.Fatalf("get missing: %v\nrequests: %v", err, fake.reqs)
	}

	sites, err := LoadSites(ctx, client, DefaultBucket, SitesObjectKey)
	if err != nil {
		t.Fatalf("load: %v\nrequests: %v", err, fake.reqs)
	}
	if len(sites) != len(defaultSites) || sites[0].ID != "go-blog" {
		t.Fatalf("sites = %#v", sites)
	}
	if fake.puts != 1 {
		t.Fatalf("puts = %d requests=%v", fake.puts, fake.reqs)
	}
	for _, req := range fake.reqs {
		if !strings.Contains(req, "/"+DefaultBucket+"/"+SitesObjectKey) {
			t.Fatalf("request path = %s", req)
		}
	}

	blob := fake.objs[DefaultBucket+"/"+SitesObjectKey]
	if !bytes.HasPrefix(blob, []byte("SQLite format 3\x00")) {
		t.Fatalf("stored prefix = %q", blob[:min(32, len(blob))])
	}

	puts := fake.puts
	again, err := LoadSites(ctx, client, DefaultBucket, SitesObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != len(sites) || fake.puts != puts {
		t.Fatalf("second load wrote or changed: puts %d -> %d", puts, fake.puts)
	}

	if err := AddSite(ctx, client, DefaultBucket, SitesObjectKey, Site{
		ID: "extra", Name: "Extra", URL: "https://example.com/rss.xml",
	}); err != nil {
		t.Fatal(err)
	}
	if fake.puts != puts+1 {
		t.Fatalf("puts = %d", fake.puts)
	}
	got, err := LoadSites(ctx, client, DefaultBucket, SitesObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	if got[len(got)-1].ID != "extra" {
		t.Fatalf("sites = %#v", got)
	}
}

func TestR2MissingBucketIsNotMissingObject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchBucket</Code><Message>No such bucket</Message></Error>`)
	}))
	defer srv.Close()

	t.Setenv("R2_ACCOUNT_ID", "0123456789abcdef0123456789abcdef")
	t.Setenv("R2_ACCESS_KEY_ID", "testaccesskey")
	t.Setenv("R2_SECRET_ACCESS_KEY", "testsecretkey")
	t.Setenv("R2_BUCKET", "")
	t.Setenv("R2_OBJECT_KEY", "")
	t.Setenv("R2_ENDPOINT", srv.URL)
	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewR2(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Get(context.Background(), DefaultBucket, SitesObjectKey)
	if err == nil || errors.Is(err, errNotFound) {
		t.Fatalf("err = %v", err)
	}
}
