package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ss49919201/rss-reader/internal/rss"
)

func main() {
	log.SetFlags(log.LstdFlags)
	statePath := flag.String("state", ".rss-state.json", "前回実行時刻などを保存するJSONファイル")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "使い方: rss-reader [-state path]\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "埋め込み feeds.json の RSS/Atom を取得し、新着を Slack に通知します。\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Webhook URL は環境変数 SLACK_WEBHOOK_URL に設定します。\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	webhook := os.Getenv("SLACK_WEBHOOK_URL")
	if webhook == "" {
		fmt.Fprintln(os.Stderr, "環境変数 SLACK_WEBHOOK_URL を設定してください")
		os.Exit(2)
	}

	sites, err := rss.LoadEmbeddedSites()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	notifier := &rss.WebhookNotifier{URL: webhook}
	err = rss.Run(ctx, sites, notifier, *statePath)
	switch {
	case err == nil:
		log.Print("完了しました")
	case errors.As(err, new(*rss.SiteErrors)):
		log.Print(err)
		os.Exit(1)
	default:
		log.Print(err)
		os.Exit(1)
	}
}
