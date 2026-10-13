package rss

import (
	"strings"
	"testing"
	"time"

	"github.com/ss49919201/rss-reader/internal/site"
)

func TestFormatSlackMessage(t *testing.T) {
	text, err := formatSlackMessage(
		site.Site{Name: "Demo"},
		Article{
			Title:     "Hello",
			Link:      "https://example.com/a",
			Published: time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC),
			Summary:   "概要です",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "*Demo*") {
		t.Fatalf("site name: %q", text)
	}
	if !strings.Contains(text, "<https://example.com/a|Hello>") {
		t.Fatalf("link: %q", text)
	}
	if !strings.Contains(text, "概要です") {
		t.Fatalf("summary: %q", text)
	}
}
