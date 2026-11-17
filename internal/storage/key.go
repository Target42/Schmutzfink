package storage

import (
	"errors"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
)

var errInvalidID = errors.New("ungültige object-id")

// NewObjectID builds a storage key: year/month plus a hex shard, then a UUID.
// The full key is what Postgres stores, so local disk and S3 stay aligned.
func NewObjectID(now time.Time) string {
	id := uuid.NewString()
	hex := strings.ReplaceAll(id, "-", "")
	stamp := now.UTC().Format("2006/01")
	if len(hex) < 4 {
		return stamp + "/zz/" + id
	}
	return stamp + "/" + hex[0:2] + "/" + hex[2:4] + "/" + id
}

// objectKey maps an ID to the relative object key.
// New IDs already contain year/month/shard. Legacy UUID-only IDs keep the old hex layout.
func objectKey(id string) (string, error) {
	id = strings.TrimSpace(strings.ReplaceAll(id, "\\", "/"))
	id = strings.Trim(id, "/")
	if id == "" || strings.Contains(id, "..") {
		return "", errInvalidID
	}
	if strings.Contains(id, "/") {
		cleaned := path.Clean(id)
		if cleaned == "." || strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, "..") {
			return "", errInvalidID
		}
		return cleaned, nil
	}
	hex := strings.ReplaceAll(id, "-", "")
	if len(hex) < 4 {
		return path.Join("zz", id), nil
	}
	return path.Join(hex[0:2], hex[2:4], id), nil
}
