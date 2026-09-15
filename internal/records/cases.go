package records

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	KindCivil    = "civil"
	KindCriminal = "criminal"
)

var (
	ErrCaseKind         = errors.New("case kind")
	ErrCaseTitle        = errors.New("case title")
	ErrLocationRedacted = errors.New("location redacted")
	ErrUnknownCase      = errors.New("unknown case")
	ErrCaseRedacted     = errors.New("case already redacted")
)

type Case struct {
	ID            string
	Title         string
	Note          string
	Kind          string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ClosedAt      *time.Time
	ClosedBy      string
	ClosedByName  string
	PhotoCount    int
	RedactedCount int
	FirstAt       *time.Time
	LastAt        *time.Time
	DueAt         *time.Time
	ThumbRecordID string
	Photos        []Record
}

type LocationRedaction struct {
	TenantID string
	PhotoID  string
}

func ParseCaseKind(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case KindCivil, "zivil", "zivilrecht", "zivilrechtlich":
		return KindCivil, nil
	case KindCriminal, "straf", "strafrecht", "strafrechtlich":
		return KindCriminal, nil
	default:
		return "", ErrCaseKind
	}
}

func RetentionYears(kind string) int {
	if kind == KindCriminal {
		return 5
	}
	return 3
}

func LocationDueAt(kind string, ref time.Time) time.Time {
	return ref.UTC().AddDate(RetentionYears(kind), 0, 0)
}

func RedactLocationExtra(extra map[string]any) map[string]any {
	if extra == nil {
		return map[string]any{}
	}
	delete(extra, "address")
	return extra
}

func applyCaseLocation(kind string, redactedAt, closedAt *time.Time) *time.Time {
	if kind == "" || redactedAt != nil || closedAt == nil {
		return nil
	}
	due := LocationDueAt(kind, *closedAt)
	return &due
}

