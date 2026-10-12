package rss

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Notifier は新着記事を外部へ送る。
type Notifier interface {
	NotifyArticle(ctx context.Context, site Site, article Article) error
}

// WebhookNotifier は Slack Incoming Webhook へ通知する。
type WebhookNotifier struct {
	URL    string
	Client *http.Client
}

type slackPayload struct {
	Text string `json:"text"`
}

// NotifyArticle は1件の記事を Slack に送る。
func (n *WebhookNotifier) NotifyArticle(ctx context.Context, site Site, article Article) error {
	if n == nil || strings.TrimSpace(n.URL) == "" {
		return fmt.Errorf("Slack Webhook URL が未設定です")
	}
	client := n.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	text := formatSlackMessage(site, article)
	body, err := json.Marshal(slackPayload{Text: text})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("Slack が HTTP %d を返しました: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return nil
}

func formatSlackMessage(site Site, article Article) string {
	when := ""
	if !article.Published.IsZero() {
		when = " (" + formatTime(article.Published) + ")"
	}
	link := safeURL(article.Link, "")
	title := article.Title
	if title == "" {
		title = "(無題)"
	}
	var b strings.Builder
	b.WriteString("*")
	b.WriteString(site.Name)
	b.WriteString("*\n")
	if link != "" {
		b.WriteString("<")
		b.WriteString(link)
		b.WriteString("|")
		b.WriteString(title)
		b.WriteString(">")
	} else {
		b.WriteString(title)
	}
	b.WriteString(when)
	if s := strings.TrimSpace(article.Summary); s != "" {
		b.WriteString("\n")
		b.WriteString(s)
	}
	return b.String()
}
