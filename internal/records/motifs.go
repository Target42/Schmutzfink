package records

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrMotifSightingRequired = errors.New("sighting required")

type Motif struct {
	ID              string
	Title           string
	Note            string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	SightingCount   int
	FirstAt         *time.Time
	LastAt          *time.Time
	ThumbSightingID string
	Sightings       []Record
}

func MotifTitleFrom(note string) string {
	t := strings.TrimSpace(note)
	if t == "" {
		return "Motiv"
	}
	if utf8.RuneCountInString(t) <= 80 {
		return t
	}
	return string([]rune(t)[:80])
}

func (r *Repo) CreateMotif(ctx context.Context, tenantID, userID, sightingID, title, note string) (Motif, error) {
	if strings.TrimSpace(sightingID) == "" {
		return Motif{}, ErrMotifSightingRequired
	}
	cur, err := r.GetSighting(ctx, tenantID, sightingID)
	if err != nil {
		return Motif{}, err
	}
	if title = strings.TrimSpace(title); title == "" {
		title = MotifTitleFrom(cur.Note)
	}
	id := uuid.NewString()
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return Motif{}, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
		INSERT INTO motifs (id, tenant_id, title, note, created_by)
		VALUES ($1, $2, $3, $4, $5)`,
		id, tenantID, title, strings.TrimSpace(note), nullIfEmpty(userID))
	if err != nil {
		return Motif{}, err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE sightings SET motif_id = $3, updated_at = now()
		WHERE id = $2 AND tenant_id = $1`, tenantID, sightingID, id)
	if err != nil {
		return Motif{}, err
	}
	if tag.RowsAffected() == 0 {
		return Motif{}, pgx.ErrNoRows
	}
	if err := tx.Commit(ctx); err != nil {
		return Motif{}, err
	}
	return r.GetMotif(ctx, tenantID, id)
}

