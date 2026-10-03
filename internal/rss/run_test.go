package rss

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunOnceWritesPartialResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/bad") {
			http.Error(w, "nope", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(sampleRSS))
	}))
	defer srv.Close()

	sites := []Site{
		{ID: "demo", Name: "Demo", URL: srv.URL + "/rss"},
		{ID: "broken", Name: "Broken", URL: srv.URL + "/bad"},
	}
	dir := t.TempDir()
	pages, err := RunOnce(context.Background(), sites, dir)
	var siteErr *SiteErrors
	if !errors.As(err, &siteErr) {
		t.Fatalf("err = %v", err)
	}
	if len(pages) != 2 || pages[0].Error != "" || pages[1].Error == "" {
		t.Fatalf("pages = %#v", pages)
	}
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "broken.html")); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "demo.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "新しい記事") {
		t.Fatal("saved html missing article")
	}
}

func TestRunOnceRejectsEmptySites(t *testing.T) {
	_, err := RunOnce(context.Background(), nil, t.TempDir())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRepeatUntilCancel(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(sampleRSS))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dir := t.TempDir()
	done := make(chan error, 1)
	go func() {
		done <- Repeat(ctx, 30*time.Millisecond, func(ctx context.Context) error {
			_, err := RunOnce(ctx, []Site{{ID: "demo", Name: "Demo", URL: srv.URL}}, dir)
			if hits.Load() >= 2 {
				cancel()
			}
			return err
		})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("repeat did not stop")
	}
	if hits.Load() < 2 {
		t.Fatalf("hits = %d", hits.Load())
	}
}

func TestRepeatOnceReturnsError(t *testing.T) {
	err := Repeat(context.Background(), 0, func(context.Context) error {
		return errors.New("boom")
	})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("err = %v", err)
	}
}
