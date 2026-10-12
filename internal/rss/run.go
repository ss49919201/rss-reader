package rss

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// SiteErrors は1件以上のサイト取得が失敗したことを表す。
type SiteErrors struct {
	Messages []string
}

func (e *SiteErrors) Error() string {
	return "一部のサイトを取得できませんでした: " + strings.Join(e.Messages, "; ")
}

// Run は登録サイトを取得し、未通知の新着記事を notifier へ送る。
// 取得に失敗したサイトがあっても残りは試し、失敗をまとめて SiteErrors で返す。
// 通知に成功した記事は state に反映して statePath に保存する。
func Run(ctx context.Context, sites []Site, notifier Notifier, statePath string) error {
	if err := Validate(sites); err != nil {
		return err
	}
	if notifier == nil {
		return fmt.Errorf("notifier が未設定です")
	}

	st, err := LoadState(statePath)
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
		for _, article := range page.Articles {
			if !st.IsNew(article) {
				continue
			}
			if err := notifier.NotifyArticle(ctx, page.Site, article); err != nil {
				notifyErrs = append(notifyErrs, fmt.Sprintf("%s / %s: %v", page.Site.Name, article.Title, err))
				continue
			}
			link := safeURL(article.Link, "")
			if link != "" {
				st.rememberLink(link)
			}
		}
	}

	st.LastRun = runAt
	if err := SaveState(statePath, st); err != nil {
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
