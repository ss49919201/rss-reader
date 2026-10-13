package remotedb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ss49919201/rss-reader/internal/objectstore"
	"github.com/ss49919201/rss-reader/internal/store"
)

// Session は R2 上の SQLite をダウンロードし、処理後にアップロードする。
type Session struct {
	Objects objectstore.ObjectStore
	Key     string
}

// WithStore は R2 から DB を取得して fn を実行し、変更をアップロードする。
func (s *Session) WithStore(ctx context.Context, fn func(store.Store) error) error {
	if s.Objects == nil {
		return fmt.Errorf("ObjectStore が未設定です")
	}
	if s.Key == "" {
		return fmt.Errorf("オブジェクトキーが空です")
	}
	dir, err := os.MkdirTemp("", "rss-db-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "rss-reader.db")

	body, etag, err := s.Objects.Get(ctx, s.Key)
	if err != nil && err != objectstore.ErrNotFound {
		return err
	}
	hadObject := err == nil

	var db *store.SQLite
	if hadObject {
		if err := os.WriteFile(path, body, 0o600); err != nil {
			return err
		}
		db, err = store.OpenFile(path)
		if err != nil {
			return err
		}
	} else {
		db, err = store.CreateEmptyFile(path)
		if err != nil {
			return err
		}
	}
	defer db.Close()

	if err := fn(db); err != nil {
		return err
	}

	newBody, err := db.ExportBytes()
	if err != nil {
		return err
	}
	ifMatch := ""
	if hadObject {
		ifMatch = etag
	}
	_, err = s.Objects.Put(ctx, s.Key, newBody, ifMatch)
	if err != nil {
		return err
	}
	return nil
}
