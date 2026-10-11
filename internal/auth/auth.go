package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const (
	CookieName        = "sf_session"
	TTL               = 7 * 24 * time.Hour
	MinPasswordLength = 8
	RoleAdmin         = "admin"
	RoleUser          = "user"
	RoleSearcher      = "searcher"
	ProviderLocal     = "local"
	ProviderLDAP      = "ldap"
	UsernameMaxRunes  = 128
)

var (
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrWeakPassword         = errors.New("weak password")
	ErrSamePassword         = errors.New("same password")
	ErrNotFound             = errors.New("not found")
	ErrDisabled             = errors.New("disabled")
	ErrLastAdmin            = errors.New("last admin")
	ErrSelfDisable          = errors.New("self disable")
	ErrSelfPassword         = errors.New("self password")
	ErrInvalidRole          = errors.New("invalid role")
	ErrInvalidUsername      = errors.New("invalid username")
	ErrExternalPassword     = errors.New("external password")
	ErrDirectoryUnavailable = errors.New("directory unavailable")
)

var dummyPasswordHash = mustHash("dummy-password-not-used")

type Principal struct {
	UserID             string
	TenantID           string
	Username           string
	Role               string
	CanDelete          bool
	MustChangePassword bool
	Disabled           bool
	AuthProvider       string
	TokenID            string
}

func (p Principal) Local() bool {
	return p.AuthProvider == "" || p.AuthProvider == ProviderLocal
}

func (p Principal) ViaAPIToken() bool {
	return p.TokenID != ""
}

func (p Principal) IsAdmin() bool {
	return !p.ViaAPIToken() && p.Role == RoleAdmin
}

func (p Principal) IsSearcher() bool {
	return p.Role == RoleSearcher
}

func (p Principal) CanWrite() bool {
	return !p.ViaAPIToken() && !p.IsSearcher()
}

func (p Principal) CanExecuteDeletion() bool {
	return p.CanDelete && p.CanWrite()
}

func NormalizeRole(role string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case RoleAdmin:
		return RoleAdmin, true
	case RoleUser, "":
		return RoleUser, true
	case RoleSearcher:
		return RoleSearcher, true
	default:
		return "", false
	}
}

type Service struct {
	DB        *pgxpool.Pool
	Directory Directory
}

func NormalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func NormalizeProvider(provider string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "", ProviderLocal:
		return ProviderLocal, true
	case ProviderLDAP, "ad":
		return ProviderLDAP, true
	default:
		return "", false
	}
}

func ValidUsername(username string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" || utf8.RuneCountInString(username) > UsernameMaxRunes {
		return "", ErrInvalidUsername
	}
	for _, r := range username {
		if unicode.IsControl(r) || r == '/' || r == '\\' {
			return "", ErrInvalidUsername
		}
	}
	return username, nil
}

func checkPassword(ctx context.Context, dir Directory, provider, username, hash, password string) error {
	if password == "" {
		return ErrInvalidCredentials
	}
	switch provider {
	case "", ProviderLocal:
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
			return ErrInvalidCredentials
		}
		return nil
	case ProviderLDAP:
		if dir == nil {
			return ErrDirectoryUnavailable
		}
		if err := dir.Authenticate(ctx, username, password); err != nil {
			if errors.Is(err, ErrInvalidCredentials) {
				return ErrInvalidCredentials
			}
			return err
		}
		return nil
	default:
		return ErrInvalidCredentials
	}
}

func (s *Service) Login(ctx context.Context, username, password string) (token string, p Principal, err error) {
	username = NormalizeUsername(username)
	var hash string
	err = s.DB.QueryRow(ctx, `
		SELECT id, tenant_id, username, password_hash, role, can_delete, must_change_password, disabled, auth_provider
		FROM users
		WHERE lower(username) = $1`, username).Scan(
		&p.UserID, &p.TenantID, &p.Username, &hash, &p.Role, &p.CanDelete, &p.MustChangePassword, &p.Disabled, &p.AuthProvider)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(password))
		return "", Principal{}, ErrInvalidCredentials
	}
	if err != nil {
		return "", Principal{}, err
	}
	if p.Disabled {
		return "", Principal{}, ErrDisabled
	}
	if err := checkPassword(ctx, s.Directory, p.AuthProvider, p.Username, hash, password); err != nil {
		return "", Principal{}, err
	}
	token, err = s.createSession(ctx, p.UserID)
	return token, p, err
}

func (s *Service) Lookup(ctx context.Context, token string) (Principal, error) {
	var p Principal
	err := s.DB.QueryRow(ctx, `
		SELECT u.id, u.tenant_id, u.username, u.role, u.can_delete, u.must_change_password, u.disabled, u.auth_provider
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now() AND NOT u.disabled`, hashToken(token)).Scan(
		&p.UserID, &p.TenantID, &p.Username, &p.Role, &p.CanDelete, &p.MustChangePassword, &p.Disabled, &p.AuthProvider)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, ErrInvalidCredentials
	}
	return p, err
}

