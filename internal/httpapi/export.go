package httpapi

import (
	"bytes"
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"schmutzfink/internal/audit"
	"schmutzfink/internal/ingest"
	"schmutzfink/internal/records"
)

func (s *Server) exportRecords(w http.ResponseWriter, r *http.Request) {
	format, err := parseExportFormat(r.URL.Query().Get("format"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	items, total, err := s.loadExportItems(w, r)
	if err != nil {
		return
	}

	var fieldDefs []records.FieldDef
	if s.Records != nil {
		fieldDefs, err = s.Records.ListFields(r.Context(), principal(r).TenantID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Felder konnten nicht geladen werden")
			return
		}
	}

	truncated := total > len(items)
	p := principal(r)
	s.writeAudit(r, p, audit.RecordExport, "", map[string]any{
		"format":    format,
		"total":     total,
		"count":     len(items),
		"truncated": truncated,
	})

	name := "schmutzfink-sichtungen-" + time.Now().UTC().Format("2006-01-02") + "." + format
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("X-Export-Total", strconv.Itoa(total))
	w.Header().Set("X-Export-Count", strconv.Itoa(len(items)))
	if truncated {
		w.Header().Set("X-Export-Truncated", "true")
	}

	rows := presentExportItems(items)
	if format == "csv" {
		writeCSVExport(w, rows, fieldDefs)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"exported_at": time.Now().UTC(),
		"total":       total,
		"truncated":   truncated,
		"items":       rows,
	})
}

func (s *Server) summaryRecords(w http.ResponseWriter, r *http.Request) {
	items, total, err := s.loadExportItems(w, r)
	if err != nil {
		return
	}
	var defs []records.FieldDef
	if s.Records != nil {
		defs, err = s.Records.ListFields(r.Context(), principal(r).TenantID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Felder konnten nicht geladen werden")
			return
		}
	}
	writeJSON(w, http.StatusOK, records.Summarize(items, total, defs))
}

func (s *Server) loadExportItems(w http.ResponseWriter, r *http.Request) ([]records.Record, int, error) {
	f, semantic, err := parseFilter(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return nil, 0, err
	}
	f.ForExport = true
	f.Offset = 0
	f.Limit = records.ExportMax

	vec, err := s.optionalExportVector(w, r, f, semantic)
	if err != nil {
		return nil, 0, err
	}
	if vec != nil {
		items, total, err := s.Records.SearchByVector(r.Context(), principal(r).TenantID, vec, f, "")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Suche fehlgeschlagen")
			return nil, 0, err
		}
		return items, total, nil
	}

	items, total, err := s.Records.List(r.Context(), principal(r).TenantID, f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Datensätze konnten nicht geladen werden")
		return nil, 0, err
	}
	return items, total, nil
}

func (s *Server) optionalExportVector(w http.ResponseWriter, r *http.Request, f records.ListFilter, semantic bool) ([]float32, error) {
	if r.Method == http.MethodPost && strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		if err := s.requireEmbeddings(w); err != nil {
			return nil, err
		}
		r.Body = http.MaxBytesReader(w, r.Body, ingest.MaxBytes+1<<20)
		if err := r.ParseMultipartForm(ingest.MaxBytes); err != nil {
			writeError(w, http.StatusBadRequest, "Datei zu groß oder ungültig")
			return nil, err
		}
		file, _, err := r.FormFile("file")
		if err == nil {
			defer file.Close()
			data, err := ingest.ReadLimited(file)
			if err != nil {
				writeError(w, http.StatusBadRequest, "Datei zu groß")
				return nil, err
			}
			img, err := ingest.Decode(data)
			if err != nil {
				if errors.Is(err, ingest.ErrTooManyPixels) {
					writeError(w, http.StatusBadRequest, "Bildauflösung zu hoch")
					return nil, err
				}
				if strings.Contains(err.Error(), "unsupported") {
					writeError(w, http.StatusBadRequest, "Nur JPEG, PNG, GIF oder WebP")
					return nil, err
				}
				writeError(w, http.StatusBadRequest, "Bild konnte nicht gelesen werden")
				return nil, err
			}
			vec, err := s.Clip.EmbedImage(img)
			if err != nil {
				writeError(w, http.StatusServiceUnavailable, "Bildsuche ist gerade nicht verfügbar")
				return nil, err
			}
			return vec, nil
		}
	}
	if semantic && f.Query != "" {
		if err := s.requireEmbeddings(w); err != nil {
			return nil, err
		}
		vec, err := s.Clip.EmbedText(f.Query)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "Bildsuche ist gerade nicht verfügbar")
			return nil, err
		}
		return vec, nil
	}
	return nil, nil
}

