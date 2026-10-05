# RSSリーダー

登録済みのRSS/Atomを取得し、結果をHTMLファイルとして保存するGoのバッチです。サイト一覧は Cloudflare R2 上の SQLite ファイルが正本です。

## 実行

環境変数（下の「サイトの保存先」）を入れてから実行します。

1回だけ取得して終了します。

```sh
go run ./cmd/rss-reader -out out
```

プロセス内で定期実行します。

```sh
go run ./cmd/rss-reader -interval 1h -out out
```

cronで1時間ごとに実行する例です。

```cron
0 * * * * cd /path/to/rss-reader && /path/to/rss-reader -out /var/www/rss
```

ビルドする場合:

```sh
go build -o rss-reader ./cmd/rss-reader
```

`-interval` を付けた実行は、SIGINT / SIGTERM で取得を中断して終了します。一部のサイトが失敗しても、成功した分と失敗内容はHTMLに残します。1回実行のときは、その場合の終了コードは1です。

## 出力

- `out/index.html` … サイト一覧と各サイトの新しい記事
- `out/<id>.html` … サイトごとの記事

## サイトの保存先（Cloudflare R2）

登録サイトは SQLite の1ファイルです。置き場所は R2 バケット `rss-reader-sites` のオブジェクト `sites.db` です。起動のたびにこのオブジェクトを読み、登録を変えたときだけ同じキーへ書き戻します。ディスク上のファイルは処理中の一時コピーだけで、消します。正本は R2 です。

アプリが読む設定は次のとおりです。

- `R2_ACCOUNT_ID` … Cloudflare アカウント ID（32桁の16進数）。必須
- `R2_ACCESS_KEY_ID` … R2 API トークンの Access Key ID。必須
- `R2_SECRET_ACCESS_KEY` … R2 API トークンの Secret Access Key。必須
- `R2_BUCKET` … バケット名。省略時は `rss-reader-sites`
- `R2_OBJECT_KEY` … オブジェクトキー。省略時は `sites.db`
- `R2_ENDPOINT` … 任意。省略時は `https://<R2_ACCOUNT_ID>.r2.cloudflarestorage.com`

S3 互換 API をパス形式で使います。オブジェクト URL は `https://<ACCOUNT_ID>.r2.cloudflarestorage.com/rss-reader-sites/sites.db` です。

初回の準備:

1. R2 にバケット `rss-reader-sites` を作ります。ダッシュボードか `npx wrangler r2 bucket create rss-reader-sites` です。
2. そのバケットに対する Object Read と Object Write の API トークンを作り、Access Key ID と Secret Access Key を控えます。
3. アカウント ID とキーを環境変数に入れます。例は `.env.example` です。

バケットに `sites.db` がまだ無いとき、最初の取得か `-list` で次の4サイトを入れたデータベースを作ってアップロードします。

- `go-blog` … https://go.dev/blog/feed.atom
- `zenn` … https://zenn.dev/feed
- `github-blog` … https://github.blog/feed/
- `hatena-hotentry` … https://b.hatena.ne.jp/hotentry.rss

## サイトの追加

`ID` は英小文字・数字・ハイフンだけで、HTMLのファイル名になります。

```sh
go run ./cmd/rss-reader -add -id example -name Example -url https://example.com/feed.xml
go run ./cmd/rss-reader -list
go run ./cmd/rss-reader -remove example
```

## テスト

```sh
go test ./...
```

## 未使用関数

`golang.org/x/tools/cmd/deadcode` で、プログラムのエントリポイント（`main`）から到達できない関数を調べます。報告があると失敗します。

Goには、Knipのように未使用ファイルまでまとめて検出する単一のツールはありません。この検査は deadcode です。

```sh
set -o pipefail; out=$(mktemp); trap 'rm -f "$out"' EXIT; GOTOOLCHAIN=go$(awk '/^go /{print $2; exit}' go.mod) go run golang.org/x/tools/cmd/deadcode@v0.51.0 ./... | tee "$out" && [ ! -s "$out" ]
```

同じコマンドを CI（`.github/workflows/deadcode.yml`）と Cursor の stop hook（`.cursor/hooks.json`）で実行します。
