package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"schmutzfink/internal/audit"
	"schmutzfink/internal/auth"
	"schmutzfink/internal/db"
)

const minProvisionTokenLen = 16

type provisionUserBody struct {
	Username   string `json:"username"`
	Role       string `json:"role"`
	CanDelete  bool   `json:"can_delete"`
	ExternalID string `json:"external_id"`
}

func (s *Server) provisionEnabled() bool {
	tok := strings.TrimSpace(s.ProvisionToken)
	return len(tok) >= minProvisionTokenLen
}

func (s *Server) provisionTenantID() string {
	if id := strings.TrimSpace(s.ProvisionTenant); id != "" {
		return id
	}
	return db.DefaultTenantID
}

func (s *Server) provisionActor() auth.Principal {
	return auth.Principal{Username: "provision", TenantID: s.provisionTenantID()}
}

func (s *Server) requireProvision(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.provisionEnabled() {
			writeError(w, http.StatusNotFound, "Nicht gefunden")
			return
		}
		raw, ok := bearerToken(r)
		if !ok || !tokenEqual(raw, strings.TrimSpace(s.ProvisionToken)) {
			writeError(w, http.StatusUnauthorized, "Nicht angemeldet")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func tokenEqual(got, want string) bool {
	if got == "" || want == "" || len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func (s *Server) provisionCreateUser(w http.ResponseWriter, r *http.Request) {
	var body provisionUserBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	if _, err := auth.ValidUsername(body.Username); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültiger Benutzername")
		return
	}
	if s.Auth == nil {
		writeError(w, http.StatusInternalServerError, "Benutzer konnte nicht angelegt werden")
		return
	}
	p, err := s.Auth.ProvisionUser(r.Context(), s.provisionTenantID(), body.Username, body.Role, body.CanDelete, body.ExternalID)
	if errors.Is(err, auth.ErrInvalidUsername) {
		writeError(w, http.StatusBadRequest, "Ungültiger Benutzername")
		return
	}
	if errors.Is(err, auth.ErrInvalidRole) {
		writeError(w, http.StatusBadRequest, "Ungültige Rolle")
		return
	}
	if isUniqueViolation(err) {
		writeError(w, http.StatusConflict, "Benutzername ist bereits vergeben")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Benutzer konnte nicht angelegt werden")
		return
	}
	s.writeAudit(r, s.provisionActor(), audit.UserCreate, p.Username, map[string]any{
		"user_id":       p.UserID,
		"role":          p.Role,
		"can_delete":    p.CanDelete,
		"auth_provider": p.AuthProvider,
		"via":           "provision",
	})
	writeJSON(w, http.StatusCreated, map[string]any{"user": presentUser(p)})
}

func (s *Server) provisionGetUser(w http.ResponseWriter, r *http.Request) {
	p, ok := s.lookupProvisionUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": presentUser(p)})
}

func (s *Server) provisionPatchUser(w http.ResponseWriter, r *http.Request) {
	p, ok := s.lookupProvisionUser(w, r)
	if !ok {
		return
	}
	var body patchUserBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || (body.Disabled == nil && body.Role == nil && body.CanDelete == nil) {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	actor := s.provisionActor()
	next, err := s.Auth.UpdateUser(r.Context(), actor.TenantID, "", p.UserID, body.Disabled, body.Role, body.CanDelete)
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
		if next.Disabled {
			action = audit.UserDisable
		}
		s.writeAudit(r, actor, action, next.Username, map[string]any{"user_id": next.UserID, "via": "provision"})
	}
	if body.Role != nil || body.CanDelete != nil {
		s.writeAudit(r, actor, audit.UserPatch, next.Username, map[string]any{
			"user_id":    next.UserID,
			"role":       next.Role,
			"can_delete": next.CanDelete,
			"via":        "provision",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": presentUser(next)})
}

func (s *Server) lookupProvisionUser(w http.ResponseWriter, r *http.Request) (auth.Principal, bool) {
	username := strings.TrimSpace(chi.URLParam(r, "username"))
	if username == "" || s.Auth == nil {
		writeError(w, http.StatusNotFound, "Benutzer nicht gefunden")
		return auth.Principal{}, false
	}
	p, err := s.Auth.GetUserByUsername(r.Context(), s.provisionTenantID(), username)
	if errors.Is(err, auth.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Benutzer nicht gefunden")
		return auth.Principal{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Benutzer konnte nicht geladen werden")
		return auth.Principal{}, false
	}
	return p, true
}
