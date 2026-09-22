package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"schmutzfink/internal/audit"
	"schmutzfink/internal/auth"
	"schmutzfink/internal/clip"
	"schmutzfink/internal/config"
	"schmutzfink/internal/db"
	"schmutzfink/internal/embedjob"
	"schmutzfink/internal/httpapi"
	"schmutzfink/internal/records"
	"schmutzfink/internal/retention"
	"schmutzfink/internal/storage"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("datenbank: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("migration: %v", err)
	}
	if err := db.Seed(ctx, pool, cfg.AdminUser, cfg.AdminPassword); err != nil {
		log.Fatalf("seed: %v", err)
	}

	store, err := storage.Open(ctx, storage.Options{
		Backend:   cfg.StorageBackend,
		Dir:       cfg.StorageDir,
		Endpoint:  cfg.S3Endpoint,
		Bucket:    cfg.S3Bucket,
		Region:    cfg.S3Region,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
		Prefix:    cfg.S3Prefix,
	})
	if err != nil {
		log.Fatalf("speicher: %v", err)
	}
	if cfg.StorageS3() {
		log.Printf("speicher: s3 %s %s", cfg.S3Endpoint, cfg.S3Bucket)
	} else {
		log.Printf("speicher: lokal %s", cfg.StorageDir)
	}

	engine := clip.New(cfg.Embeddings)
	defer engine.Close()
	repo := &records.Repo{DB: pool}
	jobs := embedjob.New(repo, store, engine)
	go jobs.Run(ctx)
	trail := &audit.Repo{DB: pool}
	go retention.Run(ctx, repo, trail)

	var directory auth.Directory
	if url := strings.TrimSpace(cfg.LDAPURL); url != "" {
		directory = auth.NewLDAP(auth.LDAPConfig{
			URL:                url,
			BindTemplate:       cfg.LDAPBindIdentity(),
			StartTLS:           cfg.LDAPStartTLS,
			InsecureSkipVerify: cfg.LDAPInsecureSkipVerify,
		})
		log.Printf("anmeldung: AD/LDAP %s", url)
		if cfg.LDAPInsecureSkipVerify {
			log.Print("warnung: LDAP_INSECURE_SKIP_VERIFY=1 — Zertifikat des Domain Controllers wird nicht geprüft")
		}
	}
	if tok := strings.TrimSpace(cfg.ProvisionToken); tok != "" && len(tok) < 16 {
		log.Print("warnung: PROVISION_TOKEN ist kürzer als 16 Zeichen — Provisioning bleibt aus")
	}

	srv := &httpapi.Server{
		Auth:            &auth.Service{DB: pool, Directory: directory},
		Records:         repo,
		Store:           store,
		Clip:            engine,
		Jobs:            jobs,
		WebDist:         cfg.WebDist,
		CookieSecure:    cfg.CookieSecure,
		TrustProxy:      cfg.TrustProxy,
		ThumbCache:      cfg.ResolvedThumbCache(),
		Audit:           trail,
		ProvisionToken:  cfg.ProvisionToken,
		ProvisionTenant: db.DefaultTenantID,
		MobileAPKPath:   cfg.MobileAPKPath,
	}

	if cfg.PublicBind() && cfg.DefaultSecrets() {
		log.Print("warnung: API lauscht nicht nur lokal und nutzt Standardgeheimnisse — HTTP_ADDR auf 127.0.0.1 setzen und Passwörter ändern")
	}

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       3 * time.Minute,
		IdleTimeout:       90 * time.Second,
	}
	go func() {
		<-ctx.Done()
		_ = httpSrv.Shutdown(context.Background())
	}()

	log.Printf("Schmutzfink hört auf %s", cfg.HTTPAddr)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
