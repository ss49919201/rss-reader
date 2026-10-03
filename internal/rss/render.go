package rss

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"time"
)

const pageCSS = `
:root {
  color-scheme: light;
  --bg: #f3efe6;
  --paper: #fffdf8;
  --ink: #1f1a14;
  --muted: #6b645a;
  --line: #e6dfd2;
  --accent: #0b6b4f;
  --accent-soft: #e7f5ef;
  --danger: #8d2f2f;
  --danger-bg: #fdecec;
}
* { box-sizing: border-box; }
body {
  margin: 0;
  background:
    radial-gradient(1100px 420px at 8% -10%, #efe4cc 0%, transparent 55%),
    var(--bg);
  color: var(--ink);
  font-family: "Hiragino Sans", "Noto Sans CJK JP", "Noto Sans JP", "Yu Gothic", sans-serif;
  line-height: 1.7;
}
a { color: var(--accent); }
header.page, main { max-width: 860px; margin: 0 auto; }
header.page { padding: 40px 24px 0; }
main { padding: 8px 24px 64px; }
h1 { font-size: 34px; font-weight: 650; letter-spacing: 0.01em; margin: 0 0 8px; }
.meta { color: var(--muted); font-size: 14px; margin: 0; }
.site {
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 16px;
  padding: 8px 22px 18px;
  margin: 16px 0;
}
.site-head { display: flex; justify-content: space-between; gap: 12px; align-items: baseline; padding-top: 14px; }
.site h2 { font-size: 22px; margin: 0; }
.site h2 a { color: inherit; text-decoration: none; }
.site h2 a:hover { color: var(--accent); }
.count {
  background: var(--accent-soft);
  color: var(--accent);
  border-radius: 999px;
  padding: 2px 10px;
  font-size: 13px;
  white-space: nowrap;
}
.feed-link { color: var(--muted); font-size: 13px; }
article { padding: 12px 0; border-top: 1px solid var(--line); }
article a.title { color: var(--ink); font-weight: 650; text-decoration: none; }
article a.title:hover { color: var(--accent); }
time, .summary { display: block; }
time { color: var(--muted); font-size: 13px; }
.summary { margin: 4px 0 0; font-size: 15px; }
.error, .empty {
  border-radius: 10px;
  padding: 10px 12px;
  margin: 12px 0 0;
}
.error { background: var(--danger-bg); color: var(--danger); }
.empty { background: #f6f3ec; color: var(--muted); }
.more { display: inline-block; margin-top: 8px; font-size: 14px; }
.back { font-size: 14px; }
footer { color: var(--muted); font-size: 13px; margin-top: 28px; }
@media (max-width: 640px) {
  header.page, main { padding-left: 16px; padding-right: 16px; }
  h1 { font-size: 28px; }
  .site-head { flex-direction: column; gap: 4px; }
}
`

var pageTmpl = template.Must(template.New("pages").Parse(`
{{define "index"}}<!DOCTYPE html>
<html lang="ja">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>RSSリーダー</title>
<style>` + pageCSS + `</style>
</head>
<body>
<header class="page">
  <h1>RSSリーダー</h1>
  <p class="meta">{{.FetchedAt}} 取得 · {{.OKCount}}件成功{{if .FailCount}} · {{.FailCount}}件失敗{{end}}</p>
</header>
<main>
{{range .Sites}}
  <section class="site">
    <div class="site-head">
      <h2><a href="{{.File}}">{{.Name}}</a></h2>
      {{if not .Error}}<span class="count">{{.Count}}件</span>{{end}}
    </div>
    {{if .SourceURL}}<a class="feed-link" href="{{.SourceURL}}">フィード</a>{{end}}
    {{if .Error}}
      <p class="error">取得できませんでした: {{.Error}}</p>
    {{else if not .Articles}}
      <p class="empty">新しい記事はありません。</p>
    {{else}}
      {{range .Articles}}
      <article>
        {{if .HasLink}}<a class="title" href="{{.Link}}" target="_blank" rel="noopener noreferrer">{{.Title}}</a>{{else}}<span class="title">{{.Title}}</span>{{end}}
        {{if .When}}<time>{{.When}}</time>{{end}}
        {{if .Summary}}<p class="summary">{{.Summary}}</p>{{end}}
      </article>
      {{end}}
      {{if .More}}<a class="more" href="{{.File}}">全{{.Count}}件を見る</a>{{end}}
    {{end}}
  </section>
{{end}}
  <footer>このページは rss-reader が生成しました。</footer>
</main>
</body>
</html>
{{end}}

{{define "site"}}<!DOCTYPE html>
<html lang="ja">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Name}} · RSSリーダー</title>
<style>` + pageCSS + `</style>
</head>
<body>
<header class="page">
  <p class="back"><a href="index.html">一覧へ戻る</a></p>
  <h1>{{.Name}}</h1>
  <p class="meta">{{.FetchedAt}} 取得{{if .SourceURL}} · <a href="{{.SourceURL}}">フィード</a>{{end}}</p>
</header>
<main>
  <section class="site">
    {{if .Error}}
      <p class="error">取得できませんでした: {{.Error}}</p>
    {{else if not .Articles}}
      <p class="empty">新しい記事はありません。</p>
    {{else}}
      {{range .Articles}}
      <article>
        {{if .HasLink}}<a class="title" href="{{.Link}}" target="_blank" rel="noopener noreferrer">{{.Title}}</a>{{else}}<span class="title">{{.Title}}</span>{{end}}
        {{if .When}}<time>{{.When}}</time>{{end}}
        {{if .Summary}}<p class="summary">{{.Summary}}</p>{{end}}
      </article>
      {{end}}
    {{end}}
  </section>
  <footer>このページは rss-reader が生成しました。</footer>
</main>
</body>
</html>
{{end}}
`))

