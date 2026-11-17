package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	APITokenPrefix    = "sft_"
	APITokenBytes     = 32
	APITokenMaxActive = 10
	APITokenNameMax   = 80
	APITokenPrefixLen = 12
)

var (
	ErrTokenLimit   = errors.New("token limit")
	ErrTokenName    = errors.New("token name")
	ErrTokenExpiry  = errors.New("token expiry")
	ErrTokenRevoked = errors.New("token revoked")
)

var APITokenExpiryDays = []int{30, 90, 365}

type APIToken struct {
	ID         string
	UserID     string
	Name       string
	Prefix     string
	ExpiresAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
}

func NormalizeTokenName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > APITokenNameMax {
		return "", ErrTokenName
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", ErrTokenName
		}
	}
	return name, nil
}

func NormalizeTokenExpiryDays(days int) (int, error) {
	for _, allowed := range APITokenExpiryDays {
		if days == allowed {
			return days, nil
		}
	}
	return 0, ErrTokenExpiry
}

func (s *Service) CreateAPIToken(ctx context.Context, userID, name string, expiresInDays int) (APIToken, string, error) {
	name, err := NormalizeTokenName(name)
	if err != nil {
		return APIToken{}, "", err
	}
	days, err := NormalizeTokenExpiryDays(expiresInDays)
	if err != nil {
		return APIToken{}, "", err
	}
	var active int
	if err := s.DB.QueryRow(ctx, `
		SELECT count(*) FROM api_tokens
		WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now()`, userID).Scan(&active); err != nil {
		return APIToken{}, "", err
	}
	if active >= APITokenMaxActive {
		return APIToken{}, "", ErrTokenLimit
	}

	raw := make([]byte, APITokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return APIToken{}, "", err
	}
	plain := APITokenPrefix + hex.EncodeToString(raw)
	tok := APIToken{
		ID:        uuid.NewString(),
		UserID:    userID,
		Name:      name,
		Prefix:    tokenPrefix(plain),
		ExpiresAt: time.Now().Add(time.Duration(days) * 24 * time.Hour),
		CreatedAt: time.Now().UTC(),
	}
	err = s.DB.QueryRow(ctx, `
		INSERT INTO api_tokens (id, user_id, token_hash, name, prefix, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, expires_at`,
		tok.ID, tok.UserID, hashToken(plain), tok.Name, tok.Prefix, tok.ExpiresAt, tok.CreatedAt,
	).Scan(&tok.CreatedAt, &tok.ExpiresAt)
	return tok, plain, err
}

func (s *Service) ListAPITokens(ctx context.Context, userID string) ([]APIToken, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT id, user_id, name, prefix, expires_at, last_used_at, revoked_at, created_at
		FROM api_tokens
		WHERE user_id = $1 AND revoked_at IS NULL
		ORDER BY created_at DESC, id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []APIToken
	for rows.Next() {
		var t APIToken
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.Prefix, &t.ExpiresAt, &t.LastUsedAt, &t.RevokedAt, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if out == nil {
		out = []APIToken{}
	}
	return out, rows.Err()
}

func (s *Service) RevokeAPIToken(ctx context.Context, userID, tokenID string) (APIToken, error) {
	var t APIToken
	err := s.DB.QueryRow(ctx, `
		UPDATE api_tokens
		SET revoked_at = now()
		WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL
		RETURNING id, user_id, name, prefix, expires_at, last_used_at, revoked_at, created_at`,
		tokenID, userID).Scan(
		&t.ID, &t.UserID, &t.Name, &t.Prefix, &t.ExpiresAt, &t.LastUsedAt, &t.RevokedAt, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return APIToken{}, ErrNotFound
	}
	return t, err
}

func (s *Service) LookupAPIToken(ctx context.Context, token string) (Principal, error) {
	token = strings.TrimSpace(token)
	if !strings.HasPrefix(token, APITokenPrefix) || len(token) < len(APITokenPrefix)+16 {
		return Principal{}, ErrInvalidCredentials
	}
	var p Principal
	err := s.DB.QueryRow(ctx, `
		UPDATE api_tokens t
		SET last_used_at = now()
		FROM users u
		WHERE t.token_hash = $1
		  AND t.user_id = u.id
		  AND t.revoked_at IS NULL
		  AND t.expires_at > now()
		  AND NOT u.disabled
		RETURNING u.id, u.tenant_id, u.username, u.role, u.can_delete,
		          u.must_change_password, u.disabled, u.auth_provider, t.id`,
		hashToken(token),
	).Scan(&p.UserID, &p.TenantID, &p.Username, &p.Role, &p.CanDelete, &p.MustChangePassword, &p.Disabled, &p.AuthProvider, &p.TokenID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, ErrInvalidCredentials
	}
	return p, err
}

func (s *Service) revokeAPITokens(ctx context.Context, userID string) error {
	_, err := s.DB.Exec(ctx, `
		UPDATE api_tokens SET revoked_at = now()
		WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

func tokenPrefix(plain string) string {
	if len(plain) <= APITokenPrefixLen {
		return plain
	}
	return plain[:APITokenPrefixLen]
}
