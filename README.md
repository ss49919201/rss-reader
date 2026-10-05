# RSSリーダー

登録済みのRSS/Atomを取得し、結果をHTMLファイルとして保存するGoのバッチです。サイト一覧はコードに直書きしています。

## 実行

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

## サイトの追加

`internal/rss/sites.go` の `Sites` に足します。`ID` は英小文字・数字・ハイフンだけで、HTMLのファイル名になります。

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
