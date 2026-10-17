package rss

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/ss49919201/andthen/result"
	"github.com/ss49919201/rss-reader/internal/site"
)

const (
	userAgent     = "rss-reader/1.0 (+https://github.com/ss49919201/rss-reader)"
	maxBodyBytes  = 5 << 20
	maxArticles   = 30
	maxSummaryLen = 180
	fetchTimeout  = 30 * time.Second
)

// Article はフィードの1件。
type Article struct {
	Title     string
	Link      string
	Published time.Time
	Summary   string
}

// SitePage は1サイト分の取得結果。Error が空でなければ取得に失敗している。
type SitePage struct {
	Site      site.Site
	Articles  []Article
	FetchedAt time.Time
	Error     string
}

var defaultClient = &http.Client{Timeout: fetchTimeout}

// FetchAll はサイトを並行に取得する。結果の並びは sites と同じ。
func FetchAll(ctx context.Context, sites []site.Site) []SitePage {
	return fetchAll(ctx, defaultClient, sites)
}

func fetchAll(ctx context.Context, client *http.Client, sites []site.Site) []SitePage {
	pages := make([]SitePage, len(sites))
	var wg sync.WaitGroup
	for i, feed := range sites {
		wg.Add(1)
		go func(i int, st site.Site) {
			defer wg.Done()
			pages[i] = Fetch(ctx, client, st)
		}(i, feed)
	}
	wg.Wait()
	return pages
}

// Fetch は1サイトのRSSまたはAtomを取得して記事にする。
func Fetch(ctx context.Context, client *http.Client, st site.Site) SitePage {
	page := SitePage{Site: st, FetchedAt: time.Now()}
	if client == nil {
		client = defaultClient
	}
	articles, err := fetchFeedArticles(ctx, client, st.URL).Get()
	if err != nil {
		page.Error = err.Error()
		return page
	}
	page.Articles = articles
	return page
}

func fetchFeedArticles(ctx context.Context, client *http.Client, feedURL string) result.Result[[]Article] {
	return result.From(http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)).
		MapErr(func(err error) error {
			return fmt.Errorf("リクエストを作成できません: %v", err)
		}).
		AndTee(func(req *http.Request) {
			req.Header.Set("User-Agent", userAgent)
			req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml;q=0.9, */*;q=0.8")
		}).
		AndThen(func(req *http.Request) result.Result[*http.Response] {
			resp, err := client.Do(req)
			if err != nil {
				return result.Err[*http.Response](fmt.Errorf("通信に失敗しました: %v", err))
			}
			return result.Ok(resp)
		}).
		AndThen(func(resp *http.Response) result.Result[[]byte] {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return result.Err[[]byte](fmt.Errorf("HTTP %d", resp.StatusCode))
			}
			body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
			if err != nil {
				return result.Err[[]byte](fmt.Errorf("本文を読めませんでした: %v", err))
			}
			if len(body) > maxBodyBytes {
				return result.Err[[]byte](fmt.Errorf("フィードが大きすぎます"))
			}
			return result.Ok(body)
		}).
		AndThen(func(body []byte) result.Result[[]Article] {
			articles, err := ParseFeedBody(body, feedURL)
			if err != nil {
				return result.Err[[]Article](fmt.Errorf("フィードを解釈できませんでした: %v", err))
			}
			return result.Ok(articles)
		})
}

// ParseFeedBody は RSS/Atom の生データを記事一覧に変換する。
func ParseFeedBody(body []byte, base string) ([]Article, error) {
	feed, err := gofeed.NewParser().Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return articlesFrom(feed, base), nil
}

func articlesFrom(feed *gofeed.Feed, base string) []Article {
	if feed == nil {
		return nil
	}
	articles := make([]Article, 0, len(feed.Items))
	seen := make(map[string]struct{})
	for _, item := range feed.Items {
		if item == nil {
			continue
		}
		link := safeURL(item.Link, base)
		if link != "" {
			if _, ok := seen[link]; ok {
				continue
			}
			seen[link] = struct{}{}
		}
		title := collapseSpace(stripTags(item.Title))
		if title == "" && link == "" {
			continue
		}
		if title == "" {
			title = "(無題)"
		}
		summarySrc := item.Description
		if summarySrc == "" {
			summarySrc = item.Content
		}
		articles = append(articles, Article{
			Title:     title,
			Link:      link,
			Published: itemTime(item),
			Summary:   summarize(summarySrc, maxSummaryLen),
		})
	}
	sort.SliceStable(articles, func(i, j int) bool {
		ti, tj := articles[i].Published, articles[j].Published
		if ti.IsZero() != tj.IsZero() {
			return !ti.IsZero()
		}
		return ti.After(tj)
	})
	if len(articles) > maxArticles {
		articles = articles[:maxArticles]
	}
	return articles
}

func itemTime(item *gofeed.Item) time.Time {
	switch {
	case item.PublishedParsed != nil:
		return *item.PublishedParsed
	case item.UpdatedParsed != nil:
		return *item.UpdatedParsed
	default:
		return time.Time{}
	}
}
