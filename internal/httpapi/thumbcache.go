package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"schmutzfink/internal/ingest"
	"schmutzfink/internal/records"
)

func (s *Server) croppedThumb(id string, roi *records.ROI, data []byte) ([]byte, error) {
	if roi == nil {
		return ingest.Thumbnail(data)
	}
	if s.ThumbCache != "" {
		path := s.thumbCachePath(id, roi)
		if cached, err := os.ReadFile(path); err == nil && len(cached) > 0 {
			return cached, nil
		}
		thumb, err := ingest.ThumbnailCrop(data, roi.X, roi.Y, roi.W, roi.H)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
			_ = os.WriteFile(path, thumb, 0o644)
		}
		return thumb, nil
	}
	return ingest.ThumbnailCrop(data, roi.X, roi.Y, roi.W, roi.H)
}

func (s *Server) thumbCachePath(id string, roi *records.ROI) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%.4f:%.4f:%.4f:%.4f", id, roi.X, roi.Y, roi.W, roi.H)))
	name := hex.EncodeToString(sum[:]) + ".jpg"
	return filepath.Join(s.ThumbCache, name[:2], name)
}
