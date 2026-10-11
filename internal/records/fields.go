package records

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrInvalidFieldType  = errors.New("invalid field type")
	ErrInvalidFieldValue = errors.New("invalid field value")
	ErrFieldKey          = errors.New("invalid field key")
	ErrFieldInUse        = errors.New("field key in use")
)

const (
	FieldText   = "text"
	FieldNumber = "number"
	FieldDate   = "date"
	FieldBool   = "bool"
	FieldSelect = "select"
)

type FieldDef struct {
	ID        string
	Key       string
	Label     string
	Type      string
	Options   []string
	Required  bool
	SortOrder int
}

type FieldFilter struct {
	Key   string
	Value string
}

func NormalizeFieldType(t string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case FieldText, FieldNumber, FieldDate, FieldBool, FieldSelect:
		return strings.ToLower(strings.TrimSpace(t)), true
	default:
		return "", false
	}
}

func FieldKeyFrom(label string) string {
	repl := strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss")
	s := repl.Replace(strings.ToLower(strings.TrimSpace(label)))
	var b strings.Builder
	prevUnderscore := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevUnderscore = false
		case unicode.IsSpace(r) || r == '-' || r == '_':
			if b.Len() > 0 && !prevUnderscore {
				b.WriteByte('_')
				prevUnderscore = true
			}
		}
	}
	out := strings.Trim(b.String(), "_")
	if len(out) > 40 {
		out = out[:40]
		out = strings.Trim(out, "_")
	}
	return out
}

func NormalizeFieldValue(def FieldDef, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	switch def.Type {
	case FieldText:
		return raw, nil
	case FieldNumber:
		n, err := strconv.ParseFloat(strings.Replace(raw, ",", ".", 1), 64)
		if err != nil {
			return "", ErrInvalidFieldValue
		}
		return strconv.FormatFloat(n, 'f', -1, 64), nil
	case FieldDate:
		t, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return "", ErrInvalidFieldValue
		}
		return t.Format("2006-01-02"), nil
	case FieldBool:
		switch strings.ToLower(raw) {
		case "1", "true", "ja", "yes", "on":
			return "ja", nil
		case "0", "false", "nein", "no", "off":
			return "nein", nil
		default:
			return "", ErrInvalidFieldValue
		}
	case FieldSelect:
		for _, opt := range def.Options {
			if strings.EqualFold(strings.TrimSpace(opt), raw) {
				return strings.TrimSpace(opt), nil
			}
		}
		return "", ErrInvalidFieldValue
	default:
		return "", ErrInvalidFieldType
	}
}

