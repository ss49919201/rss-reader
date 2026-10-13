package store

import (
	"context"
	"fmt"
	"time"

	"github.com/ss49919201/rss-reader/internal/site"
)

// Fake はテスト用のインメモリ Store。
type Fake struct {
	Sites         []site.Site
	LastRunAt     time.Time
	NotifiedLinks map[string]time.Time
}

// NewFake は空の Fake を返す。
func NewFake() *Fake {
	return &Fake{NotifiedLinks: make(map[string]time.Time)}
}

func (f *Fake) Close() error { return nil }

func (f *Fake) ListSites(context.Context) ([]site.Site, error) {
	return append([]site.Site(nil), f.Sites...), nil
}

func (f *Fake) AddSite(_ context.Context, st site.Site) error {
	if err := site.Validate([]site.Site{st}); err != nil {
		return err
	}
	f.Sites = append(f.Sites, st)
	return nil
}

func (f *Fake) RemoveSite(_ context.Context, id string) error {
	for i, s := range f.Sites {
		if s.ID == id {
			f.Sites = append(f.Sites[:i], f.Sites[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("サイトが見つかりません: %s", id)
}

func (f *Fake) LastRun(context.Context) (time.Time, error) {
	return f.LastRunAt, nil
}

func (f *Fake) SetLastRun(_ context.Context, t time.Time) error {
	f.LastRunAt = t
	return nil
}

func (f *Fake) HasNotifiedLink(_ context.Context, link string) (bool, error) {
	_, ok := f.NotifiedLinks[link]
	return ok, nil
}

func (f *Fake) RememberLink(_ context.Context, link string, at time.Time) error {
	if link == "" {
		return nil
	}
	f.NotifiedLinks[link] = at
	return nil
}
