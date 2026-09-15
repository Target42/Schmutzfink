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

type caseBody struct {
	Title    string `json:"title"`
	Note     string `json:"note"`
	Kind     string `json:"kind"`
	RecordID string `json:"record_id"`
}

type assignCaseBody struct {
	RecordID string `json:"record_id"`
}

func (s *Server) listCases(w http.ResponseWriter, r *http.Request) {
	items, err := s.Records.ListCases(r.Context(), principal(r).TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Vorgänge konnten nicht geladen werden")
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, c := range items {
		out = append(out, presentCase(c, false))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) createCase(w http.ResponseWriter, r *http.Request) {
	var body caseBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	p := principal(r)
	c, err := s.Records.CreateCase(r.Context(), p.TenantID, p.UserID, body.Title, body.Note, body.Kind, strings.TrimSpace(body.RecordID))
	if errors.Is(err, records.ErrCaseTitle) {
		writeError(w, http.StatusBadRequest, "Bitte einen Titel (Aktenzeichen oder Kurzname) angeben")
		return
	}
	if errors.Is(err, records.ErrCaseKind) {
		writeError(w, http.StatusBadRequest, "Bitte Zivil- oder Strafrecht wählen")
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Foto nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Vorgang konnte nicht angelegt werden")
		return
	}
	s.writeAudit(r, p, audit.CaseCreate, c.ID, map[string]any{"kind": c.Kind, "record_id": body.RecordID})
	writeJSON(w, http.StatusCreated, presentCase(c, true))
}

func (s *Server) getCase(w http.ResponseWriter, r *http.Request) {
	c, err := s.Records.GetCase(r.Context(), principal(r).TenantID, chi.URLParam(r, "id"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Vorgang nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Vorgang konnte nicht geladen werden")
		return
	}
	writeJSON(w, http.StatusOK, presentCase(c, true))
}

func (s *Server) patchCase(w http.ResponseWriter, r *http.Request) {
	var body caseBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	p := principal(r)
	c, err := s.Records.UpdateCase(r.Context(), p.TenantID, chi.URLParam(r, "id"), body.Title, body.Note, body.Kind)
	if errors.Is(err, records.ErrCaseTitle) {
		writeError(w, http.StatusBadRequest, "Bitte einen Titel (Aktenzeichen oder Kurzname) angeben")
		return
	}
	if errors.Is(err, records.ErrCaseKind) {
		writeError(w, http.StatusBadRequest, "Bitte Zivil- oder Strafrecht wählen")
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Vorgang nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Vorgang konnte nicht gespeichert werden")
		return
	}
	s.writeAudit(r, p, audit.CasePatch, c.ID, map[string]any{"kind": c.Kind})
	writeJSON(w, http.StatusOK, presentCase(c, true))
}

func (s *Server) closeCase(w http.ResponseWriter, r *http.Request) {
	s.setCaseClosed(w, r, true)
}

func (s *Server) reopenCase(w http.ResponseWriter, r *http.Request) {
	s.setCaseClosed(w, r, false)
}

func (s *Server) setCaseClosed(w http.ResponseWriter, r *http.Request, closed bool) {
	p := principal(r)
	id := chi.URLParam(r, "id")
	c, err := s.Records.SetCaseClosed(r.Context(), p.TenantID, id, p.UserID, closed)
	if errors.Is(err, records.ErrCaseRedacted) {
		writeError(w, http.StatusConflict, "Nach der Standortentfernung bleibt der Vorgang geschlossen")
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Vorgang nicht gefunden")
		return
	}
	if err != nil {
		if closed {
			writeError(w, http.StatusInternalServerError, "Vorgang konnte nicht geschlossen werden")
		} else {
			writeError(w, http.StatusInternalServerError, "Vorgang konnte nicht geöffnet werden")
		}
		return
	}
	action := audit.CaseReopen
	if closed {
		action = audit.CaseClose
	}
	s.writeAudit(r, p, action, id, nil)
	writeJSON(w, http.StatusOK, presentCase(c, true))
}

func (s *Server) deleteCase(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	id := chi.URLParam(r, "id")
	err := s.Records.DeleteCase(r.Context(), p.TenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Vorgang nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Vorgang konnte nicht gelöscht werden")
		return
	}
	s.writeAudit(r, p, audit.CaseDelete, id, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) assignCaseRecord(w http.ResponseWriter, r *http.Request) {
	var body assignCaseBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.RecordID) == "" {
		writeError(w, http.StatusBadRequest, "Foto fehlt")
		return
	}
	p := principal(r)
	caseID := chi.URLParam(r, "id")
	photo, err := s.Records.AssignPhotoCase(r.Context(), p.TenantID, caseID, strings.TrimSpace(body.RecordID))
	if errors.Is(err, records.ErrUnknownCase) || errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Vorgang oder Foto nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Foto konnte nicht zugeordnet werden")
		return
	}
	s.writeAudit(r, p, audit.CaseAssign, caseID, map[string]any{"record_id": body.RecordID})
	writeJSON(w, http.StatusOK, presentPhoto(photo))
}

func (s *Server) unlinkCaseRecord(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	photo, err := s.Records.UnlinkPhotoCase(r.Context(), p.TenantID, chi.URLParam(r, "id"), chi.URLParam(r, "rid"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Foto nicht gefunden oder ohne Vorgang")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Foto konnte nicht gelöst werden")
		return
	}
	s.writeAudit(r, p, audit.CaseUnlink, chi.URLParam(r, "id"), map[string]any{"record_id": photo.ID})
	writeJSON(w, http.StatusOK, presentPhoto(photo))
}

func presentCase(c records.Case, withPhotos bool) map[string]any {
	out := map[string]any{
		"id":              c.ID,
		"title":           c.Title,
		"note":            c.Note,
		"kind":            c.Kind,
		"retention_years": records.RetentionYears(c.Kind),
		"created_at":      c.CreatedAt,
		"updated_at":      c.UpdatedAt,
		"closed_at":       c.ClosedAt,
		"closed_by":       c.ClosedBy,
		"closed_by_name":  c.ClosedByName,
		"photo_count":     c.PhotoCount,
		"redacted_count":  c.RedactedCount,
		"first_at":        c.FirstAt,
		"last_at":         c.LastAt,
		"due_at":          c.DueAt,
		"thumb_url":       "",
	}
	if c.ThumbRecordID != "" {
		out["thumb_url"] = "/api/records/" + c.ThumbRecordID + "/thumb"
	}
	if withPhotos {
		out["photos"] = presentRecords(c.Photos)
	}
	return out
}
