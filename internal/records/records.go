package records

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrLastSighting    = errors.New("last sighting")
	ErrDeletionPending = errors.New("deletion already requested")
	ErrNoDeletion      = errors.New("no deletion request")
)

type DeletionRequest struct {
	RequestedAt     time.Time
	RequestedBy     string
	RequestedByName string
	Reason          string
}

type Record struct {
	ID                 string            `json:"id"`
	RecordID           string            `json:"record_id"`
	Note               string            `json:"note"`
	Tags               []string          `json:"tags"`
	Lat                *float64          `json:"lat"`
	Lon                *float64          `json:"lon"`
	CapturedAt         *time.Time        `json:"captured_at"`
	UploadedAt         time.Time         `json:"uploaded_at"`
	UploadedBy         string            `json:"uploaded_by"`
	UploadedByName     string            `json:"uploaded_by_name"`
	ContentType        string            `json:"content_type"`
	OriginalObjectID   string            `json:"-"`
	ThumbObjectID      string            `json:"-"`
	EmbeddingStatus    string            `json:"embedding_status"`
	Score              *float64          `json:"score,omitempty"`
	ROI                *ROI              `json:"roi,omitempty"`
	MotifID            string            `json:"motif_id,omitempty"`
	MotifTitle         string            `json:"motif_title,omitempty"`
	CaseID             string            `json:"case_id,omitempty"`
	CaseTitle          string            `json:"case_title,omitempty"`
	CaseKind           string            `json:"case_kind,omitempty"`
	CaseClosedAt       *time.Time        `json:"case_closed_at,omitempty"`
	LocationRedactedAt *time.Time        `json:"location_redacted_at,omitempty"`
	LocationDueAt      *time.Time        `json:"location_due_at,omitempty"`
	Fields             map[string]string `json:"fields,omitempty"`
}

type Photo struct {
	ID                 string
	Lat                *float64
	Lon                *float64
	CapturedAt         *time.Time
	UploadedAt         time.Time
	UploadedBy         string
	UploadedByName     string
	ContentType        string
	OriginalObjectID   string
	ThumbObjectID      string
	ContentHash        string
	Extra              map[string]any
	Sightings          []Sighting
	Deletion           *DeletionRequest
	CaseID             string
	CaseTitle          string
	CaseKind           string
	CaseClosedAt       *time.Time
	LocationRedactedAt *time.Time
	LocationDueAt      *time.Time
}

type Sighting struct {
	ID              string
	RecordID        string
	Note            string
	Tags            []string
	ROI             *ROI
	EmbeddingStatus string
	MotifID         string
	MotifTitle      string
	Fields          map[string]string
}

type ListFilter struct {
	Query        string
	From         *time.Time
	To           *time.Time
	HasGPS       *bool
	NearLat      *float64
	NearLon      *float64
	RadiusM      *float64
	SkipText     bool
	MinScore     *float64
	MotifID      string
	CaseID       string
	FieldFilters []FieldFilter
	Limit        int
	Offset       int
	ForExport    bool
}

const ExportMax = 10000

type Repo struct {
	DB *pgxpool.Pool
}

