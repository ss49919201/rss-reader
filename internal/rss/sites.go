package rss

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Site は取得対象の1サイト。
type Site struct {
	ID   string
	Name string
	URL  string
}

// idPattern はHTMLファイル名に使うID。ディレクトリ脱出ができない形だけ許可する。
var idPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// defaultSites は R2 に sites.db がまだ無いときの初期データ。
// 正本は R2 上の SQLite なので、2回目以降の起動ではここを読まない。
var defaultSites = []Site{
	{ID: "go-blog", Name: "Go Blog", URL: "https://go.dev/blog/feed.atom"},
	{ID: "zenn", Name: "Zenn", URL: "https://zenn.dev/feed"},
	{ID: "github-blog", Name: "GitHub Blog", URL: "https://github.blog/feed/"},
	{ID: "hatena-hotentry", Name: "はてなブックマーク 人気エントリー", URL: "https://b.hatena.ne.jp/hotentry.rss"},
}

// Validate はサイト定義がHTMLへ安全に保存できる形かを確認する。
func Validate(sites []Site) error {
	if len(sites) == 0 {
		return fmt.Errorf("サイトが登録されていません")
	}
	seen := make(map[string]struct{}, len(sites))
	for _, site := range sites {
		if !idPattern.MatchString(site.ID) || len(site.ID) > 64 {
			return fmt.Errorf("サイトIDが不正です: %q", site.ID)
		}
		if _, ok := seen[site.ID]; ok {
			return fmt.Errorf("サイトIDが重複しています: %s", site.ID)
		}
		seen[site.ID] = struct{}{}
		if strings.TrimSpace(site.Name) == "" {
			return fmt.Errorf("サイト名が空です: %s", site.ID)
		}
		if err := validateFeedURL(site.URL); err != nil {
			return fmt.Errorf("%s: %w", site.ID, err)
		}
	}
	return nil
}

func validateFeedURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("フィードURLを解釈できません: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if (scheme != "http" && scheme != "https") || u.Host == "" {
		return fmt.Errorf("フィードURLは http または https にしてください: %s", raw)
	}
	return nil
}
