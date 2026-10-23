package attributes

import (
	"context"
	"errors"
	"fmt"

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

type Service struct {
	store *db.Store
}

func NewService(store *db.Store) *Service { return &Service{store: store} }

// List returns the active attributes; administrators also see retired ones (RF-105).
func (s *Service) List(ctx context.Context, p *authz.Principal) ([]Attribute, error) {
	all, err := Load(ctx, s.store.Queries)
	if err != nil {
		return nil, err
	}
	if p.Has(authz.AttributesManage) {
		return all, nil
	}
	out := make([]Attribute, 0, len(all))
	for _, a := range all {
		if !a.Retired() {
			out = append(out, a)
		}
	}
	return out, nil
}

// Create defines a new attribute for every user without code changes (RF-94 to RF-96).
func (s *Service) Create(ctx context.Context, p *authz.Principal, req createRequest) (Attribute, error) {
	options, err := checkOptions(req.Type, req.Options, nil)
	if err != nil {
		return Attribute{}, err
	}
	var out Attribute
	err = s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		row, err := q.CreateAttribute(ctx, sqlc.CreateAttributeParams{
			Name: req.Name, NameNormalized: NormalizeName(req.Name), Type: req.Type, Required: req.Required, Options: options,
		})
		if auth.IsUniqueViolation(err) {
			return errNameConflict
		}
		if err != nil {
			return fmt.Errorf("crear atributo: %w", err)
		}
		if out, err = toAttribute(row); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "attribute.created", TargetType: "attribute", TargetID: row.ID.String(), After: out})
	})
	return out, err
}

// Update renames an attribute, changes its type (only without values,
// RF-102), whether it is required (RF-99) and its options (RF-107).
func (s *Service) Update(ctx context.Context, p *authz.Principal, id uuid.UUID, req updateRequest) (Attribute, error) {
	var out Attribute
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		before, err := get(ctx, q, id)
		if err != nil {
			return err
		}
		if before.Retired() {
			return errRetired
		}
		if req.Type != before.Type {
			n, err := q.CountAttributeValues(ctx, id)
			if err != nil {
				return fmt.Errorf("contar valores: %w", err)
			}
			if n > 0 {
				return errTypeLocked
			}
		}
		var previous []Option
		if req.Type == before.Type {
			previous = before.Options
		}
		options, err := checkOptions(req.Type, req.Options, previous)
		if err != nil {
			return err
		}
		row, err := q.UpdateAttribute(ctx, sqlc.UpdateAttributeParams{
			ID: id, Name: req.Name, NameNormalized: NormalizeName(req.Name), Type: req.Type, Required: req.Required, Options: options,
		})
		if auth.IsUniqueViolation(err) {
			return errNameConflict
		}
		if err != nil {
			return fmt.Errorf("actualizar atributo: %w", err)
		}
		if out, err = toAttribute(row); err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "attribute.updated", TargetType: "attribute", TargetID: id.String(), Before: before, After: out})
	})
	return out, err
}

// checkOptions validates the options of a select attribute; removed options
// are kept as retired (RF-107).
var checkOptions = fields.CheckOptions

// SetRetired retires an attribute keeping its values (RF-103) or restores it (RF-106).
func (s *Service) SetRetired(ctx context.Context, p *authz.Principal, id uuid.UUID, retired bool) (Attribute, error) {
	var out Attribute
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		if _, err := get(ctx, q, id); err != nil {
			return err
		}
		row, err := q.SetAttributeRetired(ctx, sqlc.SetAttributeRetiredParams{ID: id, Retired: retired})
		if auth.IsUniqueViolation(err) {
			// Restoring would clash with an active attribute of the same name.
			return errNameConflict
		}
		if err != nil {
			return fmt.Errorf("retirar atributo: %w", err)
		}
		if out, err = toAttribute(row); err != nil {
			return err
		}
		action := "attribute.retired"
		if !retired {
			action = "attribute.restored"
		}
		return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: action, TargetType: "attribute", TargetID: id.String()})
	})
	return out, err
}

func get(ctx context.Context, q *sqlc.Queries, id uuid.UUID) (Attribute, error) {
	row, err := q.GetAttribute(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Attribute{}, apperr.ErrNotFound
	}
	if err != nil {
		return Attribute{}, fmt.Errorf("leer atributo: %w", err)
	}
	return toAttribute(row)
}