func (r *Repo) ListMotifs(ctx context.Context, tenantID string) ([]Motif, error) {
	rows, err := r.DB.Query(ctx, motifListSQL+` WHERE m.tenant_id = $1 GROUP BY m.id ORDER BY m.updated_at DESC, m.id`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Motif
	for rows.Next() {
		m, err := scanMotifSummary(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if out == nil {
		out = []Motif{}
	}
	return out, rows.Err()
}

func (r *Repo) GetMotif(ctx context.Context, tenantID, id string) (Motif, error) {
	m, err := scanMotifSummary(r.DB.QueryRow(ctx, motifListSQL+` WHERE m.tenant_id = $1 AND m.id = $2 GROUP BY m.id`, tenantID, id))
	if err != nil {
		return Motif{}, err
	}
	items, err := r.motifSightings(ctx, tenantID, id)
	if err != nil {
		return Motif{}, err
	}
	m.Sightings = items
	return m, nil
}

func (r *Repo) UpdateMotif(ctx context.Context, tenantID, id, title, note string) (Motif, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Motiv"
	}
	tag, err := r.DB.Exec(ctx, `
		UPDATE motifs SET title = $3, note = $4, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`, tenantID, id, title, strings.TrimSpace(note))
	if err != nil {
		return Motif{}, err
	}
	if tag.RowsAffected() == 0 {
		return Motif{}, pgx.ErrNoRows
	}
	return r.GetMotif(ctx, tenantID, id)
}

func (r *Repo) DeleteMotif(ctx context.Context, tenantID, id string) error {
	tag, err := r.DB.Exec(ctx, `DELETE FROM motifs WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repo) AssignSighting(ctx context.Context, tenantID, motifID, sightingID string) (Motif, error) {
	if _, err := r.GetSighting(ctx, tenantID, sightingID); err != nil {
		return Motif{}, err
	}
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return Motif{}, err
	}
	defer tx.Rollback(ctx)
	var exists string
	err = tx.QueryRow(ctx, `SELECT id FROM motifs WHERE tenant_id = $1 AND id = $2`, tenantID, motifID).Scan(&exists)
	if err != nil {
		return Motif{}, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE sightings SET motif_id = $3, updated_at = now()
		WHERE id = $2 AND tenant_id = $1`, tenantID, sightingID, motifID)
	if err != nil {
		return Motif{}, err
	}
	_, err = tx.Exec(ctx, `UPDATE motifs SET updated_at = now() WHERE id = $1`, motifID)
	if err != nil {
		return Motif{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Motif{}, err
	}
	return r.GetMotif(ctx, tenantID, motifID)
}

func (r *Repo) UnlinkSighting(ctx context.Context, tenantID, motifID, sightingID string) (Motif, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return Motif{}, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		UPDATE sightings SET motif_id = NULL, updated_at = now()
		WHERE id = $3 AND motif_id = $2 AND tenant_id = $1`, tenantID, motifID, sightingID)
	if err != nil {
		return Motif{}, err
	}
	if tag.RowsAffected() == 0 {
		return Motif{}, pgx.ErrNoRows
	}
	var left int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM sightings WHERE motif_id = $1`, motifID).Scan(&left); err != nil {
		return Motif{}, err
	}
	if left == 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM motifs WHERE tenant_id = $1 AND id = $2`, tenantID, motifID); err != nil {
			return Motif{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Motif{}, err
		}
		return Motif{}, pgx.ErrNoRows
	}
	if _, err := tx.Exec(ctx, `UPDATE motifs SET updated_at = now() WHERE id = $1`, motifID); err != nil {
		return Motif{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Motif{}, err
	}
	return r.GetMotif(ctx, tenantID, motifID)
}

func (r *Repo) MotifSuggestions(ctx context.Context, tenantID, motifID string, limit int, minScore float64) ([]Record, error) {
	if limit <= 0 || limit > 40 {
		limit = 12
	}
	if _, err := r.GetMotif(ctx, tenantID, motifID); err != nil {
		return nil, err
	}
	min := ClampMinScore(minScore)
	rows, err := r.DB.Query(ctx, `
		WITH members AS (
			SELECT s.id, s.embedding
			FROM sightings s
			JOIN records r ON r.id = s.record_id
			WHERE r.tenant_id = $1 AND s.motif_id = $2 AND s.embedding IS NOT NULL
		)
		SELECT s.id, r.id, s.note, s.tags, r.lat, r.lon, r.captured_at, r.uploaded_at,
		       r.uploaded_by, u.username, r.content_type, r.original_object_id, r.thumb_object_id,
		       s.embedding_status, max(1 - (s.embedding <=> m.embedding))::float8 AS score,
		       s.roi_x, s.roi_y, s.roi_w, s.roi_h,
		       COALESCE(s.motif_id::text, ''), COALESCE(om.title, '')
		FROM sightings s
		JOIN records r ON r.id = s.record_id
		JOIN users u ON u.id = r.uploaded_by
		JOIN members m ON s.id <> m.id
		LEFT JOIN motifs om ON om.id = s.motif_id
		WHERE r.tenant_id = $1
		  AND s.embedding IS NOT NULL
		  AND (s.motif_id IS NULL OR s.motif_id <> $2)
		GROUP BY s.id, r.id, s.note, s.tags, r.lat, r.lon, r.captured_at, r.uploaded_at,
		         r.uploaded_by, u.username, r.content_type, r.original_object_id, r.thumb_object_id,
		         s.embedding_status, s.roi_x, s.roi_y, s.roi_w, s.roi_h, s.motif_id, om.title
		HAVING max(1 - (s.embedding <=> m.embedding)) >= $3
		ORDER BY score DESC, s.id
		LIMIT $4`, tenantID, motifID, min, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		rec, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if out == nil {
		out = []Record{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.decorateRecords(ctx, tenantID, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repo) motifSightings(ctx context.Context, tenantID, motifID string) ([]Record, error) {
	rows, err := r.DB.Query(ctx, selectSQL+`
		WHERE r.tenant_id = $1 AND s.motif_id = $2
		ORDER BY COALESCE(r.captured_at, r.uploaded_at) ASC, s.id`, tenantID, motifID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		rec, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if out == nil {
		out = []Record{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.decorateRecords(ctx, tenantID, out); err != nil {
		return nil, err
	}
	return out, nil
}

const motifListSQL = `
	SELECT m.id, m.title, m.note, m.created_at, m.updated_at,
	       count(s.id),
	       min(COALESCE(r.captured_at, r.uploaded_at)),
	       max(COALESCE(r.captured_at, r.uploaded_at)),
	       COALESCE((array_agg(s.id ORDER BY COALESCE(r.captured_at, r.uploaded_at) DESC, s.id))[1]::text, '')
	FROM motifs m
	LEFT JOIN sightings s ON s.motif_id = m.id
	LEFT JOIN records r ON r.id = s.record_id
`

func scanMotifSummary(row scanner) (Motif, error) {
	var m Motif
	err := row.Scan(
		&m.ID, &m.Title, &m.Note, &m.CreatedAt, &m.UpdatedAt,
		&m.SightingCount, &m.FirstAt, &m.LastAt, &m.ThumbSightingID,
	)
	if err == pgx.ErrNoRows {
		return Motif{}, err
	}
	return m, err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
