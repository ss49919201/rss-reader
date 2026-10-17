package remotedb

import (
	"context"
	"errors"
	"testing"

	"github.com/ss49919201/rss-reader/internal/objectstore"
	"github.com/ss49919201/rss-reader/internal/site"
	"github.com/ss49919201/rss-reader/internal/store"
)

type fakeObjects struct {
	body []byte
	etag string
	getErr error
	putErr error
	puts   int
}

func (f *fakeObjects) Get(_ context.Context, _ string) ([]byte, string, error) {
	if f.getErr != nil {
		return nil, "", f.getErr
	}
	if f.body == nil {
		return nil, "", objectstore.ErrNotFound
	}
	return f.body, f.etag, nil
}

func (f *fakeObjects) Put(_ context.Context, _ string, body []byte, ifMatch string) (string, error) {
	f.puts++
	if f.putErr != nil {
		return "", f.putErr
	}
	hadObject := f.body != nil
	if hadObject && ifMatch != f.etag {
		return "", errors.New("unexpected if-match")
	}
	if !hadObject && ifMatch != "" {
		return "", errors.New("unexpected if-match")
	}
	f.body = append([]byte(nil), body...)
	f.etag = "stored-etag"
	return f.etag, nil
}

func TestWithStoreCreatesEmptyWhenMissing(t *testing.T) {
	objects := &fakeObjects{}
	session := &Session{Objects: objects}
	ctx := context.Background()
	err := session.WithStore(ctx, func(st store.Store) error {
		return st.AddSite(ctx, site.Site{ID: "demo", Name: "Demo", URL: "https://example.com/rss"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if objects.puts != 1 || objects.body == nil {
		t.Fatalf("put=%d body=%v", objects.puts, objects.body)
	}
}

func TestWithStoreRoundTripsExistingObject(t *testing.T) {
	objects := &fakeObjects{}
	session := &Session{Objects: objects}
	ctx := context.Background()
	err := session.WithStore(ctx, func(st store.Store) error {
		return st.AddSite(ctx, site.Site{ID: "demo", Name: "Demo", URL: "https://example.com/rss"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if objects.puts != 1 {
		t.Fatalf("puts=%d", objects.puts)
	}
	err = session.WithStore(ctx, func(st store.Store) error {
		sites, err := st.ListSites(ctx)
		if err != nil {
			return err
		}
		if len(sites) != 1 || sites[0].ID != "demo" {
			t.Fatalf("sites=%#v", sites)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if objects.puts != 2 {
		t.Fatalf("puts=%d", objects.puts)
	}
}

func TestWithStorePropagatesCallbackError(t *testing.T) {
	objects := &fakeObjects{}
	session := &Session{Objects: objects}
	want := errors.New("callback failed")
	err := session.WithStore(context.Background(), func(store.Store) error {
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
	if objects.puts != 0 {
		t.Fatalf("put=%d want 0 on callback error", objects.puts)
	}
}
