package rss

import (
	"context"
	"errors"
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

func TestRunNotifiesNewArticles(t *testing.T) {
	st := store.NewFake()
	st.LastRunAt = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	st.Sites = []site.Site{
		{ID: "demo", Name: "Demo", URL: "https://example.com/rss"},
		{ID: "broken", Name: "Broken", URL: "https://example.com/bad"},
	}
	fetch := func(_ context.Context, sites []site.Site) []SitePage {
		return []SitePage{
			{
				Site: sites[0],
				Articles: []Article{{
					Title:     "新しい記事",
					Link:      "https://example.com/new",
					Published: time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC),
				}},
			},
			{Site: sites[1], Error: "HTTP 502"},
		}
	}
	rec := &recordingNotifier{}
	err := Run(context.Background(), st, rec, fetch)
	var siteErr *SiteErrors
	if !errors.As(err, &siteErr) {
		t.Fatalf("err = %v", err)
	}
	rec.mu.Lock()
	calls := append([]string(nil), rec.calls...)
	rec.mu.Unlock()
	if len(calls) != 1 || calls[0] != "Demo:新しい記事" {
		t.Fatalf("notifications = %#v", calls)
	}
}

func TestRunSkipsFirstExecution(t *testing.T) {
	st := store.NewFake()
	st.Sites = []site.Site{{ID: "demo", Name: "Demo", URL: "https://example.com/rss"}}
	fetch := func(_ context.Context, sites []site.Site) []SitePage {
		return []SitePage{{
			Site: sites[0],
			Articles: []Article{{
				Title:     "新しい記事",
				Link:      "https://example.com/new",
				Published: time.Now(),
			}},
		}}
	}
	rec := &recordingNotifier{}
	if err := Run(context.Background(), st, rec, fetch); err != nil {
		t.Fatal(err)
	}
	rec.mu.Lock()
	n := len(rec.calls)
	rec.mu.Unlock()
	if n != 0 {
		t.Fatalf("first run notified %d articles", n)
	}
	if st.LastRunAt.IsZero() {
		t.Fatal("last run not saved")
	}
}

func TestRunRejectsEmptySites(t *testing.T) {
	err := Run(context.Background(), store.NewFake(), &recordingNotifier{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
