package rss

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ss49919201/rss-reader/internal/site"
	"github.com/ss49919201/rss-reader/internal/store"
)

type recordingNotifier struct {
	mu    sync.Mutex
	calls []string
	err   error
}

func (r *recordingNotifier) NotifyArticle(_ context.Context, st site.Site, article Article) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	r.calls = append(r.calls, st.Name+":"+article.Title)
	return nil
}

func openTestStore(t *testing.T) store.Store {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := store.CreateEmptyFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestRunNotifiesNewArticles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/bad") {
			http.Error(w, "nope", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(sampleRSS))
	}))
	defer srv.Close()

	db := openTestStore(t)
	ctx := context.Background()
	if err := db.SetLastRun(ctx, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	for _, feed := range []site.Site{
		{ID: "demo", Name: "Demo", URL: srv.URL + "/rss"},
		{ID: "broken", Name: "Broken", URL: srv.URL + "/bad"},
	} {
		if err := db.AddSite(ctx, feed); err != nil {
			t.Fatal(err)
		}
	}

	rec := &recordingNotifier{}
	err := Run(ctx, db, rec)
	var siteErr *SiteErrors
	if !errors.As(err, &siteErr) {
		t.Fatalf("err = %v", err)
	}
	rec.mu.Lock()
	calls := append([]string(nil), rec.calls...)
	rec.mu.Unlock()
	if len(calls) < 1 {
		t.Fatalf("notifications = %#v", calls)
	}
}

func TestRunSkipsFirstExecution(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(sampleRSS))
	}))
	defer srv.Close()

	db := openTestStore(t)
	ctx := context.Background()
	if err := db.AddSite(ctx, site.Site{ID: "demo", Name: "Demo", URL: srv.URL + "/rss"}); err != nil {
		t.Fatal(err)
	}
	rec := &recordingNotifier{}
	if err := Run(ctx, db, rec); err != nil {
		t.Fatal(err)
	}
	rec.mu.Lock()
	n := len(rec.calls)
	rec.mu.Unlock()
	if n != 0 {
		t.Fatalf("first run notified %d articles", n)
	}
	last, err := db.LastRun(ctx)
	if err != nil || last.IsZero() {
		t.Fatalf("last run = %v err=%v", last, err)
	}
}

func TestRunRejectsEmptySites(t *testing.T) {
	db := openTestStore(t)
	err := Run(context.Background(), db, &recordingNotifier{})
	if err == nil {
		t.Fatal("expected error")
	}
}
