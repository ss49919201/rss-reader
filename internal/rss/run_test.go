package rss

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ss49919201/rss-reader/internal/site"
	"github.com/ss49919201/rss-reader/internal/store"
)

var _ store.Store = (*fakeStore)(nil)

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

type fakeStore struct {
	sites         []site.Site
	lastRunAt     time.Time
	notifiedLinks map[string]time.Time
}

func newFakeStore() *fakeStore {
	return &fakeStore{notifiedLinks: make(map[string]time.Time)}
}

func (f *fakeStore) Close() error { return nil }

func (f *fakeStore) ListSites(context.Context) ([]site.Site, error) {
	return append([]site.Site(nil), f.sites...), nil
}

func (f *fakeStore) AddSite(_ context.Context, st site.Site) error {
	if err := site.Validate([]site.Site{st}); err != nil {
		return err
	}
	f.sites = append(f.sites, st)
	return nil
}

func (f *fakeStore) RemoveSite(_ context.Context, id string) error {
	for i, s := range f.sites {
		if s.ID == id {
			f.sites = append(f.sites[:i], f.sites[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("サイトが見つかりません: %s", id)
}

func (f *fakeStore) LastRun(context.Context) (time.Time, error) {
	return f.lastRunAt, nil
}

func (f *fakeStore) SetLastRun(_ context.Context, t time.Time) error {
	f.lastRunAt = t
	return nil
}

func (f *fakeStore) HasNotifiedLink(_ context.Context, link string) (bool, error) {
	_, ok := f.notifiedLinks[link]
	return ok, nil
}

func (f *fakeStore) RememberLink(_ context.Context, link string, at time.Time) error {
	if link == "" {
		return nil
	}
	f.notifiedLinks[link] = at
	return nil
}

func TestRunNotifiesNewArticles(t *testing.T) {
	st := newFakeStore()
	st.lastRunAt = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	st.sites = []site.Site{
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
	st := newFakeStore()
	st.sites = []site.Site{{ID: "demo", Name: "Demo", URL: "https://example.com/rss"}}
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
	if st.lastRunAt.IsZero() {
		t.Fatal("last run not saved")
	}
}

func TestRunRejectsEmptySites(t *testing.T) {
	err := Run(context.Background(), newFakeStore(), &recordingNotifier{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