type articleView struct {
	Title   string
	Link    string
	HasLink bool
	When    string
	Summary string
}

type siteView struct {
	Name      string
	File      string
	SourceURL string
	FetchedAt string
	Error     string
	Count     int
	More      bool
	Articles  []articleView
}

type indexView struct {
	FetchedAt string
	OKCount   int
	FailCount int
	Sites     []siteView
}

// WriteHTML は一覧とサイトごとのHTMLを dir に保存する。
func WriteHTML(dir string, generatedAt time.Time, pages []SitePage) error {
	for _, page := range pages {
		if _, err := sitePath(dir, page.Site.ID); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	index := newIndexView(generatedAt, pages)
	if err := writeTemplate(filepath.Join(dir, "index.html"), "index", index); err != nil {
		return err
	}
	for _, page := range pages {
		path, err := sitePath(dir, page.Site.ID)
		if err != nil {
			return err
		}
		if err := writeTemplate(path, "site", newSiteView(page, len(page.Articles))); err != nil {
			return err
		}
	}
	return nil
}

func newIndexView(generatedAt time.Time, pages []SitePage) indexView {
	view := indexView{FetchedAt: formatTime(generatedAt), Sites: make([]siteView, 0, len(pages))}
	for _, page := range pages {
		card := newSiteView(page, 5)
		if page.Error != "" {
			view.FailCount++
		} else {
			view.OKCount++
		}
		view.Sites = append(view.Sites, card)
	}
	return view
}

func newSiteView(page SitePage, limit int) siteView {
	articles := page.Articles
	more := false
	if limit > 0 && len(articles) > limit {
		articles = articles[:limit]
		more = true
	}
	view := siteView{
		Name:      page.Site.Name,
		File:      page.Site.ID + ".html",
		SourceURL: safeURL(page.Site.URL, ""),
		FetchedAt: formatTime(page.FetchedAt),
		Error:     page.Error,
		Count:     len(page.Articles),
		More:      more,
		Articles:  make([]articleView, 0, len(articles)),
	}
	for _, article := range articles {
		link := safeURL(article.Link, "")
		view.Articles = append(view.Articles, articleView{
			Title:   article.Title,
			Link:    link,
			HasLink: link != "",
			When:    formatTime(article.Published),
			Summary: article.Summary,
		})
	}
	return view
}

func sitePath(dir, id string) (string, error) {
	if !idPattern.MatchString(id) || len(id) > 64 {
		return "", fmt.Errorf("サイトIDが不正です: %q", id)
	}
	return filepath.Join(dir, id+".html"), nil
}

func writeTemplate(path, name string, data any) error {
	var buf bytes.Buffer
	if err := pageTmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return err
	}
	return writeAtomic(path, buf.Bytes())
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".rss-tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if !ok {
			os.Remove(tmp)
		}
	}()
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(0o644); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	ok = true
	return nil
}

var jst = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		return time.FixedZone("JST", 9*60*60)
	}
	return loc
}()

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(jst).Format("2006-01-02 15:04") + " JST"
}
