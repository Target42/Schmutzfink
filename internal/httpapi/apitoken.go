package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"schmutzfink/internal/audit"
	"schmutzfink/internal/auth"
)

type createTokenBody struct {
	Name          string `json:"name"`
	ExpiresInDays int    `json:"expires_in_days"`
}

func (s *Server) listTokens(w http.ResponseWriter, r *http.Request) {
	if principal(r).ViaAPIToken() {
		writeError(w, http.StatusForbidden, "API-Token darf Token nicht verwalten")
		return
	}
	items, err := s.Auth.ListAPITokens(r.Context(), principal(r).UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Token konnten nicht geladen werden")
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, t := range items {
		out = append(out, presentAPIToken(t))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) createToken(w http.ResponseWriter, r *http.Request) {
	if principal(r).ViaAPIToken() {
		writeError(w, http.StatusForbidden, "API-Token darf Token nicht verwalten")
		return
	}
	var body createTokenBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	if body.ExpiresInDays == 0 {
		body.ExpiresInDays = 90
	}
	tok, plain, err := s.Auth.CreateAPIToken(r.Context(), principal(r).UserID, body.Name, body.ExpiresInDays)
	if errors.Is(err, auth.ErrTokenName) {
		writeError(w, http.StatusBadRequest, "Name nötig, höchstens 80 Zeichen")
		return
	}
	if errors.Is(err, auth.ErrTokenExpiry) {
		writeError(w, http.StatusBadRequest, "Laufzeit muss 30, 90 oder 365 Tage sein")
		return
	}
	if errors.Is(err, auth.ErrTokenLimit) {
		writeError(w, http.StatusConflict, "Höchstens 10 aktive Token")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Token konnte nicht angelegt werden")
		return
	}
	p := principal(r)
	s.writeAudit(r, p, audit.TokenCreate, tok.ID, map[string]any{
		"name":            tok.Name,
		"prefix":          tok.Prefix,
		"expires_in_days": body.ExpiresInDays,
	})
	out := presentAPIToken(tok)
	out["token"] = plain
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) revokeToken(w http.ResponseWriter, r *http.Request) {
	if principal(r).ViaAPIToken() {
		writeError(w, http.StatusForbidden, "API-Token darf Token nicht verwalten")
		return
	}
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	tok, err := s.Auth.RevokeAPIToken(r.Context(), principal(r).UserID, id)
	if errors.Is(err, auth.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Token nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Token konnte nicht widerrufen werden")
		return
	}
	s.writeAudit(r, principal(r), audit.TokenRevoke, tok.ID, map[string]any{
		"name":   tok.Name,
		"prefix": tok.Prefix,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func bearerToken(r *http.Request) (string, bool) {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if h == "" {
		return "", false
	}
	const prefix = "Bearer "
	if len(h) < len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", true
	}
	token := strings.TrimSpace(h[len(prefix):])
	return token, true
}

func tokenPathAllowed(method, path string) bool {
	path = strings.TrimSuffix(path, "/")
	if method == http.MethodGet {
		switch path {
		case "/api/auth/me", "/api/records", "/api/records/map", "/api/records/export",
			"/api/records/summary", "/api/embeddings", "/api/fields", "/api/geocode", "/api/motifs", "/api/cases":
			return true
		}
		if id, ok := strings.CutPrefix(path, "/api/records/"); ok {
			return isTokenUUID(id)
		}
		if rest, ok := strings.CutPrefix(path, "/api/motifs/"); ok {
			parts := strings.Split(rest, "/")
			if len(parts) == 1 {
				return isTokenUUID(parts[0])
			}
			return len(parts) == 2 && isTokenUUID(parts[0]) && parts[1] == "suggestions"
		}
		if rest, ok := strings.CutPrefix(path, "/api/cases/"); ok {
			return isTokenUUID(rest)
		}
		if rest, ok := strings.CutPrefix(path, "/api/sightings/"); ok {
			parts := strings.Split(rest, "/")
			return len(parts) == 2 && isTokenUUID(parts[0]) && parts[1] == "similar"
		}
	}
	if method == http.MethodPost {
		return path == "/api/records/export" || path == "/api/records/summary" || path == "/api/search/image"
	}
	return false
}

func isTokenUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

func presentAPIToken(t auth.APIToken) map[string]any {
	return map[string]any{
		"id":           t.ID,
		"name":         t.Name,
		"prefix":       t.Prefix,
		"expires_at":   t.ExpiresAt,
		"last_used_at": t.LastUsedAt,
		"created_at":   t.CreatedAt,
		"expired":      !t.ExpiresAt.After(time.Now()),
	}
}
