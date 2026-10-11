package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"schmutzfink/internal/audit"
	"schmutzfink/internal/records"
)

type motifBody struct {
	Title      string `json:"title"`
	Note       string `json:"note"`
	SightingID string `json:"sighting_id"`
}

type assignBody struct {
	SightingID string `json:"sighting_id"`
}

func (s *Server) listMotifs(w http.ResponseWriter, r *http.Request) {
	items, err := s.Records.ListMotifs(r.Context(), principal(r).TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Motive konnten nicht geladen werden")
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, m := range items {
		out = append(out, presentMotif(m, false))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) createMotif(w http.ResponseWriter, r *http.Request) {
	var body motifBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	p := principal(r)
	m, err := s.Records.CreateMotif(r.Context(), p.TenantID, p.UserID, strings.TrimSpace(body.SightingID), body.Title, body.Note)
	if errors.Is(err, records.ErrMotifSightingRequired) {
		writeError(w, http.StatusBadRequest, "Bitte eine Sichtung zum Motiv legen")
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Sichtung nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Motiv konnte nicht angelegt werden")
		return
	}
	s.writeAudit(r, p, audit.MotifCreate, m.ID, map[string]any{"sighting_id": body.SightingID})
	writeJSON(w, http.StatusCreated, presentMotif(m, true))
}

func (s *Server) getMotif(w http.ResponseWriter, r *http.Request) {
	m, err := s.Records.GetMotif(r.Context(), principal(r).TenantID, chi.URLParam(r, "id"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Motiv nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Motiv konnte nicht geladen werden")
		return
	}
	writeJSON(w, http.StatusOK, presentMotif(m, true))
}

func (s *Server) patchMotif(w http.ResponseWriter, r *http.Request) {
	var body motifBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	p := principal(r)
	m, err := s.Records.UpdateMotif(r.Context(), p.TenantID, chi.URLParam(r, "id"), body.Title, body.Note)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Motiv nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Motiv konnte nicht gespeichert werden")
		return
	}
	s.writeAudit(r, p, audit.MotifPatch, m.ID, nil)
	writeJSON(w, http.StatusOK, presentMotif(m, true))
}

func (s *Server) deleteMotif(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	id := chi.URLParam(r, "id")
	err := s.Records.DeleteMotif(r.Context(), p.TenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Motiv nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Motiv konnte nicht gelöscht werden")
		return
	}
	s.writeAudit(r, p, audit.MotifDelete, id, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) assignMotifSighting(w http.ResponseWriter, r *http.Request) {
	var body assignBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.SightingID) == "" {
		writeError(w, http.StatusBadRequest, "Sichtung fehlt")
		return
	}
	p := principal(r)
	motifID := chi.URLParam(r, "id")
	m, err := s.Records.AssignSighting(r.Context(), p.TenantID, motifID, strings.TrimSpace(body.SightingID))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Motiv oder Sichtung nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Sichtung konnte nicht zugeordnet werden")
		return
	}
	s.writeAudit(r, p, audit.MotifAssign, motifID, map[string]any{"sighting_id": body.SightingID})
	writeJSON(w, http.StatusOK, presentMotif(m, true))
}

func (s *Server) unlinkMotifSighting(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	motifID := chi.URLParam(r, "id")
	sightingID := chi.URLParam(r, "sid")
	m, err := s.Records.UnlinkSighting(r.Context(), p.TenantID, motifID, sightingID)
	if errors.Is(err, pgx.ErrNoRows) {
		s.writeAudit(r, p, audit.MotifUnlink, motifID, map[string]any{"sighting_id": sightingID, "deleted": true})
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Sichtung konnte nicht gelöst werden")
		return
	}
	s.writeAudit(r, p, audit.MotifUnlink, motifID, map[string]any{"sighting_id": sightingID})
	writeJSON(w, http.StatusOK, presentMotif(m, true))
}

func (s *Server) motifSuggestions(w http.ResponseWriter, r *http.Request) {
	minScore, err := queryMinScore(r, records.DefaultMinScore)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	items, err := s.Records.MotifSuggestions(r.Context(), principal(r).TenantID, chi.URLParam(r, "id"), 12, minScore)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Motiv nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Vorschläge konnten nicht geladen werden")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": presentRecords(items)})
}

func presentMotif(m records.Motif, withSightings bool) map[string]any {
	out := map[string]any{
		"id":             m.ID,
		"title":          m.Title,
		"note":           m.Note,
		"created_at":     m.CreatedAt,
		"updated_at":     m.UpdatedAt,
		"sighting_count": m.SightingCount,
		"first_at":       m.FirstAt,
		"last_at":        m.LastAt,
		"thumb_url":      "",
	}
	if m.ThumbSightingID != "" {
		out["thumb_url"] = "/api/sightings/" + m.ThumbSightingID + "/thumb"
	}
	if withSightings {
		out["sightings"] = presentRecords(m.Sightings)
	}
	return out
}