func parseExportFormat(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "json":
		return "json", nil
	case "csv":
		return "csv", nil
	default:
		return "", errors.New("format muss csv oder json sein")
	}
}

type exportItem struct {
	SightingID     string            `json:"sighting_id"`
	RecordID       string            `json:"record_id"`
	Note           string            `json:"note"`
	Tags           []string          `json:"tags"`
	Lat            *float64          `json:"lat"`
	Lon            *float64          `json:"lon"`
	CapturedAt     *time.Time        `json:"captured_at"`
	UploadedAt     time.Time         `json:"uploaded_at"`
	UploadedByName string            `json:"uploaded_by_name"`
	MotifID        string            `json:"motif_id,omitempty"`
	MotifTitle     string            `json:"motif_title,omitempty"`
	CaseID         string            `json:"case_id,omitempty"`
	CaseTitle      string            `json:"case_title,omitempty"`
	CaseKind       string            `json:"case_kind,omitempty"`
	Score          *float64          `json:"score,omitempty"`
	Fields         map[string]string `json:"fields,omitempty"`
}

func presentExportItems(items []records.Record) []exportItem {
	out := make([]exportItem, 0, len(items))
	for _, rec := range items {
		out = append(out, presentExportItem(rec))
	}
	return out
}

func presentExportItem(rec records.Record) exportItem {
	tags := rec.Tags
	if tags == nil {
		tags = []string{}
	}
	item := exportItem{
		SightingID:     rec.ID,
		RecordID:       rec.RecordID,
		Note:           rec.Note,
		Tags:           tags,
		Lat:            rec.Lat,
		Lon:            rec.Lon,
		CapturedAt:     rec.CapturedAt,
		UploadedAt:     rec.UploadedAt,
		UploadedByName: rec.UploadedByName,
		MotifID:        rec.MotifID,
		MotifTitle:     rec.MotifTitle,
		CaseID:         rec.CaseID,
		CaseTitle:      rec.CaseTitle,
		CaseKind:       rec.CaseKind,
		Score:          rec.Score,
		Fields:         rec.Fields,
	}
	return item
}

func writeCSVExport(w http.ResponseWriter, items []exportItem, fields []records.FieldDef) {
	var buf bytes.Buffer
	buf.WriteString("\uFEFF")
	cw := csv.NewWriter(&buf)
	cw.Comma = ';'
	cw.UseCRLF = true

	head := []string{
		"sighting_id", "record_id", "note", "tags", "lat", "lon",
		"captured_at", "uploaded_at", "uploaded_by_name",
		"motif_id", "motif_title", "case_id", "case_title", "case_kind", "score",
	}
	for _, f := range fields {
		head = append(head, "field."+f.Key)
	}
	_ = cw.Write(head)
	for _, item := range items {
		row := []string{
			item.SightingID,
			item.RecordID,
			item.Note,
			strings.Join(item.Tags, ", "),
			formatExportFloat(item.Lat),
			formatExportFloat(item.Lon),
			formatExportTime(item.CapturedAt),
			item.UploadedAt.UTC().Format(time.RFC3339),
			item.UploadedByName,
			item.MotifID,
			item.MotifTitle,
			item.CaseID,
			item.CaseTitle,
			item.CaseKind,
			formatExportFloat(item.Score),
		}
		for _, f := range fields {
			if item.Fields == nil {
				row = append(row, "")
				continue
			}
			row = append(row, item.Fields[f.Key])
		}
		_ = cw.Write(row)
	}
	cw.Flush()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

func formatExportFloat(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

func formatExportTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
