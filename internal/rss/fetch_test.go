package rss

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func TestFetchRSSAndAtom(t *testing.T) {
	var ua string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		switch r.URL.Path {
		case "/rss":
			w.Header().Set("Content-Type", "application/rss+xml")
			_, _ = w.Write([]byte(sampleRSS))
		case "/atom":
			w.Header().Set("Content-Type", "application/atom+xml")
			_, _ = w.Write([]byte(sampleAtom))
		case "/bad":
			http.Error(w, "nope", http.StatusInternalServerError)
		case "/html":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html>not a feed</html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := srv.Client()
	page := Fetch(context.Background(), client, Site{ID: "demo", Name: "Demo", URL: srv.URL + "/rss"})
	if page.Error != "" {
		t.Fatal(page.Error)
	}
	if ua != userAgent {
		t.Fatalf("user agent = %q", ua)
	}
	if len(page.Articles) != 4 {
		t.Fatalf("articles = %d", len(page.Articles))
	}
	if page.Articles[0].Title != "新しい記事" || page.Articles[0].Link != "https://example.com/new" {
		t.Fatalf("newest = %#v", page.Articles[0])
	}
	if page.Articles[0].Summary != "本文です" {
		t.Fatalf("summary = %q", page.Articles[0].Summary)
	}
	if page.Articles[0].Published.IsZero() {
		t.Fatal("missing published time")
	}
	if !strings.Contains(page.Articles[1].Title, "Hello & World") || strings.Contains(page.Articles[1].Title, "<script>") {
		t.Fatalf("title = %q", page.Articles[1].Title)
	}
	if page.Articles[2].Link != srv.URL+"/relative" {
		t.Fatalf("relative link = %s", page.Articles[2].Link)
	}
	if page.Articles[3].Link != "" || page.Articles[3].Title != "危険なリンク" {
		t.Fatalf("unsafe link kept: %#v", page.Articles[3])
	}

	atom := Fetch(context.Background(), client, Site{ID: "atom", Name: "Atom", URL: srv.URL + "/atom"})
	if atom.Error != "" {
		t.Fatal(atom.Error)
	}
	if len(atom.Articles) != 1 || atom.Articles[0].Title != "Atomの記事" || atom.Articles[0].Summary != "概要" {
		t.Fatalf("atom = %#v", atom.Articles)
	}

	bad := Fetch(context.Background(), client, Site{ID: "bad", Name: "Bad", URL: srv.URL + "/bad"})
	if !strings.Contains(bad.Error, "HTTP 500") {
		t.Fatalf("bad error = %q", bad.Error)
	}
	broken := Fetch(context.Background(), client, Site{ID: "html", Name: "HTML", URL: srv.URL + "/html"})
	if broken.Error == "" {
		t.Fatal("expected parse error")
	}
}

func TestFetchCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	page := Fetch(ctx, srv.Client(), Site{ID: "slow", Name: "Slow", URL: srv.URL})
	if page.Error == "" {
		t.Fatal("expected cancel error")
	}
}
