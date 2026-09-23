package db

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"schmutzfink/internal/auth"
)

const DefaultTenantID = "00000000-0000-4000-8000-000000000001"

func Seed(ctx context.Context, pool *pgxpool.Pool, adminUser, adminPassword string) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO tenants (id, name)
		VALUES ($1, 'Standard')
		ON CONFLICT (id) DO NOTHING`, DefaultTenantID)
	if err != nil {
		return err
	}

	var id string
	err = pool.QueryRow(ctx, `SELECT id FROM users LIMIT 1`).Scan(&id)
	if err == nil {
		return flagDefaultPasswords(ctx, pool)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	adminUser = strings.TrimSpace(adminUser)
	hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	mustChange := auth.WeakPassword(adminUser, adminPassword)
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, tenant_id, username, password_hash, auth_provider, role, must_change_password, can_delete)
		VALUES ($1, $2, $3, $4, 'local', $5, $6, true)`,
		uuid.NewString(), DefaultTenantID, adminUser, string(hash), auth.RoleAdmin, mustChange)
	if err != nil {
		return err
	}
	return flagDefaultPasswords(ctx, pool)
}

func flagDefaultPasswords(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := pool.Query(ctx, `
		SELECT id, password_hash
		FROM users
		WHERE auth_provider = 'local' AND NOT must_change_password`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var weak []string
	for rows.Next() {
		var id, hash string
		if err := rows.Scan(&id, &hash); err != nil {
			return err
		}
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte("admin")) == nil {
			weak = append(weak, id)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range weak {
		if _, err := pool.Exec(ctx, `
			UPDATE users SET must_change_password = true WHERE id = $1`, id); err != nil {
			return err
		}
	}
	return nil
}
