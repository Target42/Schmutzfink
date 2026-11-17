package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"
)

func TestS3MinIORoundTrip(t *testing.T) {
	store := minioStore(t)
	ctx := context.Background()
	id := NewObjectID(time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC))
	payload := []byte("minio-foto")
	if err := store.Put(ctx, id, bytes.NewReader(payload)); err != nil {
		t.Fatal(err)
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

func TestS3LegacyKey(t *testing.T) {
	store := minioStore(t)
	ctx := context.Background()
	id := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	if err := store.Put(ctx, id, bytes.NewReader([]byte("legacy"))); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Delete(ctx, id) })
	r, err := store.Open(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "legacy" {
		t.Fatalf("got %q", got)
	}
}

func TestOpenRejectsUnknownBackend(t *testing.T) {
	_, err := Open(context.Background(), Options{Backend: "ftp"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func minioStore(t *testing.T) *S3 {
	t.Helper()
	ctx := context.Background()
	store, err := NewS3(ctx, Options{
		Endpoint:  envOr("S3_ENDPOINT", "http://127.0.0.1:9000"),
		Bucket:    envOr("S3_BUCKET", "schmutzfink-test"),
		Region:    "us-east-1",
		AccessKey: envOr("S3_ACCESS_KEY", "schmutzfink"),
		SecretKey: envOr("S3_SECRET_KEY", "schmutzfink"),
		Prefix:    "tests",
	})
	if err != nil {
		t.Skipf("minio nicht erreichbar: %v", err)
	}
	return store
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
