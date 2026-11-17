package storage

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// Store holds opaque objects addressed only by ID.
// Physical paths and bucket keys stay inside this package.
type Store interface {
	Put(ctx context.Context, id string, r io.Reader) error
	Open(ctx context.Context, id string) (io.ReadCloser, error)
	Delete(ctx context.Context, id string) error
}

// Options select the backend. Unused fields are ignored.
type Options struct {
	Backend   string
	Dir       string
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	Prefix    string
}

func Open(ctx context.Context, opt Options) (Store, error) {
	switch strings.ToLower(strings.TrimSpace(opt.Backend)) {
	case "", "local", "fs", "filesystem", "disk":
		return NewLocal(opt.Dir)
	case "s3":
		return NewS3(ctx, opt)
	default:
		return nil, fmt.Errorf("unbekanntes STORAGE_BACKEND %q (local oder s3)", opt.Backend)
	}
}
