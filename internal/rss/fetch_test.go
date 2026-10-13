package rss

import (
	"strings"
	"testing"
	"time"
)

const sampleRSS = `<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <title>Example</title>
    <link>https://example.com</link>
    <item>
      <title>新しい記事</title>
      <link>https://example.com/new</link>
      <pubDate>Sat, 03 Oct 2026 00:00:00 GMT</pubDate>
      <description>&lt;p&gt;本文です&lt;/p&gt;</description>
    </item>
    <item>
      <title>Hello &amp; World &lt;script&gt;alert(1)&lt;/script&gt;</title>
      <link>https://example.com/old</link>
      <pubDate>Fri, 02 Oct 2026 00:00:00 GMT</pubDate>
      <description>older</description>
    </item>
    <item>
      <title>相対リンク</title>
      <link>/relative</link>
      <pubDate>Thu, 01 Oct 2026 00:00:00 GMT</pubDate>
    </item>
    <item>
      <title>危険なリンク</title>
      <link>javascript:alert(1)</link>
      <description>skip me not the title</description>
    </item>
    <item>
      <title>新しい記事</title>
      <link>https://example.com/new</link>
      <description>duplicate</description>
    </item>
  </channel>
</rss>`

const sampleAtom = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Example</title>
  <updated>2026-10-03T00:00:00Z</updated>
  <entry>
    <title>Atomの記事</title>
    <link href="https://example.com/atom"/>
    <updated>2026-10-03T00:00:00Z</updated>
    <summary>概要</summary>
  </entry>
</feed>`

func TestParseFeedBodyRSS(t *testing.T) {
	articles, err := ParseFeedBody([]byte(sampleRSS), "https://example.com/feed")
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 4 {
		t.Fatalf("articles = %d", len(articles))
	}
	if articles[0].Title != "新しい記事" || articles[0].Link != "https://example.com/new" {
		t.Fatalf("newest = %#v", articles[0])
	}
	if articles[0].Summary != "本文です" {
		t.Fatalf("summary = %q", articles[0].Summary)
	}
	if articles[0].Published.IsZero() {
		t.Fatal("missing published time")
	}
	if !strings.Contains(articles[1].Title, "Hello & World") || strings.Contains(articles[1].Title, "<script>") {
		t.Fatalf("title = %q", articles[1].Title)
	}
	if articles[2].Link != "https://example.com/relative" {
		t.Fatalf("relative link = %s", articles[2].Link)
	}
	if articles[3].Link != "" || articles[3].Title != "危険なリンク" {
		t.Fatalf("unsafe link kept: %#v", articles[3])
	}
}

func TestParseFeedBodyAtom(t *testing.T) {
	articles, err := ParseFeedBody([]byte(sampleAtom), "https://example.com/feed")
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 || articles[0].Title != "Atomの記事" || articles[0].Summary != "概要" {
		t.Fatalf("atom = %#v", articles)
	}
}

func TestParseFeedBodyInvalid(t *testing.T) {
	_, err := ParseFeedBody([]byte("<html>nope</html>"), "https://example.com")
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestIsNewArticle(t *testing.T) {
	since := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	has := map[string]bool{"https://example.com/seen": true}
	hasLink := func(link string) bool { return has[link] }

	newer := Article{Link: "https://example.com/n", Published: since.Add(time.Hour)}
	older := Article{Link: "https://example.com/o", Published: since.Add(-time.Hour)}
	if !isNewArticle(since, hasLink, newer) || isNewArticle(since, hasLink, older) {
		t.Fatal("published filter failed")
	}
	if isNewArticle(time.Time{}, hasLink, newer) {
		t.Fatal("first run should not notify")
	}
	unpublished := Article{Link: "https://example.com/u"}
	if !isNewArticle(since, hasLink, unpublished) {
		t.Fatal("unpublished link should notify once")
	}
	if !isNewArticle(since, hasLink, Article{Link: "https://example.com/seen"}) {
		t.Fatal("seen link should not notify")
	}
}
