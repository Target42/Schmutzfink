package records

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
)

func (r *Repo) SearchByVector(ctx context.Context, tenantID string, vec []float32, f ListFilter, excludeID string) ([]Record, int, error) {
	applyListLimit(&f)
	f.SkipText = true
	where, args := buildWhere(tenantID, f)
	where += ` AND s.embedding IS NOT NULL`
	if excludeID != "" {
		args = append(args, excludeID)
		where += ` AND s.id <> $` + itoa(len(args))
	}
	args = append(args, pgvector.NewVector(vec))
	vecPH := itoa(len(args))
	if f.MinScore != nil {
		args = append(args, *f.MinScore)
		where += ` AND (1 - (s.embedding <=> $` + vecPH + `)) >= $` + itoa(len(args))
	}

	var total int
	if err := r.DB.QueryRow(ctx, `SELECT count(*) FROM sightings s JOIN records r ON r.id = s.record_id `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	selectSQL := `
	SELECT s.id, r.id, s.note, s.tags, r.lat, r.lon, r.captured_at, r.uploaded_at,
	       r.uploaded_by, u.username, r.content_type, r.original_object_id, r.thumb_object_id,
	       s.embedding_status, 1 - (s.embedding <=> $` + vecPH + `)::float8 AS score,
	       s.roi_x, s.roi_y, s.roi_w, s.roi_h,
	       COALESCE(s.motif_id::text, ''), COALESCE(m.title, '')
	FROM sightings s
	JOIN records r ON r.id = s.record_id
	JOIN users u ON u.id = r.uploaded_by
	LEFT JOIN motifs m ON m.id = s.motif_id
`
	args = append(args, f.Limit, f.Offset)
	rows, err := r.DB.Query(ctx, selectSQL+where+` ORDER BY s.embedding <=> $`+vecPH+` LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)), args...)
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

func (r *Repo) Similar(ctx context.Context, tenantID, id string, limit int, minScore float64) ([]Record, error) {
	if limit <= 0 || limit > 40 {
		limit = 12
	}
	var vec pgvector.Vector
	err := r.DB.QueryRow(ctx, `
		SELECT s.embedding
		FROM sightings s
		JOIN records r ON r.id = s.record_id
		WHERE r.tenant_id = $1 AND s.id = $2 AND s.embedding IS NOT NULL`, tenantID, id).Scan(&vec)
	if err != nil {
		return nil, err
	}
	min := ClampMinScore(minScore)
	items, _, err := r.SearchByVector(ctx, tenantID, vec.Slice(), ListFilter{Limit: limit, MinScore: &min}, id)
	return items, err
}

func (r *Repo) ClaimPending(ctx context.Context) (id, objectID string, roi *ROI, gen int, err error) {
	var x, y, w, h *float64
	err = r.DB.QueryRow(ctx, `
		WITH next AS (
			SELECT id FROM sightings
			WHERE embedding_status = 'pending'
			ORDER BY created_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE sightings s
		SET embedding_status = 'processing', embedding_error = '', updated_at = now()
		FROM next, records rec
		WHERE s.id = next.id AND rec.id = s.record_id
		RETURNING s.id, rec.original_object_id, s.roi_x, s.roi_y, s.roi_w, s.roi_h, s.embedding_gen`).Scan(
		&id, &objectID, &x, &y, &w, &h, &gen)
	if err == nil {
		return id, objectID, roiFromNulls(x, y, w, h), gen, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil, 0, nil
	}
	return "", "", nil, 0, err
}

func (r *Repo) SaveEmbedding(ctx context.Context, id string, gen int, vec []float32) error {
	_, err := r.DB.Exec(ctx, `
		UPDATE sightings
		SET embedding = $3, embedding_status = 'ready', embedding_error = '', updated_at = now()
		WHERE id = $1 AND embedding_gen = $2 AND embedding_status = 'processing'`,
		id, gen, pgvector.NewVector(vec))
	return err
}

func (r *Repo) FailEmbedding(ctx context.Context, id string, gen int, msg string) error {
	if len(msg) > 500 {
		msg = msg[:500]
	}
	_, err := r.DB.Exec(ctx, `
		UPDATE sightings
		SET embedding_status = 'failed', embedding_error = $2, updated_at = now()
		WHERE id = $1 AND embedding_gen = $3 AND embedding_status = 'processing'`,
		id, msg, gen)
	return err
}

func (r *Repo) RequeueEmbedding(ctx context.Context, tenantID, id string) error {
	_, err := r.DB.Exec(ctx, `
		UPDATE sightings s
		SET embedding_status = 'pending',
		    embedding = NULL,
		    embedding_error = '',
		    embedding_gen = embedding_gen + 1,
		    updated_at = now()
		FROM records r
		WHERE s.id = $2 AND r.id = s.record_id AND r.tenant_id = $1`, tenantID, id)
	return err
}

func (r *Repo) RequeueStuck(ctx context.Context, olderThan time.Duration) error {
	_, err := r.DB.Exec(ctx, `
		UPDATE sightings
		SET embedding_status = 'pending', updated_at = now()
		WHERE embedding_status = 'processing'
		  AND updated_at < now() - $1::interval`, formatInterval(olderThan))
	return err
}

func (r *Repo) EmbeddingCounts(ctx context.Context, tenantID string) (pending, ready, failed int, err error) {
	err = r.DB.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE s.embedding_status IN ('pending', 'processing')),
			count(*) FILTER (WHERE s.embedding_status = 'ready'),
			count(*) FILTER (WHERE s.embedding_status = 'failed')
		FROM sightings s
		JOIN records r ON r.id = s.record_id
		WHERE r.tenant_id = $1`, tenantID).Scan(&pending, &ready, &failed)
	return
}

func formatInterval(d time.Duration) string {
	sec := int(d.Seconds())
	if sec < 1 {
		sec = 1
	}
	return itoa(sec) + " seconds"
}
