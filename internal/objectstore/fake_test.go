package objectstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"testing"
)

type fakeObjectStore struct {
	mu      sync.Mutex
	objects map[string]fakeObject
}

type fakeObject struct {
	body []byte
	etag string
}

func newFakeObjectStore() *fakeObjectStore {
	return &fakeObjectStore{objects: make(map[string]fakeObject)}
}

func (f *fakeObjectStore) Get(_ context.Context, key string) ([]byte, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	obj, ok := f.objects[key]
	if !ok {
		return nil, "", ErrNotFound
	}
	return append([]byte(nil), obj.body...), obj.etag, nil
}

func (f *fakeObjectStore) Put(_ context.Context, key string, body []byte, ifMatchETag string) (string, error) {
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

func TestFakePutPrecondition(t *testing.T) {
	f := newFakeObjectStore()
	ctx := context.Background()
	etag, err := f.Put(ctx, "rss-reader.db", []byte("v1"), "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Put(ctx, "rss-reader.db", []byte("v2"), "wrong")
	if !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("err = %v", err)
	}
	etag2, err := f.Put(ctx, "rss-reader.db", []byte("v2"), etag)
	if err != nil || etag2 == "" {
		t.Fatalf("put ok: err=%v etag=%s", err, etag2)
	}
}
