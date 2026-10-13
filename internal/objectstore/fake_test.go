package objectstore

import (
	"context"
	"errors"
	"testing"
)

func TestFakePutPrecondition(t *testing.T) {
	f := NewFake()
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
