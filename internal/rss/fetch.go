package rss

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
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
	Site      Site
	Articles  []Article
	FetchedAt time.Time
	Error     string
}

var defaultClient = &http.Client{Timeout: fetchTimeout}

// FetchAll はサイトを並行に取得する。結果の並びは sites と同じ。
func FetchAll(ctx context.Context, sites []Site) []SitePage {
	return fetchAll(ctx, defaultClient, sites)
}

func fetchAll(ctx context.Context, client *http.Client, sites []Site) []SitePage {
	pages := make([]SitePage, len(sites))
	var wg sync.WaitGroup
	for i, site := range sites {
		wg.Add(1)
		go func(i int, site Site) {
			defer wg.Done()
			pages[i] = Fetch(ctx, client, site)
		}(i, site)
	}
	wg.Wait()
	return pages
}

// Fetch は1サイトのRSSまたはAtomを取得して記事にする。
func Fetch(ctx context.Context, client *http.Client, site Site) SitePage {
	page := SitePage{Site: site, FetchedAt: time.Now()}
	if client == nil {
		client = defaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, site.URL, nil)
	if err != nil {
		page.Error = fmt.Sprintf("リクエストを作成できません: %v", err)
		return page
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml;q=0.9, */*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		page.Error = fmt.Sprintf("通信に失敗しました: %v", err)
		return page
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		page.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return page
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		page.Error = fmt.Sprintf("本文を読めませんでした: %v", err)
		return page
	}
	if len(body) > maxBodyBytes {
		page.Error = "フィードが大きすぎます"
		return page
	}

	feed, err := gofeed.NewParser().Parse(bytes.NewReader(body))
	if err != nil {
		page.Error = fmt.Sprintf("フィードを解釈できませんでした: %v", err)
		return page
	}
	page.Articles = articlesFrom(feed, site.URL)
	return page
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
		if strings.TrimSpace(summarySrc) == "" {
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
