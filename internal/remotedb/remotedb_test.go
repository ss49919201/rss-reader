package remotedb

import (
	"context"
	"testing"

	"github.com/ss49919201/rss-reader/internal/objectstore"
	"github.com/ss49919201/rss-reader/internal/site"
	"github.com/ss49919201/rss-reader/internal/store"
)

func TestWithStoreUploadsChanges(t *testing.T) {
	fake := objectstore.NewFake()
	session := &Session{Objects: fake, Key: "rss-reader.db"}
	ctx := context.Background()

	err := session.WithStore(ctx, func(st store.Store) error {
		return st.AddSite(ctx, site.Site{ID: "demo", Name: "Demo", URL: "https://example.com/feed"})
	})
	if err != nil {
		t.Fatal(err)
	}

	err = session.WithStore(ctx, func(st store.Store) error {
		sites, err := st.ListSites(ctx)
		if err != nil {
			return err
		}
		if len(sites) != 1 || sites[0].ID != "demo" {
			t.Fatalf("sites = %#v", sites)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWithStoreCreatesMissingObject(t *testing.T) {
	fake := objectstore.NewFake()
	session := &Session{Objects: fake, Key: "rss-reader.db"}
	ctx := context.Background()
	if err := session.WithStore(ctx, func(st store.Store) error {
		sites, err := st.ListSites(ctx)
		if err != nil {
			return err
		}
		if len(sites) != 0 {
			t.Fatalf("sites = %#v", sites)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	_, _, err := fake.Get(ctx, "rss-reader.db")
	if err != nil {
		t.Fatal(err)
	}
}
