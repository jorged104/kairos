// Package customfields manages the custom fields of projects and work items
// (RF-12 to RF-17). Values follow the same rules as the user attributes of
// spec 003 through the shared fields package.
package customfields

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
	"openscum/internal/fields"
)

const (
	EntityProject  = "project"
	EntityWorkItem = "work_item"
)

type Field struct {
	ID        uuid.UUID       `json:"id"`
	Entity    string          `json:"entity"`
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	Required  bool            `json:"required"`
	Options   []fields.Option `json:"options"`
	TypeIDs   []uuid.UUID     `json:"work_item_type_ids"`
	RetiredAt *time.Time      `json:"retired_at"`
}

func (f Field) Retired() bool { return f.RetiredAt != nil }

// AppliesTo reports whether a work item field applies to a work item type (RF-14).
func (f Field) AppliesTo(typeID uuid.UUID) bool {
	return len(f.TypeIDs) == 0 || slices.Contains(f.TypeIDs, typeID)
}

type fieldRequest struct {
	Entity   string          `json:"entity" validate:"omitempty,oneof=project work_item"`
	Name     string          `json:"name" validate:"required,max=200"`
	Type     string          `json:"type" validate:"required,oneof=text number date boolean select"`
	Required bool            `json:"required"`
	Options  []fields.Option `json:"options" validate:"max=200,dive"`
	TypeIDs  []uuid.UUID     `json:"work_item_type_ids" validate:"max=100"`
}

func (r *fieldRequest) Normalize() {
	r.Name = strings.TrimSpace(r.Name)
	for i := range r.Options {
		r.Options[i].Value = strings.TrimSpace(r.Options[i].Value)
	}
}

// Values maps field ids to JSON values; a JSON null removes the value.
type Values map[uuid.UUID]json.RawMessage

var (
	errNameConflict = apperr.New(http.StatusConflict, "catalog_name_conflict", "Ya existe un campo activo con ese nombre")
	errTypeLocked   = apperr.New(http.StatusConflict, "custom_field_type_locked", "No se puede cambiar el tipo de un campo con valores")
	errRetired      = apperr.New(http.StatusConflict, "custom_field_retired", "El campo está retirado")
	ErrInvalid      = apperr.New(http.StatusBadRequest, "custom_field_invalid", "El valor no es válido para el campo")
	ErrRequired     = apperr.New(http.StatusBadRequest, "custom_field_required", "Falta un campo obligatorio")
	ErrNotEnabled   = apperr.New(http.StatusBadRequest, "custom_field_not_enabled", "El campo no aplica aquí")
)

type Service struct {
	store *db.Store
}

func NewService(store *db.Store) *Service { return &Service{store: store} }

// Load returns every field definition, retired ones included.
func Load(ctx context.Context, q *sqlc.Queries) ([]Field, error) {
	rows, err := q.ListCustomFields(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar campos: %w", err)
	}
	types, err := q.ListCustomFieldTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar tipos de campos: %w", err)
	}
	out := make([]Field, 0, len(rows))
	for _, r := range rows {
		f, err := toField(r)
		if err != nil {
			return nil, err
		}
		for _, t := range types {
			if t.FieldID == r.ID {
				f.TypeIDs = append(f.TypeIDs, t.TypeID)
			}
		}
		out = append(out, f)
	}
	return out, nil
}

func (s *Service) List(ctx context.Context) ([]Field, error) { return Load(ctx, s.store.Queries) }

func (s *Service) Create(ctx context.Context, p *authz.Principal, req fieldRequest) (Field, error) {
	if req.Entity == "" {
		return Field{}, apperr.ErrValidation.WithDetail("field", "entity")
	}
	options, err := fields.CheckOptions(req.Type, req.Options, nil)
	if err != nil {
		return Field{}, err
	}
	var out Field
	err = s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		row, err := q.CreateCustomField(ctx, sqlc.CreateCustomFieldParams{
			Entity: req.Entity, Name: req.Name, NameNormalized: fields.NormalizeName(req.Name),
			Type: req.Type, Required: req.Required, Options: options,
		})
		if auth.IsUniqueViolation(err) {
			return errNameConflict
		}
		if err != nil {
			return fmt.Errorf("crear campo: %w", err)
		}
		if out, err = toField(row); err != nil {
			return err
		}
		if out.TypeIDs, err = setTypes(ctx, q, out, req.TypeIDs); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "custom_field.created", TargetType: "custom_field", TargetID: out.ID.String(), After: out})
	})
	return out, err
}

