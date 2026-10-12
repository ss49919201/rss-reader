package rss

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWebhookNotifier(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		body = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	site := Site{ID: "demo", Name: "Demo", URL: "https://example.com/feed"}
	article := Article{Title: "Hello", Link: "https://example.com/a", Published: time.Now()}
	n := &WebhookNotifier{URL: srv.URL, Client: srv.Client()}
	if err := n.NotifyArticle(context.Background(), site, article); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "Hello") || !strings.Contains(body, "Demo") {
		t.Fatalf("body = %q", body)
	}
}
