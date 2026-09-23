package config

import (
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr               string
	DatabaseURL            string
	StorageBackend         string
	StorageDir             string
	ThumbCacheDir          string
	S3Endpoint             string
	S3Bucket               string
	S3Region               string
	S3AccessKey            string
	S3SecretKey            string
	S3Prefix               string
	AdminUser              string
	AdminPassword          string
	WebDist                string
	Embeddings             bool
	CookieSecure           string
	TrustProxy             string
	LDAPURL                string
	LDAPDomain             string
	LDAPBindTemplate       string
	LDAPStartTLS           bool
	LDAPInsecureSkipVerify bool
	ProvisionToken         string
	MobileAPKPath          string
}

func Load() Config {
	_ = godotenv.Load()
	if exe, err := os.Executable(); err == nil {
		_ = godotenv.Load(filepath.Join(filepath.Dir(exe), ".env"))
	}
	return Config{
		HTTPAddr:               env("HTTP_ADDR", "127.0.0.1:8787"),
		DatabaseURL:            env("DATABASE_URL", "postgres://schmutzfink:schmutzfink@127.0.0.1:55432/schmutzfink?sslmode=disable"),
		StorageBackend:         env("STORAGE_BACKEND", "local"),
		StorageDir:             env("STORAGE_DIR", "./data/objects"),
		ThumbCacheDir:          env("THUMB_CACHE_DIR", ""),
		S3Endpoint:             env("S3_ENDPOINT", ""),
		S3Bucket:               env("S3_BUCKET", ""),
		S3Region:               env("S3_REGION", "us-east-1"),
		S3AccessKey:            env("S3_ACCESS_KEY", ""),
		S3SecretKey:            env("S3_SECRET_KEY", ""),
		S3Prefix:               env("S3_PREFIX", ""),
		AdminUser:              env("ADMIN_USER", "admin"),
		AdminPassword:          env("ADMIN_PASSWORD", "admin"),
		WebDist:                env("WEB_DIST", "./web/dist"),
		Embeddings:             env("EMBEDDINGS", "1") != "0",
		CookieSecure:           env("COOKIE_SECURE", "auto"),
		TrustProxy:             env("TRUST_PROXY", "auto"),
		LDAPURL:                env("LDAP_URL", ""),
		LDAPDomain:             env("LDAP_DOMAIN", ""),
		LDAPBindTemplate:       env("LDAP_BIND_TEMPLATE", ""),
		LDAPStartTLS:           env("LDAP_STARTTLS", "0") == "1",
		LDAPInsecureSkipVerify: env("LDAP_INSECURE_SKIP_VERIFY", "0") == "1",
		ProvisionToken:         env("PROVISION_TOKEN", ""),
		MobileAPKPath:          env("MOBILE_APK_PATH", "./data/mobile/schmutzfink-android.apk"),
	}
}

func (c Config) StorageS3() bool {
	return strings.EqualFold(strings.TrimSpace(c.StorageBackend), "s3")
}

func (c Config) ResolvedThumbCache() string {
	if dir := strings.TrimSpace(c.ThumbCacheDir); dir != "" {
		return dir
	}
	if c.StorageS3() {
		return "./data/_thumbs"
	}
	return filepath.Join(c.StorageDir, "_thumbs")
}

func (c Config) PublicBind() bool {
	host := c.HTTPAddr
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	} else if strings.HasPrefix(host, ":") {
		host = ""
	}
	host = strings.TrimSpace(host)
	return host == "" || host == "0.0.0.0" || host == "::" || host == "[::]"
}

func (c Config) DefaultSecrets() bool {
	pass := strings.TrimSpace(c.AdminPassword)
	if pass == "" || strings.EqualFold(pass, "admin") || strings.EqualFold(pass, "change_me") || strings.EqualFold(pass, "changeme") {
		return true
	}
	if tok := strings.TrimSpace(c.ProvisionToken); tok != "" && (len(tok) < 16 || strings.EqualFold(tok, "change_me") || strings.EqualFold(tok, "changeme")) {
		return true
	}
	return strings.Contains(c.DatabaseURL, "schmutzfink:schmutzfink")
}

func (c Config) LDAPBindIdentity() string {
	if t := strings.TrimSpace(c.LDAPBindTemplate); t != "" {
		return t
	}
	domain := strings.TrimSpace(c.LDAPDomain)
	if domain == "" {
		return "{username}"
	}
	if strings.Contains(domain, ".") {
		return "{username}@" + domain
	}
	return domain + `\{username}`
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