func (s *Service) Logout(ctx context.Context, token string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hashToken(token))
	return err
}

func (s *Service) CreateUser(ctx context.Context, tenantID, username, password, role string, canDelete bool) (Principal, error) {
	username, err := ValidUsername(username)
	if err != nil {
		return Principal{}, err
	}
	if WeakPassword(username, password) {
		return Principal{}, ErrWeakPassword
	}
	role, ok := NormalizeRole(role)
	if !ok {
		return Principal{}, ErrInvalidRole
	}
	if role == RoleSearcher {
		canDelete = false
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Principal{}, err
	}
	p := Principal{
		UserID:             uuid.NewString(),
		TenantID:           tenantID,
		Username:           username,
		Role:               role,
		CanDelete:          canDelete,
		MustChangePassword: true,
		AuthProvider:       ProviderLocal,
	}
	_, err = s.DB.Exec(ctx, `
		INSERT INTO users (id, tenant_id, username, password_hash, auth_provider, role, must_change_password, can_delete)
		VALUES ($1, $2, $3, $4, 'local', $5, true, $6)`, p.UserID, tenantID, username, string(hash), p.Role, p.CanDelete)
	return p, err
}

func (s *Service) ListUsers(ctx context.Context, tenantID string) ([]Principal, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT id, tenant_id, username, role, can_delete, must_change_password, disabled, auth_provider
		FROM users
		WHERE tenant_id = $1
		ORDER BY username`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Principal
	for rows.Next() {
		var p Principal
		if err := rows.Scan(&p.UserID, &p.TenantID, &p.Username, &p.Role, &p.CanDelete, &p.MustChangePassword, &p.Disabled, &p.AuthProvider); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Service) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	var username, hash, provider string
	err := s.DB.QueryRow(ctx, `
		SELECT username, password_hash, auth_provider FROM users WHERE id = $1`, userID).Scan(&username, &hash, &provider)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidCredentials
	}
	if err != nil {
		return err
	}
	if provider != "" && provider != ProviderLocal {
		return ErrExternalPassword
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(oldPassword)) != nil {
		return ErrInvalidCredentials
	}
	if oldPassword == newPassword {
		return ErrSamePassword
	}
	if WeakPassword(username, newPassword) {
		return ErrWeakPassword
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(ctx, `
		UPDATE users SET password_hash = $2, must_change_password = false WHERE id = $1`,
		userID, string(newHash))
	if err != nil {
		return err
	}
	return s.dropSessions(ctx, userID)
}

func (s *Service) GetUser(ctx context.Context, tenantID, userID string) (Principal, error) {
	var p Principal
	err := s.DB.QueryRow(ctx, `
		SELECT id, tenant_id, username, role, can_delete, must_change_password, disabled, auth_provider
		FROM users
		WHERE id = $1 AND tenant_id = $2`, userID, tenantID).Scan(
		&p.UserID, &p.TenantID, &p.Username, &p.Role, &p.CanDelete, &p.MustChangePassword, &p.Disabled, &p.AuthProvider)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, ErrNotFound
	}
	return p, err
}

func (s *Service) SetDisabled(ctx context.Context, tenantID, actorID, userID string, disabled bool) (Principal, error) {
	return s.UpdateUser(ctx, tenantID, actorID, userID, &disabled, nil, nil)
}

func (s *Service) UpdateUser(ctx context.Context, tenantID, actorID, userID string, disabled *bool, role *string, canDelete *bool) (Principal, error) {
	p, err := s.GetUser(ctx, tenantID, userID)
	if err != nil {
		return Principal{}, err
	}
	if disabled == nil && role == nil && canDelete == nil {
		return p, nil
	}

	nextDisabled := p.Disabled
	if disabled != nil {
		if actorID == userID && *disabled {
			return Principal{}, ErrSelfDisable
		}
		nextDisabled = *disabled
	}

	nextRole := p.Role
	if role != nil {
		normalized, ok := NormalizeRole(*role)
		if !ok {
			return Principal{}, ErrInvalidRole
		}
		nextRole = normalized
	}

	losingAdmin := p.IsAdmin() && !p.Disabled && (nextDisabled || nextRole != RoleAdmin)
	if losingAdmin {
		n, err := s.enabledAdminCount(ctx, tenantID, userID)
		if err != nil {
			return Principal{}, err
		}
		if n == 0 {
			return Principal{}, ErrLastAdmin
		}
	}

	nextCanDelete := p.CanDelete
	if canDelete != nil {
		nextCanDelete = *canDelete
	}
	if nextRole == RoleSearcher {
		nextCanDelete = false
	}

	if nextDisabled == p.Disabled && nextRole == p.Role && nextCanDelete == p.CanDelete {
		return p, nil
	}

	_, err = s.DB.Exec(ctx, `
		UPDATE users SET disabled = $3, role = $4, can_delete = $5
		WHERE id = $1 AND tenant_id = $2`,
		userID, tenantID, nextDisabled, nextRole, nextCanDelete)
	if err != nil {
		return Principal{}, err
	}
	wasDisabled := p.Disabled
	p.Disabled = nextDisabled
	p.Role = nextRole
	p.CanDelete = nextCanDelete
	if nextDisabled && !wasDisabled {
		if err := s.dropSessions(ctx, userID); err != nil {
			return Principal{}, err
		}
	}
	return p, nil
}

func (s *Service) GetUserByUsername(ctx context.Context, tenantID, username string) (Principal, error) {
	var p Principal
	err := s.DB.QueryRow(ctx, `
		SELECT id, tenant_id, username, role, can_delete, must_change_password, disabled, auth_provider
		FROM users
		WHERE tenant_id = $1 AND lower(username) = $2`, tenantID, NormalizeUsername(username)).Scan(
		&p.UserID, &p.TenantID, &p.Username, &p.Role, &p.CanDelete, &p.MustChangePassword, &p.Disabled, &p.AuthProvider)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, ErrNotFound
	}
	return p, err
}

func (s *Service) ProvisionUser(ctx context.Context, tenantID, username, role string, canDelete bool, externalID string) (Principal, error) {
	username, err := ValidUsername(username)
	if err != nil {
		return Principal{}, err
	}
	role, ok := NormalizeRole(role)
	if !ok {
		return Principal{}, ErrInvalidRole
	}
	if role == RoleSearcher {
		canDelete = false
	}
	externalID = strings.TrimSpace(externalID)
	p := Principal{
		UserID:       uuid.NewString(),
		TenantID:     tenantID,
		Username:     username,
		Role:         role,
		CanDelete:    canDelete,
		AuthProvider: ProviderLDAP,
	}
	_, err = s.DB.Exec(ctx, `
		INSERT INTO users (id, tenant_id, username, password_hash, auth_provider, external_id, role, must_change_password, can_delete)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7, false, $8)`,
		p.UserID, p.TenantID, p.Username, dummyPasswordHash, ProviderLDAP, externalID, p.Role, p.CanDelete)
	return p, err
}

func (s *Service) ResetPassword(ctx context.Context, tenantID, actorID, userID, password string) (Principal, error) {
	if actorID == userID {
		return Principal{}, ErrSelfPassword
	}
	p, err := s.GetUser(ctx, tenantID, userID)
	if err != nil {
		return Principal{}, err
	}
	if !p.Local() {
		return Principal{}, ErrExternalPassword
	}
	if WeakPassword(p.Username, password) {
		return Principal{}, ErrWeakPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Principal{}, err
	}
	_, err = s.DB.Exec(ctx, `
		UPDATE users SET password_hash = $3, must_change_password = true
		WHERE id = $1 AND tenant_id = $2`,
		userID, tenantID, string(hash))
	if err != nil {
		return Principal{}, err
	}
	p.MustChangePassword = true
	if err := s.dropSessions(ctx, userID); err != nil {
		return Principal{}, err
	}
	return p, nil
}

func (s *Service) enabledAdminCount(ctx context.Context, tenantID, exceptUserID string) (int, error) {
	var n int
	err := s.DB.QueryRow(ctx, `
		SELECT count(*) FROM users
		WHERE tenant_id = $1 AND role = $2 AND NOT disabled AND id <> $3`,
		tenantID, RoleAdmin, exceptUserID).Scan(&n)
	return n, err
}

func (s *Service) dropSessions(ctx context.Context, userID string) error {
	if _, err := s.DB.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID); err != nil {
		return err
	}
	return s.revokeAPITokens(ctx, userID)
}

func (s *Service) CreateSession(ctx context.Context, userID string) (string, error) {
	return s.createSession(ctx, userID)
}

func (s *Service) createSession(ctx context.Context, userID string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	_, err := s.DB.Exec(ctx, `
		INSERT INTO sessions (token_hash, user_id, expires_at)
		VALUES ($1, $2, $3)`, hashToken(token), userID, time.Now().Add(TTL))
	return token, err
}

func WeakPassword(username, password string) bool {
	if len(password) < MinPasswordLength {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(password), strings.TrimSpace(username)) {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(password)) {
	case "admin", "password", "passwort", "schmutzfink", "12345678", "change_me", "changeme":
		return true
	}
	return false
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func mustHash(password string) string {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return string(h)
}
