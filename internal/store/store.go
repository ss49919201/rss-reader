package store

import (
	"context"
	"time"

	"github.com/ss49919201/rss-reader/internal/site"
)

// Store は登録サイトと通知状態を保持する。
type Store interface {
	ListSites(ctx context.Context) ([]site.Site, error)
	AddSite(ctx context.Context, site site.Site) error
	RemoveSite(ctx context.Context, id string) error
	LastRun(ctx context.Context) (time.Time, error)
	SetLastRun(ctx context.Context, t time.Time) error
	HasNotifiedLink(ctx context.Context, link string) (bool, error)
	RememberLink(ctx context.Context, link string, at time.Time) error
	Close() error
}
