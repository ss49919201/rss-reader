package rss

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const maxNotifiedLinks = 500

// RunState は前回実行以降に通知済みの記事を追跡する。
type RunState struct {
	LastRun       time.Time `json:"last_run"`
	NotifiedLinks []string  `json:"notified_links,omitempty"`
}

// LoadState は path から状態を読み込む。ファイルが無いときは空の状態を返す。
func LoadState(path string) (RunState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return RunState{}, nil
		}
		return RunState{}, err
	}
	var st RunState
	if err := json.Unmarshal(data, &st); err != nil {
		return RunState{}, fmt.Errorf("状態ファイルを解釈できません: %w", err)
	}
	return st, nil
}

// SaveState は状態を path に書き込む。
func SaveState(path string, st RunState) error {
	if path == "" {
		return fmt.Errorf("状態ファイルのパスが空です")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, data)
}

// writeAtomic は同一ディレクトリ内の一時ファイル経由で path を置き換える。
// 書き込み途中で落ちても、既存の path は壊れない。
//
//	 目的ファイル          一時ファイル（同じ dir）
//	 .rss-state.json       .rss-tmp-*
//	      │                      │
//	      │  (1) CreateTemp      │
//	      │◄─────────────────────┤ 新規作成
//	      │                      │
//	      │  (2) Write + Close   │
//	      │                      ├── 全文書き込み
//	      │                      │
//	      │  (3) Rename(tmp→path)│
//	      │◄─────────────────────┤ 同一 FS 上で名前だけ差し替え（原子的）
//	      │                      │
//	      ▼                      ▼
//	 更新後の JSON            （削除済み）
//
// (2) までに失敗した場合は defer で tmp だけ削除し、path は触らない。
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".rss-tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if !ok {
			os.Remove(tmp)
		}
	}()
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(0o644); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	ok = true
	return nil
}

// notifiedSet は NotifiedLinks を集合として扱う。
func (st RunState) notifiedSet() map[string]struct{} {
	m := make(map[string]struct{}, len(st.NotifiedLinks))
	for _, link := range st.NotifiedLinks {
		if link != "" {
			m[link] = struct{}{}
		}
	}
	return m
}

func (st *RunState) rememberLink(link string) {
	if link == "" {
		return
	}
	for _, existing := range st.NotifiedLinks {
		if existing == link {
			return
		}
	}
	st.NotifiedLinks = append(st.NotifiedLinks, link)
	if len(st.NotifiedLinks) > maxNotifiedLinks {
		st.NotifiedLinks = st.NotifiedLinks[len(st.NotifiedLinks)-maxNotifiedLinks:]
	}
}

// IsNew は state に基づき記事が未通知かどうかを返す。初回実行（LastRun 未設定）では false。
func (st RunState) IsNew(article Article) bool {
	if st.LastRun.IsZero() {
		return false
	}
	if !article.Published.IsZero() {
		return article.Published.After(st.LastRun)
	}
	link := safeURL(article.Link, "")
	if link == "" {
		return false
	}
	_, seen := st.notifiedSet()[link]
	return !seen
}
