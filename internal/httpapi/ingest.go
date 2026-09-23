package httpapi

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"schmutzfink/internal/auth"
	"schmutzfink/internal/ingest"
	"schmutzfink/internal/records"
	"schmutzfink/internal/storage"
)

type duplicateError struct {
	Photo records.Photo
}

func (e *duplicateError) Error() string {
	return "Datei ist bereits vorhanden"
}

type photoInput struct {
	Lat        *float64
	Lon        *float64
	ClearGPS   bool
	CapturedAt *time.Time
	Note       string
	Tags       []string
	ROI        *records.ROI
	Extra      map[string]any
	Fields     map[string]string
	CaseID     string
}

func (s *Server) savePhoto(ctx context.Context, p auth.Principal, data []byte, in photoInput) (records.Photo, error) {
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	ct, err := ingest.Sniff(head)
	if err != nil {
		return records.Photo{}, err
	}
	if err := ingest.CheckDimensions(data); err != nil {
		return records.Photo{}, err
	}

	meta := ingest.Meta{ContentType: ct}
	if ct == "image/jpeg" {
		meta = ingest.ExtractJPEG(data)
		meta.ContentType = ct
	}
	if in.ClearGPS {
		meta.Lat = nil
		meta.Lon = nil
		in.Extra = setExtraAddress(in.Extra, nil)
	} else if in.Lat != nil && in.Lon != nil {
		meta.Lat = in.Lat
		meta.Lon = in.Lon
	}
	if meta.Lat != nil && meta.Lon != nil {
		if addr := addressFromExtra(in.Extra); addr != nil {
			// Mitgelieferte Adresse als Nähe-Basis für folgende Uploads.
			rememberResolvedAddress(*meta.Lat, *meta.Lon, addr)
		} else if addr := lookupAddress(ctx, *meta.Lat, *meta.Lon); addr != nil {
			in.Extra = setExtraAddress(in.Extra, addr)
		}
	}
	if in.CapturedAt != nil {
		meta.CapturedAt = in.CapturedAt
	}
	if in.CaseID != "" {
		if _, err := s.Records.GetCase(ctx, p.TenantID, in.CaseID); err != nil {
			return records.Photo{}, records.ErrUnknownCase
		}
	}

	payload, storedCT, err := ingest.NormalizeForStore(data, ct)
	if err != nil {
		return records.Photo{}, err
	}
	meta.ContentType = storedCT

	hash := ingest.SHA256Hex(payload)
	if id, err := s.Records.FindIDByHash(ctx, p.TenantID, hash); err == nil {
		photo, err := s.Records.GetPhoto(ctx, p.TenantID, id)
		if err != nil {
			return records.Photo{}, err
		}
		return photo, &duplicateError{Photo: photo}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return records.Photo{}, err
	}

	origID := storage.NewObjectID(time.Now())
	if err := s.Store.Put(ctx, origID, bytes.NewReader(payload)); err != nil {
		return records.Photo{}, err
	}
	thumbID := origID
	if thumb, err := ingest.Thumbnail(payload); err == nil {
		thumbID = storage.NewObjectID(time.Now())
		if err := s.Store.Put(ctx, thumbID, bytes.NewReader(thumb)); err != nil {
			_ = s.Store.Delete(ctx, origID)
			return records.Photo{}, err
		}
	}

	photo := records.Photo{
		ID:               uuid.NewString(),
		Lat:              meta.Lat,
		Lon:              meta.Lon,
		CapturedAt:       meta.CapturedAt,
		UploadedAt:       time.Now().UTC(),
		UploadedBy:       p.UserID,
		ContentType:      meta.ContentType,
		OriginalObjectID: origID,
		ThumbObjectID:    thumbID,
		ContentHash:      hash,
		Extra:            in.Extra,
		CaseID:           in.CaseID,
	}
	sighting := records.Sighting{
		ID:   uuid.NewString(),
		Note: in.Note,
		Tags: in.Tags,
		ROI:  in.ROI,
	}
	if err := s.Records.Insert(ctx, photo, sighting, p.TenantID); err != nil {
		_ = s.Store.Delete(ctx, origID)
		if thumbID != origID {
			_ = s.Store.Delete(ctx, thumbID)
		}
		if isContentHashConflict(err) {
			if id, findErr := s.Records.FindIDByHash(ctx, p.TenantID, hash); findErr == nil {
				existing, getErr := s.Records.GetPhoto(ctx, p.TenantID, id)
				if getErr == nil {
					return existing, &duplicateError{Photo: existing}
				}
			}
		}
		return records.Photo{}, err
	}
	if err := s.Records.SetSightingFields(ctx, p.TenantID, sighting.ID, in.Fields); err != nil {
		return records.Photo{}, err
	}
	stored, err := s.Records.GetPhoto(ctx, p.TenantID, photo.ID)
	if err != nil {
		return records.Photo{}, err
	}
	return stored, nil
}

func isContentHashConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
