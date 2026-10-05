package main

//go:generate sh ../../scripts/deadcode.sh

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ss49919201/rss-reader/internal/rss"
)

func main() {
	log.SetFlags(log.LstdFlags)
	out := flag.String("out", "out", "HTMLの保存先ディレクトリ")
	interval := flag.Duration("interval", 0, "定期実行の間隔（例: 30m, 1h）。0 なら1回実行して終了する")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "使い方: rss-reader [-interval 1h] [-out dir]\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "登録済みサイトのRSS/Atomを取得し、結果をHTMLファイルとして保存します。\n")
		fmt.Fprintf(flag.CommandLine.Output(), "サイトの追加は internal/rss/sites.go の Sites を編集します。\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *interval < 0 || (*interval > 0 && *interval < time.Second) {
		fmt.Fprintln(os.Stderr, "interval は 0、または1秒以上を指定してください")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *interval > 0 {
		log.Printf("定期実行を開始します: 間隔 %s / 保存先 %s", *interval, *out)
	}

	err := rss.Repeat(ctx, *interval, func(ctx context.Context) error {
		return runOnce(ctx, *out)
	})
	if err != nil {
		os.Exit(1)
	}
}

func runOnce(ctx context.Context, out string) error {
	pages, err := rss.RunOnce(ctx, rss.Sites, out)
	for _, page := range pages {
		if page.Error != "" {
			log.Printf("失敗 %s: %s", page.Site.Name, page.Error)
			continue
		}
		log.Printf("取得 %s: %d件", page.Site.Name, len(page.Articles))
	}
	switch {
	case err == nil:
		log.Printf("HTMLを保存しました: %s", out)
	case errors.As(err, new(*rss.SiteErrors)):
		log.Print(err)
		log.Printf("HTMLを保存しました: %s", out)
	default:
		log.Print(err)
	}
	return err
}