func (r *Repo) CreateCase(ctx context.Context, tenantID, userID, title, note, kind, recordID string) (Case, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Case{}, ErrCaseTitle
	}
	kind, err := ParseCaseKind(kind)
	if err != nil {
		return Case{}, err
	}
	id := uuid.NewString()
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return Case{}, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
		INSERT INTO cases (id, tenant_id, title, note, kind, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		id, tenantID, title, strings.TrimSpace(note), kind, nullIfEmpty(userID))
	if err != nil {
		return Case{}, err
	}
	if recordID = strings.TrimSpace(recordID); recordID != "" {
		if err := assignPhotoCaseTx(ctx, tx, tenantID, id, recordID); err != nil {
			return Case{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Case{}, err
	}
	if recordID != "" {
		if err := r.redactIfDue(ctx, tenantID, recordID); err != nil {
			return Case{}, err
		}
	}
	return r.GetCase(ctx, tenantID, id)
}

func (r *Repo) ListCases(ctx context.Context, tenantID string) ([]Case, error) {
	rows, err := r.DB.Query(ctx, caseListSQL+` WHERE c.tenant_id = $1 GROUP BY c.id ORDER BY c.updated_at DESC, c.id`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Case
	for rows.Next() {
		c, err := scanCaseSummary(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if out == nil {
		out = []Case{}
	}
	return out, rows.Err()
}

func (r *Repo) GetCase(ctx context.Context, tenantID, id string) (Case, error) {
	c, err := scanCaseSummary(r.DB.QueryRow(ctx, caseListSQL+` WHERE c.tenant_id = $1 AND c.id = $2 GROUP BY c.id`, tenantID, id))
	if err != nil {
		return Case{}, err
	}
	items, err := r.casePhotos(ctx, tenantID, id)
	if err != nil {
		return Case{}, err
	}
	c.Photos = items
	return c, nil
}

func (r *Repo) UpdateCase(ctx context.Context, tenantID, id, title, note, kind string) (Case, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Case{}, ErrCaseTitle
	}
	kind, err := ParseCaseKind(kind)
	if err != nil {
		return Case{}, err
	}
	tag, err := r.DB.Exec(ctx, `
		UPDATE cases SET title = $3, note = $4, kind = $5, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, title, strings.TrimSpace(note), kind)
	if err != nil {
		return Case{}, err
	}
	if tag.RowsAffected() == 0 {
		return Case{}, pgx.ErrNoRows
	}
	if err := r.redactDueForCase(ctx, tenantID, id); err != nil {
		return Case{}, err
	}
	return r.GetCase(ctx, tenantID, id)
}

func (r *Repo) SetCaseClosed(ctx context.Context, tenantID, id, userID string, closed bool) (Case, error) {
	cur, err := r.GetCase(ctx, tenantID, id)
	if err != nil {
		return Case{}, err
	}
	if closed {
		if cur.ClosedAt != nil {
			return cur, nil
		}
		tag, err := r.DB.Exec(ctx, `
			UPDATE cases SET closed_at = now(), closed_by = $3, updated_at = now()
			WHERE tenant_id = $1 AND id = $2 AND closed_at IS NULL`,
			tenantID, id, nullIfEmpty(userID))
		if err != nil {
			return Case{}, err
		}
		if tag.RowsAffected() == 0 {
			return Case{}, pgx.ErrNoRows
		}
		if err := r.redactDueForCase(ctx, tenantID, id); err != nil {
			return Case{}, err
		}
	} else {
		if cur.RedactedCount > 0 {
			return Case{}, ErrCaseRedacted
		}
		if cur.ClosedAt == nil {
			return cur, nil
		}
		tag, err := r.DB.Exec(ctx, `
			UPDATE cases SET closed_at = NULL, closed_by = NULL, updated_at = now()
			WHERE tenant_id = $1 AND id = $2`, tenantID, id)
		if err != nil {
			return Case{}, err
		}
		if tag.RowsAffected() == 0 {
			return Case{}, pgx.ErrNoRows
		}
	}
	return r.GetCase(ctx, tenantID, id)
}

func (r *Repo) DeleteCase(ctx context.Context, tenantID, id string) error {
	tag, err := r.DB.Exec(ctx, `DELETE FROM cases WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repo) AssignPhotoCase(ctx context.Context, tenantID, caseID, recordID string) (Photo, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return Photo{}, err
	}
	defer tx.Rollback(ctx)
	if err := assignPhotoCaseTx(ctx, tx, tenantID, caseID, recordID); err != nil {
		return Photo{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Photo{}, err
	}
	if err := r.redactIfDue(ctx, tenantID, recordID); err != nil {
		return Photo{}, err
	}
	return r.GetPhoto(ctx, tenantID, recordID)
}

func (r *Repo) UnlinkPhotoCase(ctx context.Context, tenantID, caseID, recordID string) (Photo, error) {
	tag, err := r.DB.Exec(ctx, `
		UPDATE records SET case_id = NULL, updated_at = now()
		WHERE tenant_id = $1 AND id = $2 AND case_id = $3`,
		tenantID, recordID, caseID)
	if err != nil {
		return Photo{}, err
	}
	if tag.RowsAffected() == 0 {
		return Photo{}, pgx.ErrNoRows
	}
	return r.GetPhoto(ctx, tenantID, recordID)
}

func (r *Repo) RedactDueLocations(ctx context.Context) ([]LocationRedaction, error) {
	rows, err := r.DB.Query(ctx, `
		UPDATE records rec
		SET lat = NULL,
		    lon = NULL,
		    extra = COALESCE(rec.extra, '{}'::jsonb) - 'address',
		    location_redacted_at = now(),
		    updated_at = now()
		FROM cases c
		WHERE rec.case_id = c.id
		  AND rec.location_redacted_at IS NULL
		  AND c.closed_at IS NOT NULL
		  AND c.closed_at
		      + CASE c.kind WHEN 'criminal' THEN interval '5 years' ELSE interval '3 years' END
		      <= now()
		RETURNING rec.tenant_id::text, rec.id::text`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LocationRedaction
	for rows.Next() {
		var item LocationRedaction
		if err := rows.Scan(&item.TenantID, &item.PhotoID); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if out == nil {
		out = []LocationRedaction{}
	}
	return out, rows.Err()
}

func (r *Repo) decorateRecords(ctx context.Context, tenantID string, items []Record) error {
	if err := r.attachRecordFields(ctx, tenantID, items); err != nil {
		return err
	}
	return r.attachRecordCases(ctx, tenantID, items)
}

func (r *Repo) attachRecordCases(ctx context.Context, tenantID string, items []Record) error {
	if len(items) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	ids := make([]string, 0, len(items))
	byRecord := map[string][]int{}
	for i := range items {
		rid := items[i].RecordID
		if rid == "" {
			continue
		}
		byRecord[rid] = append(byRecord[rid], i)
		if _, ok := seen[rid]; ok {
			continue
		}
		seen[rid] = struct{}{}
		ids = append(ids, rid)
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := r.DB.Query(ctx, `
		SELECT r.id::text,
		       COALESCE(c.id::text, ''),
		       COALESCE(c.title, ''),
		       COALESCE(c.kind, ''),
		       r.location_redacted_at,
		       c.closed_at
		FROM records r
		LEFT JOIN cases c ON c.id = r.case_id
		WHERE r.tenant_id = $1 AND r.id = ANY($2::uuid[])`, tenantID, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var rid, caseID, title, kind string
		var redacted, closedAt *time.Time
		if err := rows.Scan(&rid, &caseID, &title, &kind, &redacted, &closedAt); err != nil {
			return err
		}
		for _, i := range byRecord[rid] {
			items[i].CaseID = caseID
			items[i].CaseTitle = title
			items[i].CaseKind = kind
			items[i].CaseClosedAt = closedAt
			items[i].LocationRedactedAt = redacted
			items[i].LocationDueAt = applyCaseLocation(kind, redacted, closedAt)
		}
	}
	return rows.Err()
}

func (r *Repo) casePhotos(ctx context.Context, tenantID, caseID string) ([]Record, error) {
	rows, err := r.DB.Query(ctx, selectSQL+`
		WHERE r.tenant_id = $1 AND r.case_id = $2
		ORDER BY COALESCE(r.captured_at, r.uploaded_at) ASC, s.id`, tenantID, caseID)
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

func (r *Repo) redactIfDue(ctx context.Context, tenantID, recordID string) error {
	_, err := r.DB.Exec(ctx, `
		UPDATE records rec
		SET lat = NULL,
		    lon = NULL,
		    extra = COALESCE(rec.extra, '{}'::jsonb) - 'address',
		    location_redacted_at = now(),
		    updated_at = now()
		FROM cases c
		WHERE rec.id = $2 AND rec.tenant_id = $1 AND rec.case_id = c.id
		  AND rec.location_redacted_at IS NULL
		  AND c.closed_at IS NOT NULL
		  AND c.closed_at
		      + CASE c.kind WHEN 'criminal' THEN interval '5 years' ELSE interval '3 years' END
		      <= now()`, tenantID, recordID)
	return err
}

func (r *Repo) redactDueForCase(ctx context.Context, tenantID, caseID string) error {
	_, err := r.DB.Exec(ctx, `
		UPDATE records rec
		SET lat = NULL,
		    lon = NULL,
		    extra = COALESCE(rec.extra, '{}'::jsonb) - 'address',
		    location_redacted_at = now(),
		    updated_at = now()
		FROM cases c
		WHERE rec.case_id = c.id AND c.id = $2 AND rec.tenant_id = $1
		  AND rec.location_redacted_at IS NULL
		  AND c.closed_at IS NOT NULL
		  AND c.closed_at
		      + CASE c.kind WHEN 'criminal' THEN interval '5 years' ELSE interval '3 years' END
		      <= now()`, tenantID, caseID)
	return err
}

func assignPhotoCaseTx(ctx context.Context, tx pgx.Tx, tenantID, caseID, recordID string) error {
	var exists string
	err := tx.QueryRow(ctx, `SELECT id FROM cases WHERE tenant_id = $1 AND id = $2`, tenantID, caseID).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUnknownCase
		}
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE records SET case_id = $3, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`, tenantID, recordID, caseID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	_, err = tx.Exec(ctx, `UPDATE cases SET updated_at = now() WHERE id = $1`, caseID)
	return err
}

const caseListSQL = `
	SELECT c.id, c.title, c.note, c.kind, c.created_at, c.updated_at,
	       c.closed_at, COALESCE(c.closed_by::text, ''),
	       COALESCE((SELECT u.username FROM users u WHERE u.id = c.closed_by), ''),
	       count(r.id),
	       count(r.id) FILTER (WHERE r.location_redacted_at IS NOT NULL),
	       min(COALESCE(r.captured_at, r.uploaded_at)),
	       max(COALESCE(r.captured_at, r.uploaded_at)),
	       CASE WHEN c.closed_at IS NULL THEN NULL
	            ELSE c.closed_at + CASE c.kind WHEN 'criminal' THEN interval '5 years' ELSE interval '3 years' END
	       END,
	       COALESCE((array_agg(r.id ORDER BY COALESCE(r.captured_at, r.uploaded_at) DESC, r.id))[1]::text, '')
	FROM cases c
	LEFT JOIN records r ON r.case_id = c.id
`

func scanCaseSummary(row scanner) (Case, error) {
	var c Case
	err := row.Scan(
		&c.ID, &c.Title, &c.Note, &c.Kind, &c.CreatedAt, &c.UpdatedAt,
		&c.ClosedAt, &c.ClosedBy, &c.ClosedByName,
		&c.PhotoCount, &c.RedactedCount, &c.FirstAt, &c.LastAt, &c.DueAt, &c.ThumbRecordID,
	)
	if err == pgx.ErrNoRows {
		return Case{}, err
	}
	return c, err
}
