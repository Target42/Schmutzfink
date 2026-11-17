package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"schmutzfink/internal/audit"
	"schmutzfink/internal/auth"
	"schmutzfink/internal/records"
)

type deletionRequestBody struct {
	Reason string `json:"reason"`
}

func (s *Server) requestDeletion(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	id := chi.URLParam(r, "id")
	var body deletionRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	photo, err := s.Records.RequestDeletion(r.Context(), p.TenantID, id, p.UserID, body.Reason)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Datensatz nicht gefunden")
		return
	}
	if errors.Is(err, records.ErrDeletionPending) {
		writeError(w, http.StatusConflict, "Löschung wurde bereits beantragt")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Löschantrag konnte nicht gespeichert werden")
		return
	}
	reason := body.Reason
	if photo.Deletion != nil {
		reason = photo.Deletion.Reason
	}
	s.writeAudit(r, p, audit.DeletionRequest, id, map[string]any{"reason": reason})
	writeJSON(w, http.StatusOK, presentPhoto(photo))
}

func (s *Server) cancelDeletion(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	id := chi.URLParam(r, "id")
	photo, err := s.Records.GetPhoto(r.Context(), p.TenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Datensatz nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datensatz konnte nicht geladen werden")
		return
	}
	if photo.Deletion == nil {
		writeError(w, http.StatusBadRequest, "Kein Löschantrag")
		return
	}
	own := photo.Deletion.RequestedBy == p.UserID
	if !own && !p.CanExecuteDeletion() {
		writeError(w, http.StatusForbidden, "Keine Berechtigung")
		return
	}
	updated, err := s.Records.ClearDeletion(r.Context(), p.TenantID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Löschantrag konnte nicht zurückgezogen werden")
		return
	}
	action := audit.DeletionCancel
	if !own {
		action = audit.DeletionReject
	}
	s.writeAudit(r, p, action, id, map[string]any{
		"requested_by": photo.Deletion.RequestedByName,
		"reason":       photo.Deletion.Reason,
	})
	writeJSON(w, http.StatusOK, presentPhoto(updated))
}

func (s *Server) listDeletionRequests(w http.ResponseWriter, r *http.Request) {
	items, err := s.Records.ListPendingDeletions(r.Context(), principal(r).TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Löschanträge konnten nicht geladen werden")
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, photo := range items {
		out = append(out, presentPhoto(photo))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func deletionForbidden(p auth.Principal, photo records.Photo) string {
	if !p.CanExecuteDeletion() {
		return "Keine Berechtigung"
	}
	if photo.Deletion != nil && photo.Deletion.RequestedBy == p.UserID {
		return "Eigene Löschanträge dürfen nicht selbst ausgeführt werden"
	}
	return ""
}

func (s *Server) deleteRecord(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	id := chi.URLParam(r, "id")
	photo, err := s.Records.GetPhoto(r.Context(), p.TenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Datensatz nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Foto konnte nicht gelöscht werden")
		return
	}
	if msg := deletionForbidden(p, photo); msg != "" {
		writeError(w, http.StatusForbidden, msg)
		return
	}
	photo, err = s.Records.DeletePhoto(r.Context(), p.TenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Datensatz nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Foto konnte nicht gelöscht werden")
		return
	}
	if s.Store != nil {
		_ = s.Store.Delete(r.Context(), photo.OriginalObjectID)
		if photo.ThumbObjectID != "" && photo.ThumbObjectID != photo.OriginalObjectID {
			_ = s.Store.Delete(r.Context(), photo.ThumbObjectID)
		}
	}
	detail := map[string]any{"sightings": len(photo.Sightings)}
	if photo.Deletion != nil {
		detail["requested_by"] = photo.Deletion.RequestedByName
		detail["reason"] = photo.Deletion.Reason
	}
	s.writeAudit(r, p, audit.RecordDelete, id, detail)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
