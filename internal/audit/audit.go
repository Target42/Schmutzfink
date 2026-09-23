package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	Login           = "login"
	LoginFailed     = "login_failed"
	LoginDisabled   = "login_disabled"
	LoginLocked     = "login_locked"
	Logout          = "logout"
	PasswordChange  = "password_change"
	PasswordReset   = "password_reset"
	UserCreate      = "user_create"
	UserDisable     = "user_disable"
	UserEnable      = "user_enable"
	UserPatch       = "user_patch"
	RecordUpload    = "record_upload"
	RecordImport    = "record_import"
	RecordPatch     = "record_patch"
	RecordDelete    = "record_delete"
	DeletionRequest = "deletion_request"
	DeletionCancel  = "deletion_cancel"
	DeletionReject  = "deletion_reject"
	SightingAdd     = "sighting_add"
	SightingPatch   = "sighting_patch"
	SightingDelete  = "sighting_delete"
	MotifCreate     = "motif_create"
	MotifPatch      = "motif_patch"
	MotifDelete     = "motif_delete"
	MotifAssign     = "motif_assign"
	MotifUnlink     = "motif_unlink"
	FieldCreate     = "field_create"
	FieldPatch      = "field_patch"
	FieldDelete     = "field_delete"
	RecordExport    = "record_export"
	TokenCreate     = "token_create"
	TokenRevoke     = "token_revoke"
	CaseCreate      = "case_create"
	CasePatch       = "case_patch"
	CaseDelete      = "case_delete"
	CaseAssign      = "case_assign"
	CaseUnlink      = "case_unlink"
	CaseClose       = "case_close"
	CaseReopen      = "case_reopen"
	LocationRedact  = "location_redact"
)

type Event struct {
	ID       string         `json:"id"`
	At       time.Time      `json:"at"`
	TenantID string         `json:"tenant_id,omitempty"`
	UserID   string         `json:"user_id,omitempty"`
	Username string         `json:"username"`
	Action   string         `json:"action"`
	Subject  string         `json:"subject"`
	IP       string         `json:"ip"`
	Detail   map[string]any `json:"detail,omitempty"`
}

type Repo struct {
	DB *pgxpool.Pool
}

func (r *Repo) Write(ctx context.Context, ev Event) error {
	if r == nil || r.DB == nil {
		return nil
	}
	if ev.ID == "" {
		ev.ID = uuid.NewString()
	}
	if ev.At.IsZero() {
		ev.At = time.Now().UTC()
	}
	detail := json.RawMessage(`{}`)
	if ev.Detail != nil {
		b, err := json.Marshal(ev.Detail)
		if err != nil {
			return err
		}
		detail = b
	}
	_, err := r.DB.Exec(ctx, `
		INSERT INTO audit_events (id, at, tenant_id, user_id, username, action, subject, ip, detail)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		ev.ID, ev.At, nilIfEmpty(ev.TenantID), nilIfEmpty(ev.UserID), ev.Username,
		ev.Action, ev.Subject, ev.IP, detail)
	return err
}

func (r *Repo) List(ctx context.Context, tenantID string, limit, offset int) ([]Event, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	err := r.DB.QueryRow(ctx, `
		SELECT count(*) FROM audit_events
		WHERE tenant_id = $1 OR tenant_id IS NULL`, tenantID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.Query(ctx, `
		SELECT id, at, COALESCE(tenant_id::text, ''), COALESCE(user_id::text, ''),
		       username, action, subject, ip, detail
		FROM audit_events
		WHERE tenant_id = $1 OR tenant_id IS NULL
		ORDER BY at DESC, id
		LIMIT $2 OFFSET $3`, tenantID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]Event, 0, limit)
	for rows.Next() {
		var ev Event
		var raw []byte
		if err := rows.Scan(
			&ev.ID, &ev.At, &ev.TenantID, &ev.UserID,
			&ev.Username, &ev.Action, &ev.Subject, &ev.IP, &raw,
		); err != nil {
			return nil, 0, err
		}
		if len(raw) > 0 && string(raw) != "{}" && string(raw) != "null" {
			_ = json.Unmarshal(raw, &ev.Detail)
		}
		out = append(out, ev)
	}
	return out, total, rows.Err()
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
