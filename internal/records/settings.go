package records

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (r *Repo) PublicMap(ctx context.Context, tenantID string) (bool, error) {
	var on bool
	err := r.DB.QueryRow(ctx, `SELECT public_map FROM tenants WHERE id = $1`, tenantID).Scan(&on)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return on, err
}

func (r *Repo) SetPublicMap(ctx context.Context, tenantID string, on bool) error {
	tag, err := r.DB.Exec(ctx, `UPDATE tenants SET public_map = $2 WHERE id = $1`, tenantID, on)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
