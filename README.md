# RSSリーダー

登録済みのRSS/Atomを取得し、結果をHTMLファイルとして保存するGoのバッチです。サイト一覧は Cloudflare R2 に置いた SQLite ファイルに保存します。

## R2 の準備

サイト一覧は R2 バケットのオブジェクト `sites.db`（SQLite）が正本です。実行のたびに R2 から一時ファイルへダウンロードして読み、サイトを追加・削除したときだけアップロードし直します。ローカルには一時ファイル以外を残しません。

1. バケットを作ります。名前は既存のものと重ならないものを選んでください。

   ```sh
   npx wrangler r2 bucket create <bucket>
   ```

2. Cloudflare ダッシュボードの R2 →「API トークンを管理」で、そのバケットに「オブジェクト読み取りと書き込み」権限のトークンを作り、Access Key ID と Secret Access Key を控えます。

3. 環境変数を設定します。

   | 変数 | 必須 | 内容 |
   | --- | --- | --- |
   | `R2_ACCOUNT_ID` | ○ | Cloudflare のアカウントID。エンドポイント `https://<R2_ACCOUNT_ID>.r2.cloudflarestorage.com` に使います |
   | `R2_ACCESS_KEY_ID` | ○ | R2 API トークンの Access Key ID |
   | `R2_SECRET_ACCESS_KEY` | ○ | R2 API トークンの Secret Access Key |
   | `R2_BUCKET` | ○ | バケット名 |
   | `R2_OBJECT_KEY` | | オブジェクトキー。既定は `sites.db` |
   | `R2_ENDPOINT` | | エンドポイントを上書きします（S3互換のローカルサーバーで試すとき用）。指定すると `R2_ACCOUNT_ID` は不要です |

初回、`sites.db` がまだなければ、Go Blog・Zenn・GitHub Blog・はてなブックマーク人気エントリーの4件を登録したデータベースを作ってアップロードします。

書き込みは ETag を使った条件付き PUT です。別のプロセスが同時に書き換えていたら、取得からやり直すので更新は失われません。

## 実行

1回だけ取得して終了します。

```sh
go run ./cmd/rss-reader -out out
```

プロセス内で定期実行します。

```sh
go run ./cmd/rss-reader -interval 1h -out out
```

cronで1時間ごとに実行する例です。cron の環境にも `R2_*` 変数を渡してください。

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

## サイトの追加

R2 上の `sites.db` を `sites` サブコマンドで編集します。`ID` は英小文字・数字・ハイフンだけで、HTMLのファイル名になります。

```sh
go run ./cmd/rss-reader sites list
go run ./cmd/rss-reader sites add example "Example" https://example.com/feed.xml
go run ./cmd/rss-reader sites remove example
```

`-interval` で定期実行している間も、毎回 R2 から読み直すので、追加・削除は次の取得から反映されます。

テーブルは `sites (id TEXT PRIMARY KEY, name TEXT NOT NULL, url TEXT NOT NULL)` で、登録順に取得します。

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
