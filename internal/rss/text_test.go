package rss

import "testing"

func TestStripTags(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{in: "no tags", want: "no tags"},
		{in: "<p>日本語</p>", want: " 日本語 "},
		{in: "a &amp; b", want: "a & b"},
		{in: "<script>alert(1)</script>ok", want: " ok"},
		{in: "<SCRIPT>alert(1)</SCRIPT>ok", want: " ok"},
		{in: "İ<style>body{}</style>本文", want: "İ 本文"},
		{in: "a <!-- secret --> b", want: "a   b"},
		{in: "hello <br", want: "hello "},
	}
	for _, tt := range tests {
		if got := stripTags(tt.in); got != tt.want {
			t.Errorf("stripTags(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSummarize(t *testing.T) {
	got := summarize("<p>hello <b>world</b></p>", 20)
	if got != "hello world" {
		t.Fatalf("summarize = %q", got)
	}
	long := summarize("alpha beta gamma delta", 10)
	if long == "alpha beta gamma delta" || len([]rune(long)) > 12 {
		t.Fatalf("summarize long = %q", long)
	}
}

func TestSafeURL(t *testing.T) {
	if got := safeURL("javascript:alert(1)", "https://example.com"); got != "" {
		t.Fatalf("javascript url leaked: %s", got)
	}
	if got := safeURL("/posts/1", "https://example.com/feed.xml"); got != "https://example.com/posts/1" {
		t.Fatalf("relative url = %s", got)
	}
	if got := safeURL("https://example.com/a", ""); got != "https://example.com/a" {
		t.Fatalf("absolute url = %s", got)
	}
	if got := safeURL("", "https://example.com"); got != "" {
		t.Fatalf("empty url = %s", got)
	}
}
