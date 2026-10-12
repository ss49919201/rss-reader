package rss

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStateIsNew(t *testing.T) {
	since := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	st := RunState{LastRun: since}
	newer := Article{Title: "a", Link: "https://example.com/n", Published: since.Add(time.Hour)}
	older := Article{Title: "b", Link: "https://example.com/o", Published: since.Add(-time.Hour)}
	if !st.IsNew(newer) || st.IsNew(older) {
		t.Fatal("published time filter failed")
	}
	empty := RunState{}
	if empty.IsNew(newer) {
		t.Fatal("first run should not notify")
	}
}

func TestSaveLoadState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	want := RunState{
		LastRun:       time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		NotifiedLinks: []string{"https://example.com/a"},
	}
	if err := SaveState(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if !got.LastRun.Equal(want.LastRun) || len(got.NotifiedLinks) != 1 {
		t.Fatalf("got %#v", got)
	}
}
