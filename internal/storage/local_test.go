package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLocalRoundTrip(t *testing.T) {
	store, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	id := NewObjectID(time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC))
	payload := []byte("hallo-foto")
	if err := store.Put(ctx, id, bytes.NewReader(payload)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filepath.ToSlash(mustPath(t, store, id)), "/2026/09/") {
		t.Fatalf("expected year/month folders: %s", mustPath(t, store, id))
	}
	r, err := store.Open(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("got %q", got)
	}
	if err := store.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(ctx, id); err == nil {
		t.Fatal("expected missing after delete")
	}
}

func TestLocalLegacyID(t *testing.T) {
	store, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	if err := store.Put(context.Background(), id, bytes.NewReader([]byte("alt"))); err != nil {
		t.Fatal(err)
	}
	path := mustPath(t, store, id)
	if !strings.Contains(filepath.ToSlash(path), "/a1/b2/") {
		t.Fatalf("legacy shard missing: %s", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func mustPath(t *testing.T, store *Local, id string) string {
	t.Helper()
	p, err := store.pathFor(id)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