func (r *Repo) ListFields(ctx context.Context, tenantID string) ([]FieldDef, error) {
	rows, err := r.DB.Query(ctx, `
		SELECT id, key, label, field_type, options, required, sort_order
		FROM custom_fields
		WHERE tenant_id = $1
		ORDER BY sort_order, label, key`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FieldDef
	for rows.Next() {
		d, err := scanFieldDef(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if out == nil {
		out = []FieldDef{}
	}
	return out, rows.Err()
}

func (r *Repo) CreateField(ctx context.Context, tenantID string, in FieldDef) (FieldDef, error) {
	typ, ok := NormalizeFieldType(in.Type)
	if !ok {
		return FieldDef{}, ErrInvalidFieldType
	}
	in.Type = typ
	in.Label = strings.TrimSpace(in.Label)
	if in.Label == "" {
		return FieldDef{}, ErrFieldKey
	}
	in.Key = FieldKeyFrom(in.Key)
	if in.Key == "" {
		in.Key = FieldKeyFrom(in.Label)
	}
	if in.Key == "" {
		return FieldDef{}, ErrFieldKey
	}
	in.Options = cleanOptions(in.Options, typ)
	if typ == FieldSelect && len(in.Options) == 0 {
		return FieldDef{}, ErrInvalidFieldValue
	}
	in.ID = uuid.NewString()
	_, err := r.DB.Exec(ctx, `
		INSERT INTO custom_fields (id, tenant_id, key, label, field_type, options, required, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		in.ID, tenantID, in.Key, in.Label, in.Type, in.Options, in.Required, in.SortOrder)
	if isUniqueViolation(err) {
		return FieldDef{}, ErrFieldInUse
	}
	if err != nil {
		return FieldDef{}, err
	}
	return in, nil
}

func (r *Repo) UpdateField(ctx context.Context, tenantID, id string, in FieldDef) (FieldDef, error) {
	cur, err := r.getField(ctx, tenantID, id)
	if err != nil {
		return FieldDef{}, err
	}
	in.Label = strings.TrimSpace(in.Label)
	if in.Label == "" {
		in.Label = cur.Label
	}
	in.Options = cleanOptions(in.Options, cur.Type)
	if cur.Type == FieldSelect && len(in.Options) == 0 {
		in.Options = cur.Options
	}
	_, err = r.DB.Exec(ctx, `
		UPDATE custom_fields
		SET label = $3, options = $4, required = $5, sort_order = $6
		WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, in.Label, in.Options, in.Required, in.SortOrder)
	if err != nil {
		return FieldDef{}, err
	}
	cur.Label = in.Label
	cur.Options = in.Options
	cur.Required = in.Required
	cur.SortOrder = in.SortOrder
	return cur, nil
}

func (r *Repo) DeleteField(ctx context.Context, tenantID, id string) error {
	tag, err := r.DB.Exec(ctx, `DELETE FROM custom_fields WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repo) SetSightingFields(ctx context.Context, tenantID, sightingID string, values map[string]string) error {
	if values == nil {
		return nil
	}
	if _, err := r.GetSighting(ctx, tenantID, sightingID); err != nil {
		return err
	}
	defs, err := r.ListFields(ctx, tenantID)
	if err != nil {
		return err
	}
	byKey := map[string]FieldDef{}
	for _, d := range defs {
		byKey[d.Key] = d
	}
	for key, raw := range values {
		def, ok := byKey[key]
		if !ok {
			return ErrFieldKey
		}
		val, err := NormalizeFieldValue(def, raw)
		if err != nil {
			return err
		}
		if val == "" {
			if def.Required {
				return ErrInvalidFieldValue
			}
			if _, err := r.DB.Exec(ctx, `
				DELETE FROM sighting_field_values
				WHERE sighting_id = $1 AND field_id = $2`, sightingID, def.ID); err != nil {
				return err
			}
			continue
		}
		if _, err := r.DB.Exec(ctx, `
			INSERT INTO sighting_field_values (sighting_id, field_id, value_text)
			VALUES ($1, $2, $3)
			ON CONFLICT (sighting_id, field_id) DO UPDATE SET value_text = EXCLUDED.value_text`,
			sightingID, def.ID, val); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repo) getField(ctx context.Context, tenantID, id string) (FieldDef, error) {
	return scanFieldDef(r.DB.QueryRow(ctx, `
		SELECT id, key, label, field_type, options, required, sort_order
		FROM custom_fields
		WHERE tenant_id = $1 AND id = $2`, tenantID, id))
}

func (r *Repo) attachSightingFields(ctx context.Context, tenantID string, items []Sighting) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, len(items))
	index := map[string]int{}
	for i := range items {
		ids[i] = items[i].ID
		index[items[i].ID] = i
		if items[i].Fields == nil {
			items[i].Fields = map[string]string{}
		}
	}
	return r.scanFieldRows(ctx, tenantID, ids, func(sid, key, val string) {
		if i, ok := index[sid]; ok {
			items[i].Fields[key] = val
		}
	})
}

func (r *Repo) attachRecordFields(ctx context.Context, tenantID string, items []Record) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, len(items))
	index := map[string]int{}
	for i := range items {
		ids[i] = items[i].ID
		index[items[i].ID] = i
		if items[i].Fields == nil {
			items[i].Fields = map[string]string{}
		}
	}
	return r.scanFieldRows(ctx, tenantID, ids, func(sid, key, val string) {
		if i, ok := index[sid]; ok {
			items[i].Fields[key] = val
		}
	})
}

func (r *Repo) scanFieldRows(ctx context.Context, tenantID string, ids []string, add func(sightingID, key, value string)) error {
	rows, err := r.DB.Query(ctx, `
		SELECT v.sighting_id::text, f.key, v.value_text
		FROM sighting_field_values v
		JOIN custom_fields f ON f.id = v.field_id
		WHERE f.tenant_id = $1 AND v.sighting_id = ANY($2::uuid[])`, tenantID, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sid, key, val string
		if err := rows.Scan(&sid, &key, &val); err != nil {
			return err
		}
		add(sid, key, val)
	}
	return rows.Err()
}

func scanFieldDef(row scanner) (FieldDef, error) {
	var d FieldDef
	err := row.Scan(&d.ID, &d.Key, &d.Label, &d.Type, &d.Options, &d.Required, &d.SortOrder)
	if d.Options == nil {
		d.Options = []string{}
	}
	if err == pgx.ErrNoRows {
		return FieldDef{}, err
	}
	return d, err
}

func cleanOptions(opts []string, typ string) []string {
	if typ != FieldSelect {
		return []string{}
	}
	var out []string
	seen := map[string]bool{}
	for _, opt := range opts {
		opt = strings.TrimSpace(opt)
		if opt == "" {
			continue
		}
		k := strings.ToLower(opt)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, opt)
	}
	return out
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
