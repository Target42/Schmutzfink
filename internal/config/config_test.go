package config

import (
	"path/filepath"
	"testing"
)

func TestPublicBind(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{addr: "127.0.0.1:8787", want: false},
		{addr: "localhost:8787", want: false},
		{addr: ":8787", want: true},
		{addr: "0.0.0.0:8787", want: true},
		{addr: "[::]:8787", want: true},
	}
	for _, tc := range cases {
		c := Config{HTTPAddr: tc.addr}
		if got := c.PublicBind(); got != tc.want {
			t.Errorf("PublicBind(%q) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}

func TestDefaultSecrets(t *testing.T) {
	c := Config{AdminPassword: "ein-gutes-kennwort", DatabaseURL: "postgres://a:b@127.0.0.1/db"}
	if c.DefaultSecrets() {
		t.Fatal("custom secrets")
	}
	c.AdminPassword = "admin"
	if !c.DefaultSecrets() {
		t.Fatal("admin password")
	}
	c.AdminPassword = "ein-gutes-kennwort"
	c.ProvisionToken = "kurz"
	if !c.DefaultSecrets() {
		t.Fatal("short provision token")
	}
}

func TestResolvedThumbCache(t *testing.T) {
	local := Config{StorageBackend: "local", StorageDir: "./data/objects"}
	if got, want := local.ResolvedThumbCache(), filepath.Join("./data/objects", "_thumbs"); got != want {
		t.Fatalf("local cache %q want %q", got, want)
	}
	s3 := Config{StorageBackend: "s3"}
	if got := s3.ResolvedThumbCache(); got != "./data/_thumbs" {
		t.Fatalf("s3 cache %q", got)
	}
	custom := Config{StorageBackend: "s3", ThumbCacheDir: "E:/cache"}
	if got := custom.ResolvedThumbCache(); got != "E:/cache" {
		t.Fatalf("custom cache %q", got)
	}
}

func TestLDAPBindIdentity(t *testing.T) {
	c := Config{LDAPDomain: "stadt.local"}
	if got := c.LDAPBindIdentity(); got != "{username}@stadt.local" {
		t.Fatalf("domain: %q", got)
	}
	c.LDAPBindTemplate = `STADT\{username}`
	if got := c.LDAPBindIdentity(); got != `STADT\{username}` {
		t.Fatalf("template wins: %q", got)
	}
}

func TestLoadStorageEnv(t *testing.T) {
	t.Setenv("STORAGE_BACKEND", "s3")
	t.Setenv("S3_BUCKET", "test-bucket")
	t.Setenv("S3_ENDPOINT", "http://127.0.0.1:9000")
	c := Load()
	if !c.StorageS3() || c.S3Bucket != "test-bucket" || c.S3Endpoint != "http://127.0.0.1:9000" {
		t.Fatalf("loaded %+v", c)
	}
}
