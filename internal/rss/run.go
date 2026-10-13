package rss

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ss49919201/rss-reader/internal/site"
	"github.com/ss49919201/rss-reader/internal/store"
)

// SiteErrors は1件以上のサイト取得が失敗したことを表す。
type SiteErrors struct {
	Messages []string
}

func (e *SiteErrors) Error() string {
	return "一部のサイトを取得できませんでした: " + strings.Join(e.Messages, "; ")
}

// Run は store の登録サイトを取得し、未通知の新着記事を notifier へ送る。
func Run(ctx context.Context, st store.Store, notifier Notifier) error {
	if st == nil {
		return fmt.Errorf("store が未設定です")
	}
	if notifier == nil {
		return fmt.Errorf("notifier が未設定です")
	}

	sites, err := st.ListSites(ctx)
	if err != nil {
		return err
	}
	if err := site.Validate(sites); err != nil {
		return err
	}

	lastRun, err := st.LastRun(ctx)
	if err != nil {
		return err
	}
	runAt := time.Now()

	pages := FetchAll(ctx, sites)
	var messages []string
	var notifyErrs []string

	for _, page := range pages {
		if page.Error != "" {
			messages = append(messages, fmt.Sprintf("%s: %s", page.Site.Name, page.Error))
			continue
		}
		hasLink := func(link string) bool {
			ok, err := st.HasNotifiedLink(ctx, link)
			if err != nil {
				return true
			}
			return ok
		}
		for _, article := range page.Articles {
			if !isNewArticle(lastRun, hasLink, article) {
				continue
			}
			if err := notifier.NotifyArticle(ctx, page.Site, article); err != nil {
				notifyErrs = append(notifyErrs, fmt.Sprintf("%s / %s: %v", page.Site.Name, article.Title, err))
				continue
			}
			link := SafeURL(article.Link, "")
			if link != "" {
				if err := st.RememberLink(ctx, link, runAt); err != nil {
					return err
				}
			}
		}
	}

	if err := st.SetLastRun(ctx, runAt); err != nil {
		return err
	}

	if len(notifyErrs) > 0 {
		return fmt.Errorf("Slack 通知に失敗しました: %s", strings.Join(notifyErrs, "; "))
	}
	if len(messages) > 0 {
		return &SiteErrors{Messages: messages}
	}
	return nil
}

func isNewArticle(lastRun time.Time, hasLink func(string) bool, article Article) bool {
	if lastRun.IsZero() {
		return false
	}
	if !article.Published.IsZero() {
		return article.Published.After(lastRun)
	}
	link := SafeURL(article.Link, "")
	if link == "" {
		return false
	}
	return !hasLink(link)
}
