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

	"github.com/ss49919201/rss-reader/internal/feedseed"
	"github.com/ss49919201/rss-reader/internal/objectstore"
	"github.com/ss49919201/rss-reader/internal/remotedb"
	"github.com/ss49919201/rss-reader/internal/rss"
	"github.com/ss49919201/rss-reader/internal/site"
	"github.com/ss49919201/rss-reader/internal/store"
)

func main() {
	log.SetFlags(log.LstdFlags)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		os.Exit(cmdRun(os.Args[2:]))
	case "feeds":
		os.Exit(cmdFeeds(os.Args[2:]))
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "使い方:\n")
	fmt.Fprintf(os.Stderr, "  rss-reader run\n")
	fmt.Fprintf(os.Stderr, "  rss-reader feeds list\n")
	fmt.Fprintf(os.Stderr, "  rss-reader feeds add -id ID -name NAME -url URL\n")
	fmt.Fprintf(os.Stderr, "  rss-reader feeds remove -id ID\n")
	fmt.Fprintf(os.Stderr, "  rss-reader feeds seed [-file feeds/feeds.json]\n")
}

func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	webhook := os.Getenv("SLACK_WEBHOOK_URL")
	if webhook == "" {
		fmt.Fprintln(os.Stderr, "環境変数 SLACK_WEBHOOK_URL を設定してください")
		return 2
	}
	session, err := newSession()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	notifier := &rss.WebhookNotifier{URL: webhook}
	var runErr error
	err = session.WithStore(ctx, func(st store.Store) error {
		runErr = rss.Run(ctx, st, notifier)
		if runErr != nil {
			var siteErr *rss.SiteErrors
			if errors.As(runErr, &siteErr) {
				return nil
			}
			return runErr
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, objectstore.ErrPreconditionFailed) {
			fmt.Fprintln(os.Stderr, "R2 上の DB が別の実行で更新されました。concurrency 設定を確認してください")
			return 1
		}
		log.Print(err)
		return 1
	}
	switch {
	case runErr == nil:
		log.Print("完了しました")
	case errors.As(runErr, new(*rss.SiteErrors)):
		log.Print(runErr)
		return 1
	default:
		log.Print(runErr)
		return 1
	}
	return 0
}

func cmdFeeds(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "feeds のサブコマンドを指定してください (list, add, remove, seed)")
		return 2
	}
	switch args[0] {
	case "list":
		return cmdFeedsList(args[1:])
	case "add":
		return cmdFeedsAdd(args[1:])
	case "remove":
		return cmdFeedsRemove(args[1:])
	case "seed":
		return cmdFeedsSeed(args[1:])
	default:
		fmt.Fprintln(os.Stderr, "不明な feeds サブコマンドです")
		return 2
	}
}

func cmdFeedsList(args []string) int {
	fs := flag.NewFlagSet("feeds list", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	session, err := newSession()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	ctx := context.Background()
	err = session.WithStore(ctx, func(st store.Store) error {
		sites, err := st.ListSites(ctx)
		if err != nil {
			return err
		}
		for _, site := range sites {
			fmt.Printf("%s\t%s\t%s\n", site.ID, site.Name, site.URL)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func cmdFeedsAdd(args []string) int {
	fs := flag.NewFlagSet("feeds add", flag.ExitOnError)
	id := fs.String("id", "", "サイト ID")
	name := fs.String("name", "", "表示名")
	url := fs.String("url", "", "フィード URL")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *id == "" || *name == "" || *url == "" {
		fmt.Fprintln(os.Stderr, "-id, -name, -url は必須です")
		return 2
	}
	session, err := newSession()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	feed := site.Site{ID: *id, Name: *name, URL: *url}
	ctx := context.Background()
	err = session.WithStore(ctx, func(st store.Store) error {
		return st.AddSite(ctx, feed)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func cmdFeedsRemove(args []string) int {
	fs := flag.NewFlagSet("feeds remove", flag.ExitOnError)
	id := fs.String("id", "", "サイト ID")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *id == "" {
		fmt.Fprintln(os.Stderr, "-id は必須です")
		return 2
	}
	session, err := newSession()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	ctx := context.Background()
	err = session.WithStore(ctx, func(st store.Store) error {
		return st.RemoveSite(ctx, *id)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func cmdFeedsSeed(args []string) int {
	fs := flag.NewFlagSet("feeds seed", flag.ExitOnError)
	file := fs.String("file", "feeds/feeds.json", "初期フィード JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	sites, err := feedseed.LoadSitesFile(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	session, err := newSession()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	ctx := context.Background()
	err = session.WithStore(ctx, func(st store.Store) error {
		for _, site := range sites {
			if err := st.AddSite(ctx, site); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func newSession() (*remotedb.Session, error) {
	objects, err := objectstore.NewR2FromEnv()
	if err != nil {
		return nil, err
	}
	key := os.Getenv("R2_OBJECT_KEY")
	if key == "" {
		key = "rss-reader.db"
	}
	return &remotedb.Session{Objects: objects, Key: key}, nil
}
