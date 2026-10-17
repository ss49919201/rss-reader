package rss

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"text/template"
	"time"

	"github.com/ss49919201/andthen/result"
	"github.com/ss49919201/rss-reader/internal/site"
)

// Notifier は新着記事を外部へ送る。
type Notifier interface {
	NotifyArticle(ctx context.Context, site site.Site, article Article) error
}

// WebhookNotifier は Slack Incoming Webhook へ通知する。
type WebhookNotifier struct {
	URL    string
	Client *http.Client
}

type slackPayload struct {
	Text string `json:"text"`
}

type slackMessageView struct {
	SiteName   string
	Title      string
	Link       string
	HasLink    bool
	When       string
	Summary    string
	HasSummary bool
}

var slackMessageTmpl = template.Must(template.New("slack").Parse(`*{{.SiteName}}*
{{if .HasLink}}<{{.Link}}|{{.Title}}>{{else}}{{.Title}}{{end}}{{if .When}} ({{.When}}){{end}}{{if .HasSummary}}
{{.Summary}}{{end}}`))

// NotifyArticle は1件の記事を Slack に送る。
func (n *WebhookNotifier) NotifyArticle(ctx context.Context, st site.Site, article Article) error {
	if n == nil || n.URL == "" {
		return fmt.Errorf("Slack Webhook URL が未設定です")
	}
	client := n.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	_, err := result.From(formatSlackMessage(st, article)).
		AndThen(func(text string) result.Result[[]byte] {
			return result.From(json.Marshal(slackPayload{Text: text}))
		}).
		AndThen(func(body []byte) result.Result[*http.Response] {
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.URL, bytes.NewReader(body))
			if err != nil {
				return result.Err[*http.Response](err)
			}
			req.Header.Set("Content-Type", "application/json")
			return result.From(client.Do(req))
		}).
		AndThrough(func(resp *http.Response) result.Result[struct{}] {
			defer resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
				return result.Err[struct{}](fmt.Errorf("Slack が HTTP %d を返しました: %s", resp.StatusCode, string(msg)))
			}
			return result.Ok(struct{}{})
		}).
		Get()
	return err
}

func formatSlackMessage(st site.Site, article Article) (string, error) {
	link := safeURL(article.Link, "")
	title := article.Title
	if title == "" {
		title = "(無題)"
	}
	when := ""
	if !article.Published.IsZero() {
		when = formatTime(article.Published)
	}
	view := slackMessageView{
		SiteName:   st.Name,
		Title:      title,
		Link:       link,
		HasLink:    link != "",
		When:       when,
		Summary:    article.Summary,
		HasSummary: article.Summary != "",
	}
	var buf bytes.Buffer
	if err := slackMessageTmpl.Execute(&buf, view); err != nil {
		return "", err
	}
	return buf.String(), nil
}
