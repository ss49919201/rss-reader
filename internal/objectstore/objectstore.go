package objectstore

import (
	"context"
	"errors"
)

// ErrNotFound はオブジェクトが存在しないときに返す。
var ErrNotFound = errors.New("object not found")

// ErrPreconditionFailed は If-Match による条件付き PUT が失敗したときに返す。
var ErrPreconditionFailed = errors.New("precondition failed")

// ObjectStore はオブジェクトの取得と配置を行う。
type ObjectStore interface {
	Get(ctx context.Context, key string) (body []byte, etag string, err error)
	Put(ctx context.Context, key string, body []byte, ifMatchETag string) (etag string, err error)
}
