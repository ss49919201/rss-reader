# RSSリーダー

埋め込んだ `internal/rss/feeds.json` の RSS/Atom を取得し、新着記事を Slack に通知する Go のバッチです。

## 実行

```sh
export SLACK_WEBHOOK_URL='https://hooks.slack.com/services/...'
go run ./cmd/rss-reader -state .rss-state.json
```

ビルドする場合:

```sh
go build -o rss-reader ./cmd/rss-reader
```

初回実行では通知せず、実行時刻だけ状態ファイルに記録します。2回目以降は、前回実行時刻より新しい公開日時の記事（公開日時が無い記事は未通知のリンクのみ）を Slack に送ります。一部のフィードが失敗しても残りは試し、失敗はまとめてログと終了コードで返します。

## フィードの追加

`internal/rss/feeds.json` を編集してから再ビルドします。`id` は英小文字・数字・ハイフンだけにしてください。

## GitHub Actions

`.github/workflows/rss-reader.yml` が1時間ごとに実行します。リポジトリの secret `SLACK_WEBHOOK_URL` を設定してください。前回実行時刻は Actions cache 上の `.rss-state.json` に保存されます。

## テスト

```sh
go test ./...
```

## 未使用関数

`golang.org/x/tools/cmd/deadcode` で、プログラムのエントリポイント（`main`）から到達できない関数を調べます。報告があると失敗します。

```sh
set -o pipefail; out=$(mktemp); trap 'rm -f "$out"' EXIT; GOTOOLCHAIN=go$(awk '/^go /{print $2; exit}' go.mod) go run golang.org/x/tools/cmd/deadcode@v0.51.0 ./... | tee "$out" && [ ! -s "$out" ]
```

同じコマンドを CI（`.github/workflows/deadcode.yml`）と Cursor の stop hook（`.cursor/hooks.json`）で実行します。
