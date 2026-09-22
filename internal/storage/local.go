package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Local struct {
	Root string
}

func NewLocal(root string) (*Local, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &Local{Root: abs}, nil
}

func (s *Local) pathFor(id string) (string, error) {
	key, err := objectKey(id)
	if err != nil {
		return "", err
	}
	full := filepath.Clean(filepath.Join(s.Root, filepath.FromSlash(key)))
	rel, err := filepath.Rel(s.Root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errInvalidID
	}
	return full, nil
}

func (s *Local) Put(_ context.Context, id string, r io.Reader) error {
	path, err := s.pathFor(id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func (s *Local) Open(_ context.Context, id string) (io.ReadCloser, error) {
	path, err := s.pathFor(id)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func (s *Local) Delete(_ context.Context, id string) error {
	path, err := s.pathFor(id)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