// Update changes a field. The type is locked once there are values; making it
// required does not invalidate existing records (RF-17, as RF-99 of 003).
func (s *Service) Update(ctx context.Context, p *authz.Principal, id uuid.UUID, req fieldRequest) (Field, error) {
	var out Field
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		before, err := get(ctx, q, id)
		if err != nil {
			return err
		}
		if req.Entity != "" && req.Entity != before.Entity {
			return apperr.ErrValidation.WithDetail("field", "entity")
		}
		if req.Type != before.Type {
			used, err := q.CustomFieldHasValues(ctx, id)
			if err != nil {
				return fmt.Errorf("comprobar valores: %w", err)
			}
			if used {
				return errTypeLocked
			}
		}
		previous := before.Options
		if req.Type != before.Type {
			previous = nil
		}
		options, err := fields.CheckOptions(req.Type, req.Options, previous)
		if err != nil {
			return err
		}
		row, err := q.UpdateCustomField(ctx, sqlc.UpdateCustomFieldParams{
			ID: id, Name: req.Name, NameNormalized: fields.NormalizeName(req.Name), Type: req.Type, Required: req.Required, Options: options,
		})
		if auth.IsUniqueViolation(err) {
			return errNameConflict
		}
		if err != nil {
			return fmt.Errorf("actualizar campo: %w", err)
		}
		if out, err = toField(row); err != nil {
			return err
		}
		if out.TypeIDs, err = setTypes(ctx, q, out, req.TypeIDs); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "custom_field.updated", TargetType: "custom_field", TargetID: id.String(), Before: before, After: out})
	})
	return out, err
}

func (s *Service) SetRetired(ctx context.Context, p *authz.Principal, id uuid.UUID, retired bool) (Field, error) {
	var out Field
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		if _, err := get(ctx, q, id); err != nil {
			return err
		}
		row, err := q.SetCustomFieldRetired(ctx, sqlc.SetCustomFieldRetiredParams{ID: id, Retired: retired})
		if auth.IsUniqueViolation(err) {
			return errNameConflict
		}
		if err != nil {
			return fmt.Errorf("retirar campo: %w", err)
		}
		if out, err = toField(row); err != nil {
			return err
		}
		action := "custom_field.retired"
		if !retired {
			action = "custom_field.restored"
		}
		return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: action, TargetType: "custom_field", TargetID: id.String()})
	})
	return out, err
}

// setTypes replaces the work item types a field is limited to.
func setTypes(ctx context.Context, q *sqlc.Queries, f Field, typeIDs []uuid.UUID) ([]uuid.UUID, error) {
	if f.Entity != EntityWorkItem && len(typeIDs) > 0 {
		return nil, apperr.ErrValidation.WithDetail("field", "work_item_type_ids")
	}
	if err := q.DeleteCustomFieldTypes(ctx, f.ID); err != nil {
		return nil, fmt.Errorf("borrar tipos del campo: %w", err)
	}
	out := []uuid.UUID{}
	for _, t := range typeIDs {
		if _, err := q.GetWorkItemType(ctx, t); errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.ErrValidation.WithDetail("field", "work_item_type_ids")
		} else if err != nil {
			return nil, fmt.Errorf("leer tipo: %w", err)
		}
		if err := q.AddCustomFieldType(ctx, sqlc.AddCustomFieldTypeParams{FieldID: f.ID, TypeID: t}); err != nil {
			return nil, fmt.Errorf("limitar campo: %w", err)
		}
		if !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out, nil
}

func get(ctx context.Context, q *sqlc.Queries, id uuid.UUID) (Field, error) {
	row, err := q.GetCustomField(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Field{}, apperr.ErrNotFound
	}
	if err != nil {
		return Field{}, fmt.Errorf("leer campo: %w", err)
	}
	return toField(row)
}

func toField(r sqlc.CustomField) (Field, error) {
	f := Field{ID: r.ID, Entity: r.Entity, Name: r.Name, Type: r.Type, Required: r.Required, RetiredAt: r.RetiredAt,
		Options: []fields.Option{}, TypeIDs: []uuid.UUID{}}
	if len(r.Options) > 0 {
		if err := json.Unmarshal(r.Options, &f.Options); err != nil {
			return f, fmt.Errorf("leer opciones del campo %s: %w", r.ID, err)
		}
	}
	return f, nil
}

// Scope tells Apply which fields may hold a value for a record.
type Scope struct {
	Entity  string
	Enabled map[uuid.UUID]bool // fields enabled in the project (RF-15)
	TypeID  uuid.UUID          // work items only (RF-14)
}

func (sc Scope) applies(f Field) bool {
	if f.Entity != sc.Entity || !sc.Enabled[f.ID] {
		return false
	}
	return sc.Entity != EntityWorkItem || f.AppliesTo(sc.TypeID)
}

// Store reads and writes the values of one record.
type Store interface {
	current(ctx context.Context) (map[uuid.UUID]json.RawMessage, error)
	upsert(ctx context.Context, field uuid.UUID, v json.RawMessage) error
	remove(ctx context.Context, field uuid.UUID) error
}

