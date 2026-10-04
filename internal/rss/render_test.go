package rss

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteHTML(t *testing.T) {
	dir := t.TempDir()
	when := time.Date(2026, 10, 3, 0, 30, 0, 0, time.UTC)
	pages := []SitePage{
		{
			Site:      Site{ID: "go-blog", Name: "Go Blog", URL: "https://go.dev/blog/feed.atom"},
			FetchedAt: when,
			Articles: []Article{
				{Title: `<script>alert(1)</script>`, Link: "javascript:alert(1)", Summary: "本文"},
				{Title: "通常の記事", Link: "https://example.com/ok", Published: when, Summary: "概要"},
			},
		},
		{
			Site:      Site{ID: "zenn", Name: "Zenn", URL: "https://zenn.dev/feed"},
			FetchedAt: when,
			Error:     "HTTP 500",
		},
	}
	for i := 0; i < 6; i++ {
		pages[0].Articles = append(pages[0].Articles, Article{
			Title: "extra",
			Link:  "https://example.com/e",
		})
	}

	if err := WriteHTML(dir, when, pages); err != nil {
		t.Fatal(err)
	}

	index := readFile(t, filepath.Join(dir, "index.html"))
	if !strings.Contains(index, "RSSリーダー") || !strings.Contains(index, "2026-10-03 09:30 JST") {
		t.Fatalf("index header missing: %s", index[:min(400, len(index))])
	}
	if !strings.Contains(index, "go-blog.html") || !strings.Contains(index, "全8件を見る") {
		t.Fatal("index should link to the full site page")
	}
	if !strings.Contains(index, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatal("title was not escaped")
	}
	if strings.Contains(index, "javascript:alert") {
		t.Fatal("javascript link was rendered")
	}
	if !strings.Contains(index, "取得できませんでした: HTTP 500") {
		t.Fatal("site error was not rendered")
	}
	if strings.Contains(index, ".rss-tmp-") {
		t.Fatal("temp file name leaked")
	}

	site := readFile(t, filepath.Join(dir, "go-blog.html"))
	if !strings.Contains(site, "通常の記事") || !strings.Contains(site, "https://example.com/ok") {
		t.Fatal("site page missing article")
	}
	if !strings.Contains(site, `href="index.html"`) {
		t.Fatal("site page missing back link")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("files = %d", len(entries))
	}
}

func TestWriteHTMLRejectsBadID(t *testing.T) {
	dir := t.TempDir()
	err := WriteHTML(dir, time.Now(), []SitePage{{
		Site: Site{ID: "../escape", Name: "x", URL: "https://example.com"},
	}})
	if err == nil {
		t.Fatal("expected error")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("wrote files despite bad id: %d", len(entries))
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
