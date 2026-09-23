package httpapi

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAndroidMetaPath(t *testing.T) {
	got := androidMetaPath(`D:\data\mobile\schmutzfink-android.apk`)
	want := `D:\data\mobile\schmutzfink-android.json`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestLoadAndroidMobileInfoMissing(t *testing.T) {
	s := &Server{MobileAPKPath: filepath.Join(t.TempDir(), "missing.apk")}
	info := s.loadAndroidMobileInfo()
	if info.Available {
		t.Fatal("expected unavailable")
	}
}

func TestLoadAndroidMobileInfoWithMeta(t *testing.T) {
	dir := t.TempDir()
	apk := filepath.Join(dir, "schmutzfink-android.apk")
	if err := os.WriteFile(apk, []byte("apk-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	meta := filepath.Join(dir, "schmutzfink-android.json")
	if err := os.WriteFile(meta, []byte(`{"version":"1.2.3","version_code":4,"released_at":"2026-09-20T12:00:00Z","notes":"Feldtest"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Server{MobileAPKPath: apk}
	info := s.loadAndroidMobileInfo()
	if !info.Available {
		t.Fatal("expected available")
	}
	if info.Version != "1.2.3" || info.VersionCode != 4 {
		t.Fatalf("version: %+v", info)
	}
	if info.Notes != "Feldtest" {
		t.Fatalf("notes: %q", info.Notes)
	}
	if info.SizeBytes != 9 {
		t.Fatalf("size: %d", info.SizeBytes)
	}
}
