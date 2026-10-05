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
	"time"

	"github.com/ss49919201/rss-reader/internal/rss"
	"github.com/ss49919201/rss-reader/internal/sitedb"
)

func main() {
	log.SetFlags(log.LstdFlags)
	out := flag.String("out", "out", "HTMLの保存先ディレクトリ")
	interval := flag.Duration("interval", 0, "定期実行の間隔（例: 30m, 1h）。0 なら1回実行して終了する")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "使い方: rss-reader [-interval 1h] [-out dir]\n")
		fmt.Fprintf(flag.CommandLine.Output(), "       rss-reader sites list | add <id> <name> <url> | remove <id>\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "登録済みサイトのRSS/Atomを取得し、結果をHTMLファイルとして保存します。\n")
		fmt.Fprintf(flag.CommandLine.Output(), "サイト一覧は Cloudflare R2 上の SQLite ファイルにあります。R2_* 環境変数で接続先を指定します（README参照）。\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	cfg, err := sitedb.ConfigFromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	store := sitedb.New(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if flag.Arg(0) == "sites" {
		if err := runSites(ctx, store, flag.Args()[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if flag.NArg() > 0 {
		flag.Usage()
		os.Exit(2)
	}

	if *interval < 0 || (*interval > 0 && *interval < time.Second) {
		fmt.Fprintln(os.Stderr, "interval は 0、または1秒以上を指定してください")
		os.Exit(2)
	}

	if *interval > 0 {
		log.Printf("定期実行を開始します: 間隔 %s / 保存先 %s", *interval, *out)
	}

	err = rss.Repeat(ctx, *interval, func(ctx context.Context) error {
		return runOnce(ctx, store, *out)
	})
	if err != nil {
		os.Exit(1)
	}
}

func runOnce(ctx context.Context, store *sitedb.Store, out string) error {
	sites, err := store.Load(ctx)
	if err != nil {
		log.Printf("サイト一覧を読めませんでした: %v", err)
		return err
	}
	pages, err := rss.RunOnce(ctx, sites, out)
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

func runSites(ctx context.Context, store *sitedb.Store, args []string) error {
	usage := errors.New("使い方: rss-reader sites list | add <id> <name> <url> | remove <id>")
	if len(args) == 0 {
		return usage
	}
	switch {
	case args[0] == "list" && len(args) == 1:
		sites, err := store.Load(ctx)
		if err != nil {
			return err
		}
		for _, site := range sites {
			fmt.Printf("%s\t%s\t%s\n", site.ID, site.Name, site.URL)
		}
		return nil
	case args[0] == "add" && len(args) == 4:
		if err := store.Add(ctx, rss.Site{ID: args[1], Name: args[2], URL: args[3]}); err != nil {
			return err
		}
		fmt.Printf("追加しました: %s (%s)\n", args[1], store.Location())
		return nil
	case args[0] == "remove" && len(args) == 2:
		if err := store.Remove(ctx, args[1]); err != nil {
			return err
		}
		fmt.Printf("削除しました: %s (%s)\n", args[1], store.Location())
		return nil
	default:
		return usage
	}
}
