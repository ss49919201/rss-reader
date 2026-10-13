# RSSリーダー

登録サイトと通知状態を Cloudflare R2 上の SQLite で管理し、新着記事を Slack に通知する Go のバッチです。

## 実行

```sh
export SLACK_WEBHOOK_URL='https://hooks.slack.com/services/...'
export R2_ACCOUNT_ID='...'
export R2_ACCESS_KEY_ID='...'
export R2_SECRET_ACCESS_KEY='...'
export R2_BUCKET='...'
# 任意: export R2_OBJECT_KEY='rss-reader.db'

go run ./cmd/rss-reader run
```

## フィード管理

R2 上の DB を操作します（いずれもダウンロード → 変更 → アップロード）。

```sh
go run ./cmd/rss-reader feeds list
go run ./cmd/rss-reader feeds add -id zenn -name Zenn -url https://zenn.dev/feed
go run ./cmd/rss-reader feeds remove -id zenn
go run ./cmd/rss-reader feeds seed -file feeds/feeds.json
```

`feeds seed` はリポジトリ同梱の `feeds/feeds.json` を初期データとして投入します（重複 ID はエラー）。

## GitHub Actions

`.github/workflows/rss-reader.yml` が1時間ごとに `run` を実行します。`workflow_dispatch` では `command` 入力で `feeds list` や `feeds seed` なども実行できます。`concurrency` で直列化しています。

## スキーマ

`internal/store/schema.sql` を参照してください。

## テスト

```sh
go test ./...
```

## 未使用関数

`golang.org/x/tools/cmd/deadcode` で、プログラムのエントリポイント（`main`）から到達できない関数を調べます。

```sh
set -o pipefail; out=$(mktemp); trap 'rm -f "$out"' EXIT; GOTOOLCHAIN=go$(awk '/^go /{print $2; exit}' go.mod) go run golang.org/x/tools/cmd/deadcode@v0.51.0 ./... | tee "$out" && [ ! -s "$out" ]
```
