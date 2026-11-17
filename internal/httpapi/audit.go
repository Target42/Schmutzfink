package httpapi

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"schmutzfink/internal/audit"
	"schmutzfink/internal/auth"
)

func (s *Server) writeAudit(r *http.Request, actor auth.Principal, action, subject string, detail map[string]any) {
	if s.Audit == nil {
		return
	}
	ev := audit.Event{
		TenantID: actor.TenantID,
		UserID:   actor.UserID,
		Username: actor.Username,
		Action:   action,
		Subject:  subject,
		IP:       clientIP(r.RemoteAddr),
		Detail:   detail,
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Second)
	defer cancel()
	if err := s.Audit.Write(ctx, ev); err != nil {
		log.Printf("audit: %v", err)
	}
}

func (s *Server) listAudit(w http.ResponseWriter, r *http.Request) {
	limit, offset := 50, 0
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "limit ungültig")
			return
		}
		limit = n
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "offset ungültig")
			return
		}
		offset = n
	}
	if s.Audit == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []audit.Event{}, "total": 0})
		return
	}
	items, total, err := s.Audit.List(r.Context(), principal(r).TenantID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Protokoll konnte nicht geladen werden")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}