// Apply validates and saves values in the caller's transaction and returns
// the record's values afterwards. With requireAll, every active required field
// that applies must end up with a value (RF-16).
func Apply(ctx context.Context, defs []Field, st Store, sc Scope, values Values, requireAll bool) (map[uuid.UUID]json.RawMessage, error) {
	current, err := st.current(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]Field, len(defs))
	for _, f := range defs {
		byID[f.ID] = f
	}
	for id, raw := range values {
		f, ok := byID[id]
		if !ok {
			return nil, ErrInvalid.WithDetail("custom_field", id.String())
		}
		if f.Retired() {
			// Retired values are read-only history (RF-17, as RF-104 of 003).
			if bytes.Equal(bytes.TrimSpace(raw), current[id]) {
				continue
			}
			return nil, errRetired.WithDetail("custom_field", id.String())
		}
		if !sc.applies(f) {
			return nil, ErrNotEnabled.WithDetail("custom_field", id.String())
		}
		v, ok := fields.Normalize(f.Type, f.Options, raw, current[id])
		if !ok {
			return nil, ErrInvalid.WithDetail("custom_field", id.String())
		}
		if v == nil {
			if err := st.remove(ctx, id); err != nil {
				return nil, err
			}
			delete(current, id)
			continue
		}
		if err := st.upsert(ctx, id, v); err != nil {
			return nil, err
		}
		current[id] = v
	}
	if requireAll {
		for _, f := range defs {
			if f.Required && !f.Retired() && sc.applies(f) && current[f.ID] == nil {
				return nil, ErrRequired.WithDetail("custom_field", f.ID.String())
			}
		}
	}
	return current, nil
}

type projectValues struct {
	q  *sqlc.Queries
	id uuid.UUID
}

// ProjectValues is the value store of a project.
func ProjectValues(q *sqlc.Queries, projectID uuid.UUID) Store {
	return projectValues{q: q, id: projectID}
}

func (s projectValues) current(ctx context.Context) (map[uuid.UUID]json.RawMessage, error) {
	rows, err := s.q.ListProjectCustomValues(ctx, s.id)
	if err != nil {
		return nil, fmt.Errorf("leer valores: %w", err)
	}
	out := make(map[uuid.UUID]json.RawMessage, len(rows))
	for _, r := range rows {
		out[r.FieldID] = r.Value
	}
	return out, nil
}

func (s projectValues) upsert(ctx context.Context, field uuid.UUID, v json.RawMessage) error {
	if err := s.q.UpsertProjectCustomValue(ctx, sqlc.UpsertProjectCustomValueParams{ProjectID: s.id, FieldID: field, Value: v}); err != nil {
		return fmt.Errorf("guardar valor: %w", err)
	}
	return nil
}

func (s projectValues) remove(ctx context.Context, field uuid.UUID) error {
	if err := s.q.DeleteProjectCustomValue(ctx, sqlc.DeleteProjectCustomValueParams{ProjectID: s.id, FieldID: field}); err != nil {
		return fmt.Errorf("borrar valor: %w", err)
	}
	return nil
}

type workItemValues struct {
	q  *sqlc.Queries
	id uuid.UUID
}

// WorkItemValues is the value store of a work item.
func WorkItemValues(q *sqlc.Queries, workItemID uuid.UUID) Store {
	return workItemValues{q: q, id: workItemID}
}

func (s workItemValues) current(ctx context.Context) (map[uuid.UUID]json.RawMessage, error) {
	rows, err := s.q.ListWorkItemCustomValues(ctx, s.id)
	if err != nil {
		return nil, fmt.Errorf("leer valores: %w", err)
	}
	out := make(map[uuid.UUID]json.RawMessage, len(rows))
	for _, r := range rows {
		out[r.FieldID] = r.Value
	}
	return out, nil
}

func (s workItemValues) upsert(ctx context.Context, field uuid.UUID, v json.RawMessage) error {
	if err := s.q.UpsertWorkItemCustomValue(ctx, sqlc.UpsertWorkItemCustomValueParams{WorkItemID: s.id, FieldID: field, Value: v}); err != nil {
		return fmt.Errorf("guardar valor: %w", err)
	}
	return nil
}

func (s workItemValues) remove(ctx context.Context, field uuid.UUID) error {
	if err := s.q.DeleteWorkItemCustomValue(ctx, sqlc.DeleteWorkItemCustomValueParams{WorkItemID: s.id, FieldID: field}); err != nil {
		return fmt.Errorf("borrar valor: %w", err)
	}
	return nil
}

// EnabledFields returns the fields enabled in a project.
func EnabledFields(ctx context.Context, q *sqlc.Queries, projectID uuid.UUID) (map[uuid.UUID]bool, error) {
	ids, err := q.ListProjectCustomFieldIDs(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("campos del proyecto: %w", err)
	}
	out := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}
