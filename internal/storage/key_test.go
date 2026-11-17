package storage

import (
	"strings"
	"testing"
	"time"
)

func TestNewObjectID(t *testing.T) {
	id := NewObjectID(time.Date(2026, 9, 13, 15, 4, 0, 0, time.UTC))
	parts := strings.Split(id, "/")
	if len(parts) != 5 {
		t.Fatalf("want year/month/aa/bb/uuid, got %q", id)
	}
	if parts[0] != "2026" || parts[1] != "09" {
		t.Fatalf("year/month: %q", id)
	}
	hex := strings.ReplaceAll(parts[4], "-", "")
	if !strings.HasPrefix(hex, parts[2]+parts[3]) {
		t.Fatalf("shard %s/%s does not match uuid %s", parts[2], parts[3], parts[4])
	}
}

func TestObjectKeyLegacy(t *testing.T) {
	id := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	got, err := objectKey(id)
	if err != nil {
		t.Fatal(err)
	}
	want := "a1/b2/" + id
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestObjectKeyRejectsTraversal(t *testing.T) {
	for _, id := range []string{"", "..", "../secret", "2026/09/../x", "a\\..\\b"} {
		if _, err := objectKey(id); err == nil {
			t.Fatalf("expected error for %q", id)
		}
	}
}

func TestObjectKeyNewLayout(t *testing.T) {
	id := "2026/09/a1/b2/a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	got, err := objectKey(id)
	if err != nil {
		t.Fatal(err)
	}
	if got != id {
		t.Fatalf("got %q", got)
	}
}
