package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"schmutzfink/internal/audit"
	"schmutzfink/internal/auth"
)

type loginBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type userBody struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	Role         string `json:"role"`
	CanDelete    bool   `json:"can_delete"`
	AuthProvider string `json:"auth_provider"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r.RemoteAddr)
	var body loginBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	if body.Username == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "Benutzername und Passwort nötig")
		return
	}
	if s.loginLimit != nil && !s.loginLimit.allow(ip, body.Username) {
		s.writeAudit(r, auth.Principal{Username: body.Username}, audit.LoginLocked, body.Username, nil)
		writeError(w, http.StatusTooManyRequests, "Zu viele Anmeldeversuche. Bitte später erneut versuchen.")
		return
	}
	token, p, err := s.Auth.Login(r.Context(), body.Username, body.Password)
	if errors.Is(err, auth.ErrDisabled) {
		s.writeAudit(r, auth.Principal{Username: body.Username}, audit.LoginDisabled, body.Username, nil)
		writeError(w, http.StatusForbidden, "Konto ist deaktiviert")
		return
	}
	if errors.Is(err, auth.ErrDirectoryUnavailable) {
		writeError(w, http.StatusServiceUnavailable, "AD-Anmeldung ist nicht eingerichtet")
		return
	}
	if errors.Is(err, auth.ErrInvalidCredentials) {
		if s.loginLimit != nil {
			s.loginLimit.fail(ip, body.Username)
		}
		s.writeAudit(r, auth.Principal{Username: body.Username}, audit.LoginFailed, body.Username, nil)
		writeError(w, http.StatusUnauthorized, "Benutzername oder Passwort falsch")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Anmeldung fehlgeschlagen")
		return
	}
	if s.loginLimit != nil {
		s.loginLimit.ok(ip, body.Username)
	}
	s.setSessionCookie(w, r, token, int(auth.TTL.Seconds()))
	s.writeAudit(r, p, audit.Login, p.UserID, nil)
	writeJSON(w, http.StatusOK, map[string]any{"user": presentUser(p)})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	var actor auth.Principal
	if c, err := r.Cookie(auth.CookieName); err == nil {
		if p, err := s.Auth.Lookup(r.Context(), c.Value); err == nil {
			actor = p
		}
		_ = s.Auth.Logout(r.Context(), c.Value)
	}
	if actor.UserID != "" {
		s.writeAudit(r, actor, audit.Logout, actor.UserID, nil)
	}
	s.setSessionCookie(w, r, "", -1)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"user": presentUser(principal(r))})
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !principal(r).IsAdmin() {
			writeError(w, http.StatusForbidden, "Keine Berechtigung")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireWriter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !principal(r).CanWrite() {
			writeError(w, http.StatusForbidden, "Keine Berechtigung")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireDeletionOfficer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !principal(r).CanExecuteDeletion() {
			writeError(w, http.StatusForbidden, "Keine Berechtigung")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.Auth.ListUsers(r.Context(), principal(r).TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Benutzer konnten nicht geladen werden")
		return
	}
	out := make([]map[string]any, 0, len(users))
	for _, u := range users {
		out = append(out, presentUser(u))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "ldap": s.Auth != nil && s.Auth.Directory != nil})
}

type passwordBody struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	var body passwordBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	err := s.Auth.ChangePassword(r.Context(), principal(r).UserID, body.OldPassword, body.NewPassword)
	if errors.Is(err, auth.ErrExternalPassword) {
		writeError(w, http.StatusBadRequest, "Passwort liegt im Active Directory")
		return
	}
	if errors.Is(err, auth.ErrInvalidCredentials) {
		writeError(w, http.StatusBadRequest, "Aktuelles Passwort ist falsch")
		return
	}
	if errors.Is(err, auth.ErrSamePassword) {
		writeError(w, http.StatusBadRequest, "Das neue Passwort muss sich vom bisherigen unterscheiden")
		return
	}
	if errors.Is(err, auth.ErrWeakPassword) {
		writeError(w, http.StatusBadRequest, "Passwort zu schwach (mind. 8 Zeichen, nicht Benutzername, kein Standardwort)")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Passwort konnte nicht geändert werden")
		return
	}
	token, err := s.Auth.CreateSession(r.Context(), principal(r).UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Sitzung konnte nicht erneuert werden")
		return
	}
	s.setSessionCookie(w, r, token, int(auth.TTL.Seconds()))
	p := principal(r)
	p.MustChangePassword = false
	s.writeAudit(r, p, audit.PasswordChange, p.UserID, nil)
	writeJSON(w, http.StatusOK, map[string]any{"user": presentUser(p)})
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var body userBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	provider, ok := auth.NormalizeProvider(body.AuthProvider)
	if !ok {
		writeError(w, http.StatusBadRequest, "Ungültige Anmeldeart")
		return
	}
	var (
		p   auth.Principal
		err error
	)
	if provider == auth.ProviderLDAP {
		p, err = s.Auth.ProvisionUser(r.Context(), principal(r).TenantID, body.Username, body.Role, body.CanDelete, "")
	} else {
		if body.Username == "" || auth.WeakPassword(body.Username, body.Password) {
			writeError(w, http.StatusBadRequest, "Benutzername und ein starkes Passwort (mind. 8 Zeichen, nicht Benutzername) nötig")
			return
		}
		p, err = s.Auth.CreateUser(r.Context(), principal(r).TenantID, body.Username, body.Password, body.Role, body.CanDelete)
	}
	if errors.Is(err, auth.ErrInvalidUsername) {
		writeError(w, http.StatusBadRequest, "Ungültiger Benutzername")
		return
	}
	if isUniqueViolation(err) {
		writeError(w, http.StatusConflict, "Benutzername ist bereits vergeben")
		return
	}
	if errors.Is(err, auth.ErrInvalidRole) {
		writeError(w, http.StatusBadRequest, "Ungültige Rolle")
		return
	}
	if errors.Is(err, auth.ErrWeakPassword) {
		writeError(w, http.StatusBadRequest, "Passwort zu schwach (mind. 8 Zeichen, nicht Benutzername, kein Standardwort)")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Benutzer konnte nicht angelegt werden")
		return
	}
	s.writeAudit(r, principal(r), audit.UserCreate, p.Username, map[string]any{
		"user_id":       p.UserID,
		"role":          p.Role,
		"can_delete":    p.CanDelete,
		"auth_provider": p.AuthProvider,
	})
	writeJSON(w, http.StatusCreated, map[string]any{"user": presentUser(p)})
}

type patchUserBody struct {
	Disabled  *bool   `json:"disabled"`
	Role      *string `json:"role"`
	CanDelete *bool   `json:"can_delete"`
}

func (s *Server) patchUser(w http.ResponseWriter, r *http.Request) {
	id, ok := userIDParam(w, r)
	if !ok {
		return
	}
	var body patchUserBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || (body.Disabled == nil && body.Role == nil && body.CanDelete == nil) {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	actor := principal(r)
	p, err := s.Auth.UpdateUser(r.Context(), actor.TenantID, actor.UserID, id, body.Disabled, body.Role, body.CanDelete)
	if errors.Is(err, auth.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Benutzer nicht gefunden")
		return
	}
	if errors.Is(err, auth.ErrSelfDisable) {
		writeError(w, http.StatusBadRequest, "Das eigene Konto kann nicht deaktiviert werden")
		return
	}
	if errors.Is(err, auth.ErrLastAdmin) {
		writeError(w, http.StatusBadRequest, "Der letzte Admin kann nicht deaktiviert oder herabgestuft werden")
		return
	}
	if errors.Is(err, auth.ErrInvalidRole) {
		writeError(w, http.StatusBadRequest, "Ungültige Rolle")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Benutzer konnte nicht geändert werden")
		return
	}
	if body.Disabled != nil {
		action := audit.UserEnable
		if p.Disabled {
			action = audit.UserDisable
		}
		s.writeAudit(r, actor, action, p.Username, map[string]any{"user_id": p.UserID})
	}
	if body.Role != nil || body.CanDelete != nil {
		s.writeAudit(r, actor, audit.UserPatch, p.Username, map[string]any{
			"user_id":    p.UserID,
			"role":       p.Role,
			"can_delete": p.CanDelete,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": presentUser(p)})
}

type resetPasswordBody struct {
	Password string `json:"password"`
}

func (s *Server) resetUserPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := userIDParam(w, r)
	if !ok {
		return
	}
	var body resetPasswordBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	actor := principal(r)
	p, err := s.Auth.ResetPassword(r.Context(), actor.TenantID, actor.UserID, id, body.Password)
	if errors.Is(err, auth.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Benutzer nicht gefunden")
		return
	}
	if errors.Is(err, auth.ErrSelfPassword) {
		writeError(w, http.StatusBadRequest, "Eigenes Passwort über das Konto-Menü ändern")
		return
	}
	if errors.Is(err, auth.ErrExternalPassword) {
		writeError(w, http.StatusBadRequest, "Passwort liegt im Active Directory")
		return
	}
	if errors.Is(err, auth.ErrWeakPassword) {
		writeError(w, http.StatusBadRequest, "Passwort zu schwach (mind. 8 Zeichen, nicht Benutzername, kein Standardwort)")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Passwort konnte nicht gesetzt werden")
		return
	}
	s.writeAudit(r, actor, audit.PasswordReset, p.Username, map[string]any{"user_id": p.UserID})
	writeJSON(w, http.StatusOK, map[string]any{"user": presentUser(p)})
}

func userIDParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return "", false
	}
	return id, true
}

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   cookieSecure(s.CookieSecure, r),
		SameSite: http.SameSiteLaxMode,
	})
}

func cookieSecure(mode string, r *http.Request) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "1", "true", "yes", "always":
		return true
	case "0", "false", "no", "never":
		return false
	}
	if r != nil && r.TLS != nil {
		return true
	}
	if r == nil || !proxyTrusted(r) {
		return false
	}
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = r.Header.Get("X-Forwarded-Scheme")
	}
	if i := strings.IndexByte(proto, ','); i >= 0 {
		proto = proto[:i]
	}
	return strings.EqualFold(strings.TrimSpace(proto), "https")
}

func presentUser(p auth.Principal) map[string]any {
	role := p.Role
	if role == "" {
		role = auth.RoleUser
	}
	out := map[string]any{
		"id":                   p.UserID,
		"username":             p.Username,
		"role":                 role,
		"can_delete":           p.CanExecuteDeletion(),
		"must_change_password": p.MustChangePassword,
		"disabled":             p.Disabled,
	}
	if p.ViaAPIToken() {
		out["api_token"] = true
	}
	provider := p.AuthProvider
	if provider == "" {
		provider = auth.ProviderLocal
	}
	out["auth_provider"] = provider
	return out
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
