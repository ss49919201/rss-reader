package objectstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

// Fake はテスト用のインメモリ ObjectStore。
type Fake struct {
	mu      sync.Mutex
	objects map[string]fakeObject
}

type fakeObject struct {
	body []byte
	etag string
}

// NewFake は空の Fake を作る。
func NewFake() *Fake {
	return &Fake{objects: make(map[string]fakeObject)}
}

func (f *Fake) Get(_ context.Context, key string) ([]byte, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	obj, ok := f.objects[key]
	if !ok {
		return nil, "", ErrNotFound
	}
	return append([]byte(nil), obj.body...), obj.etag, nil
}

func (f *Fake) Put(_ context.Context, key string, body []byte, ifMatchETag string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if ifMatchETag != "" {
		obj, ok := f.objects[key]
		if !ok || obj.etag != ifMatchETag {
			return "", ErrPreconditionFailed
		}
	}
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:]) + `"`
	f.objects[key] = fakeObject{body: append([]byte(nil), body...), etag: etag}
	return etag, nil
}
