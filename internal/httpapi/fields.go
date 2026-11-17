package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"schmutzfink/internal/audit"
	"schmutzfink/internal/records"
)

type fieldBody struct {
	Key       string   `json:"key"`
	Label     string   `json:"label"`
	Type      string   `json:"type"`
	Options   []string `json:"options"`
	Required  bool     `json:"required"`
	SortOrder int      `json:"sort_order"`
}

func (s *Server) listFields(w http.ResponseWriter, r *http.Request) {
	items, err := s.Records.ListFields(r.Context(), principal(r).TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Felder konnten nicht geladen werden")
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, d := range items {
		out = append(out, presentField(d))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) createField(w http.ResponseWriter, r *http.Request) {
	var body fieldBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	p := principal(r)
	d, err := s.Records.CreateField(r.Context(), p.TenantID, records.FieldDef{
		Key:       body.Key,
		Label:     body.Label,
		Type:      body.Type,
		Options:   body.Options,
		Required:  body.Required,
		SortOrder: body.SortOrder,
	})
	if errors.Is(err, records.ErrInvalidFieldType) {
		writeError(w, http.StatusBadRequest, "Feldtyp muss text, number, date, bool oder select sein")
		return
	}
	if errors.Is(err, records.ErrFieldKey) {
		writeError(w, http.StatusBadRequest, "Bezeichnung nötig")
		return
	}
	if errors.Is(err, records.ErrInvalidFieldValue) {
		writeError(w, http.StatusBadRequest, "Auswahlfeld braucht mindestens eine Option")
		return
	}
	if errors.Is(err, records.ErrFieldInUse) {
		writeError(w, http.StatusConflict, "Schlüssel ist bereits vergeben")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Feld konnte nicht angelegt werden")
		return
	}
	s.writeAudit(r, p, audit.FieldCreate, d.ID, map[string]any{"key": d.Key, "label": d.Label})
	writeJSON(w, http.StatusCreated, presentField(d))
}

func (s *Server) patchField(w http.ResponseWriter, r *http.Request) {
	var body fieldBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	p := principal(r)
	d, err := s.Records.UpdateField(r.Context(), p.TenantID, chi.URLParam(r, "id"), records.FieldDef{
		Label:     body.Label,
		Options:   body.Options,
		Required:  body.Required,
		SortOrder: body.SortOrder,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Feld nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Feld konnte nicht gespeichert werden")
		return
	}
	s.writeAudit(r, p, audit.FieldPatch, d.ID, map[string]any{"key": d.Key})
	writeJSON(w, http.StatusOK, presentField(d))
}

func (s *Server) deleteField(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	id := chi.URLParam(r, "id")
	err := s.Records.DeleteField(r.Context(), p.TenantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Feld nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Feld konnte nicht gelöscht werden")
		return
	}
	s.writeAudit(r, p, audit.FieldDelete, id, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func presentField(d records.FieldDef) map[string]any {
	return map[string]any{
		"id":         d.ID,
		"key":        d.Key,
		"label":      d.Label,
		"type":       d.Type,
		"options":    d.Options,
		"required":   d.Required,
		"sort_order": d.SortOrder,
	}
}

func parseFieldValues(raw json.RawMessage) (map[string]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out map[string]string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func applySightingFields(w http.ResponseWriter, r *http.Request, rec *records.Repo, tenantID, sightingID string, raw json.RawMessage) bool {
	values, err := parseFieldValues(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Felder ungültig")
		return false
	}
	if values == nil {
		return true
	}
	if err := rec.SetSightingFields(r.Context(), tenantID, sightingID, values); err != nil {
		if errors.Is(err, records.ErrInvalidFieldValue) || errors.Is(err, records.ErrFieldKey) {
			writeError(w, http.StatusBadRequest, "Feldwert ungültig oder Pflichtfeld leer")
			return false
		}
		writeError(w, http.StatusInternalServerError, "Felder konnten nicht gespeichert werden")
		return false
	}
	return true
}
