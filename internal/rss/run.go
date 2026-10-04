package rss

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// SiteErrors はHTMLを保存したあとに、1件以上のサイト取得が失敗したことを表す。
type SiteErrors struct {
	Messages []string
}

func (e *SiteErrors) Error() string {
	return "一部のサイトを取得できませんでした: " + strings.Join(e.Messages, "; ")
}

// RunOnce は登録サイトを取得し、outDir にHTMLを書く。
// 取得に失敗したサイトがあっても、成功分と失敗内容をHTMLへ保存してから SiteErrors を返す。
func RunOnce(ctx context.Context, sites []Site, outDir string) ([]SitePage, error) {
	if err := Validate(sites); err != nil {
		return nil, err
	}
	pages := FetchAll(ctx, sites)
	if err := WriteHTML(outDir, time.Now(), pages); err != nil {
		return pages, err
	}
	var messages []string
	for _, page := range pages {
		if page.Error != "" {
			messages = append(messages, fmt.Sprintf("%s: %s", page.Site.Name, page.Error))
		}
	}
	if len(messages) > 0 {
		return pages, &SiteErrors{Messages: messages}
	}
	return pages, nil
}

// Repeat は fn をすぐに1回実行する。interval が正のときは、その間隔で繰り返す。
// キャンセルされると nil を返す。interval が 0 以下のときは1回で終わり、fn のエラーを返す。
func Repeat(ctx context.Context, interval time.Duration, fn func(context.Context) error) error {
	err := fn(ctx)
	if interval <= 0 {
		return err
	}
	if ctx.Err() != nil {
		return nil
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if ctx.Err() != nil {
				return nil
			}
			if err := fn(ctx); err != nil && ctx.Err() != nil {
				return nil
			}
		}
	}
}
