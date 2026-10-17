package remotedb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ss49919201/andthen/result"
	"github.com/ss49919201/rss-reader/internal/objectstore"
	"github.com/ss49919201/rss-reader/internal/store"
)

const objectKey = "rss-reader.db"

// Session は R2 上の SQLite をダウンロードし、処理後にアップロードする。
type Session struct {
	Objects objectstore.ObjectStore
}

type localDB struct {
	sqlite  *store.SQLite
	ifMatch string
}

// WithStore は R2 から DB を取得して fn を実行し、変更をアップロードする。
func (s *Session) WithStore(ctx context.Context, fn func(store.Store) error) error {
	if s.Objects == nil {
		return fmt.Errorf("ObjectStore が未設定です")
	}
	dir, err := os.MkdirTemp("", "rss-db-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "rss-reader.db")

	ldb, err := openLocalDB(ctx, s.Objects, path).Get()
	if err != nil {
		return err
	}
	defer ldb.sqlite.Close()

	_, err = result.Ok(ldb).
		AndThrough(func(ldb localDB) result.Result[struct{}] {
			return result.From(struct{}{}, fn(ldb.sqlite))
		}).
		AndThen(func(ldb localDB) result.Result[struct{}] {
			newBody, err := ldb.sqlite.ExportBytes()
			if err != nil {
				return result.Err[struct{}](err)
			}
			_, err = s.Objects.Put(ctx, objectKey, newBody, ldb.ifMatch)
			return result.From(struct{}{}, err)
		}).
		Get()
	return err
}

func openLocalDB(ctx context.Context, objects objectstore.ObjectStore, path string) result.Result[localDB] {
	body, etag, err := objects.Get(ctx, objectKey)
	if err != nil && err != objectstore.ErrNotFound {
		return result.Err[localDB](err)
	}
	if err == nil {
		return result.From(struct{}{}, os.WriteFile(path, body, 0o600)).
			AndThen(func(struct{}) result.Result[localDB] {
				db, err := store.OpenFile(path)
				if err != nil {
					return result.Err[localDB](err)
				}
				return result.Ok(localDB{sqlite: db, ifMatch: etag})
			})
	}
	db, err := store.CreateEmptyFile(path)
	if err != nil {
		return result.Err[localDB](err)
	}
	return result.Ok(localDB{sqlite: db, ifMatch: ""})
}
