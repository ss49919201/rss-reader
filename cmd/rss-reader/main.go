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
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	log.SetFlags(log.LstdFlags)
	fs := flag.NewFlagSet("rss-reader", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	out := fs.String("out", "out", "HTMLの保存先ディレクトリ")
	interval := fs.Duration("interval", 0, "定期実行の間隔（例: 30m, 1h）。0 なら1回実行して終了する")
	list := fs.Bool("list", false, "登録サイトを表示して終了する")
	add := fs.Bool("add", false, "サイトを登録または更新して終了する")
	remove := fs.String("remove", "", "このIDのサイトを登録解除して終了する")
	id := fs.String("id", "", "登録するサイトID（英小文字・数字・ハイフン）")
	name := fs.String("name", "", "登録するサイト名")
	feedURL := fs.String("url", "", "登録するフィードのURL")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "使い方: rss-reader [-interval 1h] [-out dir]\n")
		fmt.Fprintf(fs.Output(), "        rss-reader -list\n")
		fmt.Fprintf(fs.Output(), "        rss-reader -add -id ID -name NAME -url FEED_URL\n")
		fmt.Fprintf(fs.Output(), "        rss-reader -remove ID\n\n")
		fmt.Fprintf(fs.Output(), "登録サイトは Cloudflare R2 の SQLite（バケット %s、オブジェクト %s）に保存します。\n", rss.DefaultBucket, rss.SitesObjectKey)
		fmt.Fprintf(fs.Output(), "環境変数: R2_ACCOUNT_ID, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY。任意で R2_BUCKET, R2_OBJECT_KEY, R2_ENDPOINT。\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "引数が多すぎます")
		fs.Usage()
		return 2
	}
	if *interval < 0 || (*interval > 0 && *interval < time.Second) {
		fmt.Fprintln(os.Stderr, "interval は 0、または1秒以上を指定してください")
		return 2
	}
	managing := *list || *add || *remove != ""
	switch {
	case *list && (*add || *remove != "" || *id != "" || *name != "" || *feedURL != ""):
		fmt.Fprintln(os.Stderr, "-list はほかの登録操作と同時に指定できません")
		return 2
	case *add && *remove != "":
		fmt.Fprintln(os.Stderr, "-add と -remove は同時に指定できません")
		return 2
	case *remove != "" && (*id != "" || *name != "" || *feedURL != ""):
		fmt.Fprintln(os.Stderr, "-remove は -id / -name / -url と同時に指定できません")
		return 2
	case *add && (*id == "" || *name == "" || *feedURL == ""):
		fmt.Fprintln(os.Stderr, "-add には -id, -name, -url が必要です")
		return 2
	case !*add && (*id != "" || *name != "" || *feedURL != ""):
		fmt.Fprintln(os.Stderr, "-id, -name, -url は -add と一緒に指定してください")
		return 2
	case managing && *interval != 0:
		fmt.Fprintln(os.Stderr, "-interval は取得のときだけ指定できます")
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := rss.ConfigFromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	store, err := rss.NewR2(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	switch {
	case *list:
		sites, err := rss.LoadSites(ctx, store, cfg.Bucket, cfg.ObjectKey)
		if err != nil {
			log.Print(err)
			return 1
		}
		for _, site := range sites {
			fmt.Printf("%s\t%s\t%s\n", site.ID, site.Name, site.URL)
		}
		return 0
	case *add:
		err = rss.AddSite(ctx, store, cfg.Bucket, cfg.ObjectKey, rss.Site{ID: *id, Name: *name, URL: *feedURL})
		if err != nil {
			log.Print(err)
			return 1
		}
		log.Printf("登録しました: %s", *id)
		return 0
	case *remove != "":
		err = rss.RemoveSite(ctx, store, cfg.Bucket, cfg.ObjectKey, *remove)
		if err != nil {
			log.Print(err)
			return 1
		}
		log.Printf("登録を解除しました: %s", *remove)
		return 0
	default:
		if *interval > 0 {
			log.Printf("定期実行を開始します: 間隔 %s / 保存先 %s", *interval, *out)
		}
		err = rss.Repeat(ctx, *interval, func(ctx context.Context) error {
			return runOnce(ctx, store, cfg, *out)
		})
		if err != nil {
			return 1
		}
		return 0
	}
}

func runOnce(ctx context.Context, store *rss.R2, cfg rss.R2Config, out string) error {
	sites, err := rss.LoadSites(ctx, store, cfg.Bucket, cfg.ObjectKey)
	if err != nil {
		log.Print(err)
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