func (r *Repo) Insert(ctx context.Context, photo Photo, sighting Sighting, tenantID string) error {
	if strings.TrimSpace(photo.ContentHash) == "" {
		return errors.New("content hash fehlt")
	}
	if sighting.Tags == nil {
		sighting.Tags = []string{}
	}
	extra := json.RawMessage(`{}`)
	if photo.Extra != nil {
		b, err := json.Marshal(photo.Extra)
		if err != nil {
			return err
		}
		extra = b
	}
	rx, ry, rw, rh := roiCols(sighting.ROI)
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
		INSERT INTO records (
			id, tenant_id, original_object_id, thumb_object_id, content_type,
			lat, lon, captured_at, uploaded_at, uploaded_by, extra, content_hash, case_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		photo.ID, tenantID, photo.OriginalObjectID, photo.ThumbObjectID, photo.ContentType,
		photo.Lat, photo.Lon, photo.CapturedAt, photo.UploadedAt, photo.UploadedBy, extra, photo.ContentHash,
		nullIfEmpty(photo.CaseID))
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO sightings (
			id, record_id, tenant_id, note, tags, embedding_status,
			roi_x, roi_y, roi_w, roi_h
		) VALUES ($1, $2, $3, $4, $5, 'pending', $6, $7, $8, $9)`,
		sighting.ID, photo.ID, tenantID, sighting.Note, sighting.Tags, rx, ry, rw, rh)
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if photo.CaseID != "" {
		return r.redactIfDue(ctx, tenantID, photo.ID)
	}
	return nil
}

func (r *Repo) FindIDByHash(ctx context.Context, tenantID, hash string) (string, error) {
	if hash == "" {
		return "", pgx.ErrNoRows
	}
	var id string
	err := r.DB.QueryRow(ctx, `
		SELECT id FROM records WHERE tenant_id = $1 AND content_hash = $2`, tenantID, hash).Scan(&id)
	return id, err
}

func (r *Repo) GetPhoto(ctx context.Context, tenantID, id string) (Photo, error) {
	photo, err := scanPhoto(r.DB.QueryRow(ctx, photoSQL+` WHERE r.tenant_id = $1 AND r.id = $2`, tenantID, id))
	if err != nil {
		return Photo{}, err
	}
	sightings, err := r.listSightings(ctx, tenantID, id)
	if err != nil {
		return Photo{}, err
	}
	photo.Sightings = sightings
	return photo, nil
}

func (r *Repo) GetSighting(ctx context.Context, tenantID, id string) (Record, error) {
	rec, err := scanOne(r.DB.QueryRow(ctx, selectSQL+` WHERE r.tenant_id = $1 AND s.id = $2`, tenantID, id))
	if err != nil {
		return Record{}, err
	}
	items := []Record{rec}
	if err := r.decorateRecords(ctx, tenantID, items); err != nil {
		return Record{}, err
	}
	return items[0], nil
}

func (r *Repo) List(ctx context.Context, tenantID string, f ListFilter) ([]Record, int, error) {
	applyListLimit(&f)
	where, args := buildWhere(tenantID, f)
	var total int
	countSQL := `SELECT count(*) FROM sightings s JOIN records r ON r.id = s.record_id ` + where
	if err := r.DB.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.Limit, f.Offset)
	rows, err := r.DB.Query(ctx, selectSQL+where+` ORDER BY COALESCE(r.captured_at, r.uploaded_at) DESC, r.uploaded_at DESC, s.id LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		rec, err := scanRow(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, rec)
	}
	if out == nil {
		out = []Record{}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := r.decorateRecords(ctx, tenantID, out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *Repo) MapPoints(ctx context.Context, tenantID string, f ListFilter) ([]Record, error) {
	gps := true
	f.HasGPS = &gps
	f.Limit = 2000
	f.Offset = 0
	items, _, err := r.List(ctx, tenantID, f)
	return items, err
}

func (r *Repo) UpdatePhoto(ctx context.Context, tenantID, id string, photo Photo) (Photo, error) {
	cur, err := r.GetPhoto(ctx, tenantID, id)
	if err != nil {
		return Photo{}, err
	}
	if cur.LocationRedactedAt != nil && (floatChanged(cur.Lat, photo.Lat) || floatChanged(cur.Lon, photo.Lon)) {
		return Photo{}, ErrLocationRedacted
	}
	_, err = r.DB.Exec(ctx, `
		UPDATE records SET
			lat = $3,
			lon = $4,
			captured_at = $5,
			updated_at = now()
		WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, photo.Lat, photo.Lon, photo.CapturedAt)
	if err != nil {
		return Photo{}, err
	}
	return r.GetPhoto(ctx, tenantID, id)
}

func (r *Repo) AddSighting(ctx context.Context, tenantID, recordID string, in Sighting) (Sighting, error) {
	if in.Tags == nil {
		in.Tags = []string{}
	}
	rx, ry, rw, rh := roiCols(in.ROI)
	var sid string
	err := r.DB.QueryRow(ctx, `
		INSERT INTO sightings (
			id, record_id, tenant_id, note, tags, embedding_status,
			roi_x, roi_y, roi_w, roi_h
		)
		SELECT $3, r.id, r.tenant_id, $4, $5, 'pending', $6, $7, $8, $9
		FROM records r
		WHERE r.tenant_id = $1 AND r.id = $2
		RETURNING id`,
		tenantID, recordID, in.ID, in.Note, in.Tags, rx, ry, rw, rh).Scan(&sid)
	if err != nil {
		return Sighting{}, err
	}
	items, err := r.listSightings(ctx, tenantID, recordID)
	if err != nil {
		return Sighting{}, err
	}
	for _, s := range items {
		if s.ID == sid {
			return s, nil
		}
	}
	return Sighting{}, pgx.ErrNoRows
}

func (r *Repo) UpdateSighting(ctx context.Context, tenantID, id string, in Sighting) (Sighting, error) {
	if in.Tags == nil {
		in.Tags = []string{}
	}
	rx, ry, rw, rh := roiCols(in.ROI)
	var recordID string
	err := r.DB.QueryRow(ctx, `
		UPDATE sightings s
		SET note = $3, tags = $4, roi_x = $5, roi_y = $6, roi_w = $7, roi_h = $8, updated_at = now()
		FROM records r
		WHERE s.id = $2 AND r.id = s.record_id AND r.tenant_id = $1
		RETURNING s.record_id`,
		tenantID, id, in.Note, in.Tags, rx, ry, rw, rh).Scan(&recordID)
	if err != nil {
		return Sighting{}, err
	}
	items, err := r.listSightings(ctx, tenantID, recordID)
	if err != nil {
		return Sighting{}, err
	}
	for _, s := range items {
		if s.ID == id {
			return s, nil
		}
	}
	return Sighting{}, pgx.ErrNoRows
}

func (r *Repo) DeleteSighting(ctx context.Context, tenantID, id string) (Photo, error) {
	cur, err := r.GetSighting(ctx, tenantID, id)
	if err != nil {
		return Photo{}, err
	}
	var n int
	if err := r.DB.QueryRow(ctx, `
		SELECT count(*) FROM sightings s
		JOIN records r ON r.id = s.record_id
		WHERE r.tenant_id = $1 AND s.record_id = $2`, tenantID, cur.RecordID).Scan(&n); err != nil {
		return Photo{}, err
	}
	if n <= 1 {
		if cur.ROI == nil {
			return Photo{}, ErrLastSighting
		}
		cleared := Sighting{ID: cur.ID, RecordID: cur.RecordID, Note: cur.Note, Tags: cur.Tags}
		if _, err := r.UpdateSighting(ctx, tenantID, id, cleared); err != nil {
			return Photo{}, err
		}
		if err := r.RequeueEmbedding(ctx, tenantID, id); err != nil {
			return Photo{}, err
		}
		return r.GetPhoto(ctx, tenantID, cur.RecordID)
	}
	_, err = r.DB.Exec(ctx, `
		DELETE FROM sightings s
		USING records r
		WHERE s.id = $2 AND r.id = s.record_id AND r.tenant_id = $1`, tenantID, id)
	if err != nil {
		return Photo{}, err
	}
	return r.GetPhoto(ctx, tenantID, cur.RecordID)
}

func (r *Repo) RequestDeletion(ctx context.Context, tenantID, id, userID, reason string) (Photo, error) {
	photo, err := r.GetPhoto(ctx, tenantID, id)
	if err != nil {
		return Photo{}, err
	}
	if photo.Deletion != nil && photo.Deletion.RequestedBy != userID {
		return Photo{}, ErrDeletionPending
	}
	reason = strings.TrimSpace(reason)
	if len([]rune(reason)) > 500 {
		reason = string([]rune(reason)[:500])
	}
	_, err = r.DB.Exec(ctx, `
		UPDATE records SET
			deletion_requested_at = COALESCE(deletion_requested_at, now()),
			deletion_requested_by = $3,
			deletion_reason = $4,
			updated_at = now()
		WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, userID, reason)
	if err != nil {
		return Photo{}, err
	}
	return r.GetPhoto(ctx, tenantID, id)
}

func (r *Repo) ClearDeletion(ctx context.Context, tenantID, id string) (Photo, error) {
	photo, err := r.GetPhoto(ctx, tenantID, id)
	if err != nil {
		return Photo{}, err
	}
	if photo.Deletion == nil {
		return Photo{}, ErrNoDeletion
	}
	_, err = r.DB.Exec(ctx, `
		UPDATE records SET
			deletion_requested_at = NULL,
			deletion_requested_by = NULL,
			deletion_reason = '',
			updated_at = now()
		WHERE tenant_id = $1 AND id = $2`,
		tenantID, id)
	if err != nil {
		return Photo{}, err
	}
	return r.GetPhoto(ctx, tenantID, id)
}

func (r *Repo) ListPendingDeletions(ctx context.Context, tenantID string) ([]Photo, error) {
	rows, err := r.DB.Query(ctx, photoSQL+`
		WHERE r.tenant_id = $1 AND r.deletion_requested_at IS NOT NULL
		ORDER BY r.deletion_requested_at, r.id`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Photo
	for rows.Next() {
		photo, err := scanPhoto(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, photo)
	}
	if out == nil {
		out = []Photo{}
	}
	return out, rows.Err()
}

func (r *Repo) DeletePhoto(ctx context.Context, tenantID, id string) (Photo, error) {
	photo, err := r.GetPhoto(ctx, tenantID, id)
	if err != nil {
		return Photo{}, err
	}
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return Photo{}, err
	}
	defer tx.Rollback(ctx)

	var motifIDs []string
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT motif_id::text FROM sightings
		WHERE record_id = $1 AND motif_id IS NOT NULL`, id)
	if err != nil {
		return Photo{}, err
	}
	for rows.Next() {
		var mid string
		if err := rows.Scan(&mid); err != nil {
			rows.Close()
			return Photo{}, err
		}
		motifIDs = append(motifIDs, mid)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Photo{}, err
	}

	tag, err := tx.Exec(ctx, `DELETE FROM records WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return Photo{}, err
	}
	if tag.RowsAffected() == 0 {
		return Photo{}, pgx.ErrNoRows
	}
	if len(motifIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			DELETE FROM motifs m
			WHERE m.tenant_id = $1 AND m.id = ANY($2::uuid[])
			  AND NOT EXISTS (SELECT 1 FROM sightings s WHERE s.motif_id = m.id)`,
			tenantID, motifIDs); err != nil {
			return Photo{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Photo{}, err
	}
	return photo, nil
}

func (r *Repo) listSightings(ctx context.Context, tenantID, recordID string) ([]Sighting, error) {
	rows, err := r.DB.Query(ctx, `
		SELECT s.id, s.record_id, s.note, s.tags, s.embedding_status,
		       s.roi_x, s.roi_y, s.roi_w, s.roi_h,
		       COALESCE(s.motif_id::text, ''), COALESCE(m.title, '')
		FROM sightings s
		JOIN records r ON r.id = s.record_id
		LEFT JOIN motifs m ON m.id = s.motif_id
		WHERE r.tenant_id = $1 AND s.record_id = $2
		ORDER BY s.created_at, s.id`, tenantID, recordID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Sighting
	for rows.Next() {
		s, err := scanSighting(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if out == nil {
		out = []Sighting{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.attachSightingFields(ctx, tenantID, out); err != nil {
		return nil, err
	}
	return out, nil
}

const photoSQL = `
	SELECT r.id, r.lat, r.lon, r.captured_at, r.uploaded_at,
	       r.uploaded_by, u.username, r.content_type, r.original_object_id, r.thumb_object_id,
	       r.deletion_requested_at, COALESCE(r.deletion_requested_by::text, ''),
	       COALESCE(d.username, ''), r.deletion_reason,
	       COALESCE(c.id::text, ''), COALESCE(c.title, ''), COALESCE(c.kind, ''),
	       r.location_redacted_at, c.closed_at
	FROM records r
	JOIN users u ON u.id = r.uploaded_by
	LEFT JOIN users d ON d.id = r.deletion_requested_by
	LEFT JOIN cases c ON c.id = r.case_id
`

const selectSQL = `
	SELECT s.id, r.id, s.note, s.tags, r.lat, r.lon, r.captured_at, r.uploaded_at,
	       r.uploaded_by, u.username, r.content_type, r.original_object_id, r.thumb_object_id,
	       s.embedding_status, NULL::float8 AS score,
	       s.roi_x, s.roi_y, s.roi_w, s.roi_h,
	       COALESCE(s.motif_id::text, ''), COALESCE(m.title, '')
	FROM sightings s
	JOIN records r ON r.id = s.record_id
	JOIN users u ON u.id = r.uploaded_by
	LEFT JOIN motifs m ON m.id = s.motif_id
`

func buildWhere(tenantID string, f ListFilter) (string, []any) {
	args := []any{tenantID}
	parts := []string{`WHERE r.tenant_id = $1`}
	if q := strings.TrimSpace(f.Query); q != "" && !f.SkipText {
		args = append(args, likeContains(q))
		n := len(args)
		parts = append(parts, `AND (
			s.note ILIKE $`+itoa(n)+` ESCAPE '\' OR
			EXISTS (SELECT 1 FROM unnest(s.tags) AS tag WHERE tag ILIKE $`+itoa(n)+` ESCAPE '\') OR
			EXISTS (
				SELECT 1 FROM sighting_field_values v
				JOIN custom_fields f ON f.id = v.field_id
				WHERE v.sighting_id = s.id AND f.tenant_id = $1
				  AND (v.value_text ILIKE $`+itoa(n)+` ESCAPE '\' OR f.label ILIKE $`+itoa(n)+` ESCAPE '\')
			) OR
			EXISTS (
				SELECT 1 FROM motifs mo
				WHERE mo.id = s.motif_id AND mo.tenant_id = $1 AND mo.title ILIKE $`+itoa(n)+` ESCAPE '\'
			) OR
			EXISTS (
				SELECT 1 FROM cases ca
				WHERE ca.id = r.case_id AND ca.tenant_id = $1 AND ca.title ILIKE $`+itoa(n)+` ESCAPE '\'
			)
		)`)
	}
	if f.From != nil {
		args = append(args, *f.From)
		parts = append(parts, `AND COALESCE(r.captured_at, r.uploaded_at) >= $`+itoa(len(args)))
	}
	if f.To != nil {
		args = append(args, *f.To)
		parts = append(parts, `AND COALESCE(r.captured_at, r.uploaded_at) < $`+itoa(len(args)))
	}
	if f.HasGPS != nil {
		if *f.HasGPS {
			parts = append(parts, `AND r.lat IS NOT NULL AND r.lon IS NOT NULL`)
		} else {
			parts = append(parts, `AND (r.lat IS NULL OR r.lon IS NULL)`)
		}
	}
	if f.NearLat != nil && f.NearLon != nil && f.RadiusM != nil {
		lat, lon, radius := *f.NearLat, *f.NearLon, *f.RadiusM
		minLat, maxLat, minLon, maxLon := geoBBox(lat, lon, radius)
		args = append(args, minLat, maxLat, minLon, maxLon, lat, lon, radius)
		n := len(args)
		parts = append(parts,
			`AND r.lat IS NOT NULL AND r.lon IS NOT NULL`,
			`AND r.lat BETWEEN $`+itoa(n-6)+` AND $`+itoa(n-5),
			`AND r.lon BETWEEN $`+itoa(n-4)+` AND $`+itoa(n-3),
			`AND (2 * 6371000 * asin(sqrt(LEAST(1.0,
				power(sin(radians(r.lat - $`+itoa(n-2)+`)/2), 2) +
				cos(radians($`+itoa(n-2)+`)) * cos(radians(r.lat)) *
				power(sin(radians(r.lon - $`+itoa(n-1)+`)/2), 2)
			)))) <= $`+itoa(n),
		)
	}
	switch mid := strings.TrimSpace(f.MotifID); mid {
	case "":
	case "none":
		parts = append(parts, `AND s.motif_id IS NULL`)
	default:
		args = append(args, mid)
		parts = append(parts, `AND s.motif_id = $`+itoa(len(args)))
	}
	switch cid := strings.TrimSpace(f.CaseID); cid {
	case "":
	case "none":
		parts = append(parts, `AND r.case_id IS NULL`)
	default:
		args = append(args, cid)
		parts = append(parts, `AND r.case_id = $`+itoa(len(args)))
	}
	for _, ff := range f.FieldFilters {
		key := strings.TrimSpace(ff.Key)
		val := strings.TrimSpace(ff.Value)
		if key == "" || val == "" {
			continue
		}
		args = append(args, key, likeContains(val))
		n := len(args)
		parts = append(parts, `AND EXISTS (
			SELECT 1 FROM sighting_field_values v
			JOIN custom_fields f ON f.id = v.field_id
			WHERE v.sighting_id = s.id AND f.tenant_id = $1
			  AND f.key = $`+itoa(n-1)+`
			  AND v.value_text ILIKE $`+itoa(n)+` ESCAPE '\'
		)`)
	}
	return strings.Join(parts, " "), args
}

func applyListLimit(f *ListFilter) {
	max, def := 200, 40
	if f.ForExport {
		max, def = ExportMax, ExportMax
	}
	if f.Limit <= 0 || f.Limit > max {
		f.Limit = def
	}
}

func geoBBox(lat, lon, radiusM float64) (minLat, maxLat, minLon, maxLon float64) {
	const metersPerDeg = 111320.0
	dLat := radiusM / metersPerDeg
	minLat = math.Max(-90, lat-dLat)
	maxLat = math.Min(90, lat+dLat)
	cos := math.Cos(lat * math.Pi / 180)
	if cos < 0.01 {
		cos = 0.01
	}
	dLon := radiusM / (metersPerDeg * cos)
	minLon = math.Max(-180, lon-dLon)
	maxLon = math.Min(180, lon+dLon)
	return
}

func likeContains(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return `%` + s + `%`
}

func floatChanged(a, b *float64) bool {
	if a == nil && b == nil {
		return false
	}
	if a == nil || b == nil {
		return true
	}
	return *a != *b
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

type scanner interface {
	Scan(dest ...any) error
}

func scanOne(row scanner) (Record, error) {
	rec, err := scanRow(row)
	if err == pgx.ErrNoRows {
		return Record{}, err
	}
	return rec, err
}

func scanRow(row scanner) (Record, error) {
	var rec Record
	var rx, ry, rw, rh *float64
	err := row.Scan(
		&rec.ID, &rec.RecordID, &rec.Note, &rec.Tags, &rec.Lat, &rec.Lon, &rec.CapturedAt, &rec.UploadedAt,
		&rec.UploadedBy, &rec.UploadedByName, &rec.ContentType, &rec.OriginalObjectID, &rec.ThumbObjectID,
		&rec.EmbeddingStatus, &rec.Score, &rx, &ry, &rw, &rh, &rec.MotifID, &rec.MotifTitle,
	)
	if rec.Tags == nil {
		rec.Tags = []string{}
	}
	rec.ROI = roiFromNulls(rx, ry, rw, rh)
	return rec, err
}

func scanPhoto(row scanner) (Photo, error) {
	var p Photo
	var requestedAt *time.Time
	var requestedBy, requestedByName, reason string
	err := row.Scan(
		&p.ID, &p.Lat, &p.Lon, &p.CapturedAt, &p.UploadedAt,
		&p.UploadedBy, &p.UploadedByName, &p.ContentType, &p.OriginalObjectID, &p.ThumbObjectID,
		&requestedAt, &requestedBy, &requestedByName, &reason,
		&p.CaseID, &p.CaseTitle, &p.CaseKind, &p.LocationRedactedAt, &p.CaseClosedAt,
	)
	if err == pgx.ErrNoRows {
		return Photo{}, err
	}
	if err != nil {
		return Photo{}, err
	}
	p.LocationDueAt = applyCaseLocation(p.CaseKind, p.LocationRedactedAt, p.CaseClosedAt)
	if requestedAt != nil {
		p.Deletion = &DeletionRequest{
			RequestedAt:     *requestedAt,
			RequestedBy:     requestedBy,
			RequestedByName: requestedByName,
			Reason:          reason,
		}
	}
	return p, nil
}

func scanSighting(row scanner) (Sighting, error) {
	var s Sighting
	var rx, ry, rw, rh *float64
	err := row.Scan(&s.ID, &s.RecordID, &s.Note, &s.Tags, &s.EmbeddingStatus, &rx, &ry, &rw, &rh, &s.MotifID, &s.MotifTitle)
	if s.Tags == nil {
		s.Tags = []string{}
	}
	s.ROI = roiFromNulls(rx, ry, rw, rh)
	return s, err
}
