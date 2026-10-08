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

## Cursor クラウドエージェント（pstack）

クラウドエージェントは、このリポジトリの `.cursor/skills/poteto-mode/SKILL.md` を custom mode `poteto-mode` として起動する。中身は [cursor/plugins](https://github.com/cursor/plugins) の `pstack/`（commit は `.cursor/skills/PSTACK_VERSION`、ライセンスは `.cursor/skills/LICENSE`）を編集せずに置いている。

- スキルは `.cursor/skills/<name>/`
- subagent（`poteto-agent`、Comment Sicko）は `.cursor/agents/`
- スキルから `../../docs` と `../../README.md` で参照されるガイドと pstack の README、およびそこから辿る `automations/` は、同じ相対位置になるよう `.cursor/docs`、`.cursor/README.md`、`.cursor/automations` に置いている
- `.cursor/hooks.json` は pstack のものではなく、deadcode の stop hook のまま

更新するときは、リポジトリのルートで次を実行する。`.cursor/hooks.json` は消さない。

```sh
d=$(mktemp -d) && git clone --filter=blob:none --sparse --depth 1 https://github.com/cursor/plugins.git "$d" && git -C "$d" sparse-checkout set pstack && SHA=$(git -C "$d" rev-parse HEAD) && VER=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["version"])' "$d/pstack/.cursor-plugin/plugin.json") && rm -rf .cursor/skills .cursor/agents .cursor/docs .cursor/automations .cursor/README.md && cp -a "$d/pstack/skills" .cursor/skills && cp -a "$d/pstack/agents" .cursor/agents && cp -a "$d/pstack/docs" .cursor/docs && cp -a "$d/pstack/automations" .cursor/automations && cp -a "$d/pstack/README.md" .cursor/README.md && cp -a "$d/pstack/LICENSE" .cursor/skills/LICENSE && printf 'commit: %s\nlicense: MIT\nplugin_version: %s\nsource: https://github.com/cursor/plugins\npath: pstack\nurl: https://github.com/cursor/plugins/tree/%s/pstack\n' "$SHA" "$VER" "$SHA" > .cursor/skills/PSTACK_VERSION && rm -rf "$d"
```
