package catalog

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
)

type Service struct {
	store *db.Store
}

func NewService(store *db.Store) *Service { return &Service{store: store} }

// Get returns the whole catalog, retired elements included.
func (s *Service) Get(ctx context.Context) (Catalog, error) {
	out := Catalog{Methodologies: []Methodology{}, Types: []Type{}, Statuses: []Status{}, Priorities: []Priority{}, Sizes: []Size{}}
	ms, err := s.store.ListMethodologies(ctx)
	if err != nil {
		return out, fmt.Errorf("listar metodologías: %w", err)
	}
	phases, err := s.store.ListPhaseDefinitions(ctx)
	if err != nil {
		return out, fmt.Errorf("listar fases: %w", err)
	}
	for _, m := range ms {
		dto := toMethodology(m)
		for _, p := range phases {
			if p.MethodologyID == m.ID {
				dto.Phases = append(dto.Phases, toPhase(p))
			}
		}
		out.Methodologies = append(out.Methodologies, dto)
	}
	types, err := s.store.ListWorkItemTypes(ctx)
	if err != nil {
		return out, fmt.Errorf("listar tipos: %w", err)
	}
	for _, t := range types {
		out.Types = append(out.Types, toType(t))
	}
	statuses, err := s.store.ListStatuses(ctx)
	if err != nil {
		return out, fmt.Errorf("listar estados: %w", err)
	}
	for _, st := range statuses {
		out.Statuses = append(out.Statuses, toStatus(st))
	}
	prios, err := s.store.ListPriorities(ctx)
	if err != nil {
		return out, fmt.Errorf("listar prioridades: %w", err)
	}
	for _, pr := range prios {
		out.Priorities = append(out.Priorities, toPriority(pr))
	}
	sizes, err := s.store.ListWorkItemSizes(ctx)
	if err != nil {
		return out, fmt.Errorf("listar tallas: %w", err)
	}
	for _, sz := range sizes {
		out.Sizes = append(out.Sizes, toSize(sz))
	}
	return out, nil
}

// mutate runs fn in a transaction, maps duplicate names to a conflict and
// audits the result.
func mutate[T any](ctx context.Context, s *Service, p *authz.Principal, kind, action string, id func(T) uuid.UUID,
	fn func(q *sqlc.Queries) (T, error)) (T, error) {
	var out T
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		var err error
		out, err = fn(q)
		if auth.IsUniqueViolation(err) {
			return errNameConflict
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{
			Actor: audit.ByUser(p), Action: "catalog." + kind + "." + action,
			TargetType: kind, TargetID: id(out).String(), After: out,
		})
	})
	return out, err
}

func retiredAction(retired bool) string {
	if retired {
		return "retired"
	}
	return "restored"
}

// Metodologías y fases (RF-1, RF-2) ----------------------------------------------

func methodologyID(m Methodology) uuid.UUID { return m.ID }

func (s *Service) CreateMethodology(ctx context.Context, p *authz.Principal, req methodologyRequest) (Methodology, error) {
	return mutate(ctx, s, p, "methodology", "created", methodologyID, func(q *sqlc.Queries) (Methodology, error) {
		r, err := q.CreateMethodology(ctx, sqlc.CreateMethodologyParams{Name: req.Name, NameNormalized: NormalizeName(req.Name), Position: req.Position})
		return toMethodology(r), err
	})
}

func (s *Service) UpdateMethodology(ctx context.Context, p *authz.Principal, id uuid.UUID, req methodologyRequest) (Methodology, error) {
	return mutate(ctx, s, p, "methodology", "updated", methodologyID, func(q *sqlc.Queries) (Methodology, error) {
		r, err := q.UpdateMethodology(ctx, sqlc.UpdateMethodologyParams{ID: id, Name: req.Name, NameNormalized: NormalizeName(req.Name), Position: req.Position})
		return toMethodology(r), err
	})
}

func (s *Service) SetMethodologyRetired(ctx context.Context, p *authz.Principal, id uuid.UUID, retired bool) (Methodology, error) {
	return mutate(ctx, s, p, "methodology", retiredAction(retired), methodologyID, func(q *sqlc.Queries) (Methodology, error) {
		r, err := q.SetMethodologyRetired(ctx, sqlc.SetMethodologyRetiredParams{ID: id, Retired: retired})
		return toMethodology(r), err
	})
}

func phaseID(ph Phase) uuid.UUID { return ph.ID }

func (s *Service) CreatePhase(ctx context.Context, p *authz.Principal, methodology uuid.UUID, req phaseRequest) (Phase, error) {
	return mutate(ctx, s, p, "phase_definition", "created", phaseID, func(q *sqlc.Queries) (Phase, error) {
		if _, err := q.GetMethodology(ctx, methodology); err != nil {
			return Phase{}, err
		}
		r, err := q.CreatePhaseDefinition(ctx, sqlc.CreatePhaseDefinitionParams{
			MethodologyID: methodology, Name: req.Name, Position: req.Position,
			HasStartGate: req.HasStartGate, HasFinishGate: req.HasFinishGate,
		})
		return toPhase(r), err
	})
}

func (s *Service) UpdatePhase(ctx context.Context, p *authz.Principal, id uuid.UUID, req phaseRequest) (Phase, error) {
	return mutate(ctx, s, p, "phase_definition", "updated", phaseID, func(q *sqlc.Queries) (Phase, error) {
		r, err := q.UpdatePhaseDefinition(ctx, sqlc.UpdatePhaseDefinitionParams{
			ID: id, Name: req.Name, Position: req.Position, HasStartGate: req.HasStartGate, HasFinishGate: req.HasFinishGate,
		})
		return toPhase(r), err
	})
}

func (s *Service) SetPhaseRetired(ctx context.Context, p *authz.Principal, id uuid.UUID, retired bool) (Phase, error) {
	return mutate(ctx, s, p, "phase_definition", retiredAction(retired), phaseID, func(q *sqlc.Queries) (Phase, error) {
		r, err := q.SetPhaseDefinitionRetired(ctx, sqlc.SetPhaseDefinitionRetiredParams{ID: id, Retired: retired})
		return toPhase(r), err
	})
}

// Tipos (RF-3, RF-150) ---------------------------------------------------------------------

func typeID(t Type) uuid.UUID { return t.ID }

func (s *Service) CreateType(ctx context.Context, p *authz.Principal, req typeRequest) (Type, error) {
	return mutate(ctx, s, p, "work_item_type", "created", typeID, func(q *sqlc.Queries) (Type, error) {
		r, err := q.CreateWorkItemType(ctx, sqlc.CreateWorkItemTypeParams{Name: req.Name, NameNormalized: NormalizeName(req.Name), Color: req.Color, Position: req.Position, DefaultWeight: req.DefaultWeight})
		return toType(r), err
	})
}

func (s *Service) UpdateType(ctx context.Context, p *authz.Principal, id uuid.UUID, req typeRequest) (Type, error) {
	return mutate(ctx, s, p, "work_item_type", "updated", typeID, func(q *sqlc.Queries) (Type, error) {
		r, err := q.UpdateWorkItemType(ctx, sqlc.UpdateWorkItemTypeParams{ID: id, Name: req.Name, NameNormalized: NormalizeName(req.Name), Color: req.Color, Position: req.Position, DefaultWeight: req.DefaultWeight})
		return toType(r), err
	})
}

func (s *Service) SetTypeRetired(ctx context.Context, p *authz.Principal, id uuid.UUID, retired bool) (Type, error) {
	return mutate(ctx, s, p, "work_item_type", retiredAction(retired), typeID, func(q *sqlc.Queries) (Type, error) {
		r, err := q.SetWorkItemTypeRetired(ctx, sqlc.SetWorkItemTypeRetiredParams{ID: id, Retired: retired})
		return toType(r), err
	})
}

// Estados (RF-4, RF-5, RF-10) -----------------------------------------------------------

func statusID(st Status) uuid.UUID { return st.ID }

func (s *Service) CreateStatus(ctx context.Context, p *authz.Principal, req statusRequest) (Status, error) {
	if req.IsDefault && req.IsClosed {
		return Status{}, apperr.ErrValidation.WithDetail("field", "is_closed")
	}
	return mutate(ctx, s, p, "status", "created", statusID, func(q *sqlc.Queries) (Status, error) {
		if req.IsDefault {
			if err := q.ClearDefaultStatus(ctx, uuid.Nil); err != nil {
				return Status{}, err
			}
		}
		r, err := q.CreateStatus(ctx, sqlc.CreateStatusParams{
			Name: req.Name, NameNormalized: NormalizeName(req.Name), Color: req.Color, Position: req.Position,
			IsClosed: req.IsClosed, IsDefault: req.IsDefault,
		})
		return toStatus(r), err
	})
}

// UpdateStatus keeps exactly one default (RF-5): the default can only move to
// another status, never be switched off. is_closed is locked once a work item
// uses the status, because it drives derived values and events.
func (s *Service) UpdateStatus(ctx context.Context, p *authz.Principal, id uuid.UUID, req statusRequest) (Status, error) {
	if req.IsDefault && req.IsClosed {
		return Status{}, apperr.ErrValidation.WithDetail("field", "is_closed")
	}
	return mutate(ctx, s, p, "status", "updated", statusID, func(q *sqlc.Queries) (Status, error) {
		cur, err := q.GetStatusForUpdate(ctx, id)
		if err != nil {
			return Status{}, err
		}
		if cur.IsDefault && !req.IsDefault {
			return Status{}, errDefaultRequired
		}
		if req.IsDefault && cur.RetiredAt != nil {
			return Status{}, errDefaultRequired
		}
		if cur.IsClosed != req.IsClosed {
			used, err := q.StatusInUse(ctx, id)
			if err != nil {
				return Status{}, err
			}
			if used {
				return Status{}, errClosedLocked
			}
		}
		if req.IsDefault && !cur.IsDefault {
			if err := q.ClearDefaultStatus(ctx, id); err != nil {
				return Status{}, err
			}
		}
		r, err := q.UpdateStatus(ctx, sqlc.UpdateStatusParams{
			ID: id, Name: req.Name, NameNormalized: NormalizeName(req.Name), Color: req.Color, Position: req.Position,
			IsClosed: req.IsClosed, IsDefault: req.IsDefault,
		})
		return toStatus(r), err
	})
}

func (s *Service) SetStatusRetired(ctx context.Context, p *authz.Principal, id uuid.UUID, retired bool) (Status, error) {
	return mutate(ctx, s, p, "status", retiredAction(retired), statusID, func(q *sqlc.Queries) (Status, error) {
		cur, err := q.GetStatusForUpdate(ctx, id)
		if err != nil {
			return Status{}, err
		}
		if retired && cur.IsDefault {
			return Status{}, errDefaultRequired
		}
		r, err := q.SetStatusRetired(ctx, sqlc.SetStatusRetiredParams{ID: id, Retired: retired})
		return toStatus(r), err
	})
}

// Prioridades (RF-7, RF-10) ---------------------------------------------------------------

func priorityID(pr Priority) uuid.UUID { return pr.ID }

func (s *Service) CreatePriority(ctx context.Context, p *authz.Principal, req priorityRequest) (Priority, error) {
	return mutate(ctx, s, p, "priority", "created", priorityID, func(q *sqlc.Queries) (Priority, error) {
		if req.IsDefault {
			if err := q.ClearDefaultPriority(ctx, uuid.Nil); err != nil {
				return Priority{}, err
			}
		}
		r, err := q.CreatePriority(ctx, sqlc.CreatePriorityParams{
			Name: req.Name, NameNormalized: NormalizeName(req.Name), Color: req.Color, Position: req.Position, IsDefault: req.IsDefault,
		})
		return toPriority(r), err
	})
}

func (s *Service) UpdatePriority(ctx context.Context, p *authz.Principal, id uuid.UUID, req priorityRequest) (Priority, error) {
	return mutate(ctx, s, p, "priority", "updated", priorityID, func(q *sqlc.Queries) (Priority, error) {
		cur, err := q.GetPriority(ctx, id)
		if err != nil {
			return Priority{}, err
		}
		if (cur.IsDefault && !req.IsDefault) || (req.IsDefault && cur.RetiredAt != nil) {
			return Priority{}, errDefaultRequired
		}
		if req.IsDefault && !cur.IsDefault {
			if err := q.ClearDefaultPriority(ctx, id); err != nil {
				return Priority{}, err
			}
		}
		r, err := q.UpdatePriority(ctx, sqlc.UpdatePriorityParams{
			ID: id, Name: req.Name, NameNormalized: NormalizeName(req.Name), Color: req.Color, Position: req.Position, IsDefault: req.IsDefault,
		})
		return toPriority(r), err
	})
}

func (s *Service) SetPriorityRetired(ctx context.Context, p *authz.Principal, id uuid.UUID, retired bool) (Priority, error) {
	return mutate(ctx, s, p, "priority", retiredAction(retired), priorityID, func(q *sqlc.Queries) (Priority, error) {
		cur, err := q.GetPriority(ctx, id)
		if err != nil {
			return Priority{}, err
		}
		if retired && cur.IsDefault {
			return Priority{}, errDefaultRequired
		}
		r, err := q.SetPriorityRetired(ctx, sqlc.SetPriorityRetiredParams{ID: id, Retired: retired})
		return toPriority(r), err
	})
}

// Flujos (RF-6) -----------------------------------------------------------------------------

func (s *Service) Transitions(ctx context.Context, typeID uuid.UUID) ([]Transition, error) {
	if _, err := s.store.GetWorkItemType(ctx, typeID); errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("leer tipo: %w", err)
	}
	rows, err := s.store.ListTransitions(ctx, typeID)
	if err != nil {
		return nil, fmt.Errorf("listar transiciones: %w", err)
	}
	out := make([]Transition, 0, len(rows))
	for _, r := range rows {
		out = append(out, Transition{FromStatusID: r.FromStatusID, ToStatusID: r.ToStatusID, Role: r.Role})
	}
	return out, nil
}

// SetTransitions replaces the workflow of a type. An empty list allows every
// change between active statuses (RF-87).
func (s *Service) SetTransitions(ctx context.Context, p *authz.Principal, typeID uuid.UUID, ts []Transition) ([]Transition, error) {
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		if _, err := q.GetWorkItemType(ctx, typeID); errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		} else if err != nil {
			return fmt.Errorf("leer tipo: %w", err)
		}
		before, err := q.ListTransitions(ctx, typeID)
		if err != nil {
			return fmt.Errorf("listar transiciones: %w", err)
		}
		if err := q.DeleteTransitions(ctx, typeID); err != nil {
			return fmt.Errorf("borrar transiciones: %w", err)
		}
		statuses := map[uuid.UUID]bool{}
		all, err := q.ListStatuses(ctx)
		if err != nil {
			return fmt.Errorf("listar estados: %w", err)
		}
		for _, st := range all {
			statuses[st.ID] = true
		}
		type key struct {
			from, to uuid.UUID
			role     string
		}
		seen := map[key]bool{}
		for _, t := range ts {
			if !statuses[t.FromStatusID] || !statuses[t.ToStatusID] || t.FromStatusID == t.ToStatusID {
				return apperr.ErrValidation.WithDetail("field", "transitions")
			}
			k := key{from: t.FromStatusID, to: t.ToStatusID}
			if t.Role != nil {
				k.role = *t.Role
			}
			if seen[k] {
				continue
			}
			seen[k] = true
			if err := q.CreateTransition(ctx, sqlc.CreateTransitionParams{
				TypeID: typeID, FromStatusID: t.FromStatusID, ToStatusID: t.ToStatusID, Role: t.Role,
			}); err != nil {
				return fmt.Errorf("crear transición: %w", err)
			}
		}
		return audit.Record(ctx, q, audit.Entry{
			Actor: audit.ByUser(p), Action: "catalog.workflow.updated", TargetType: "work_item_type",
			TargetID: typeID.String(), Before: len(before), After: len(seen),
		})
	})
	if err != nil {
		return nil, err
	}
	return s.Transitions(ctx, typeID)
}

// AllowedTargets lists the statuses a work item of typeID in status from can
// move to for these roles (RF-86 to RF-89). Admins can force any change.
func AllowedTargets(ctx context.Context, q *sqlc.Queries, p *authz.Principal, typeID, from uuid.UUID) ([]uuid.UUID, error) {
	if p.HasRole(authz.RoleAdmin) {
		rows, err := q.ListStatuses(ctx)
		if err != nil {
			return nil, fmt.Errorf("listar estados: %w", err)
		}
		out := make([]uuid.UUID, 0, len(rows))
		for _, r := range rows {
			if r.RetiredAt == nil && r.ID != from {
				out = append(out, r.ID)
			}
		}
		return out, nil
	}
	roles := make([]string, 0, len(p.Roles))
	for _, r := range p.Roles {
		roles = append(roles, string(r))
	}
	ids, err := q.AllowedTargetStatuses(ctx, sqlc.AllowedTargetStatusesParams{FromStatusID: from, TypeID: typeID, Roles: roles})
	if err != nil {
		return nil, fmt.Errorf("transiciones permitidas: %w", err)
	}
	if ids == nil {
		ids = []uuid.UUID{}
	}
	return ids, nil
}

// Tallas (spec 006, RF-1, RF-2, RF-9) --------------------------------------------------------

func sizeID(sz Size) uuid.UUID { return sz.ID }

func (s *Service) CreateSize(ctx context.Context, p *authz.Principal, req sizeRequest) (Size, error) {
	return mutate(ctx, s, p, "size", "created", sizeID, func(q *sqlc.Queries) (Size, error) {
		r, err := q.CreateWorkItemSize(ctx, sqlc.CreateWorkItemSizeParams{
			Name: req.Name, NameNormalized: NormalizeName(req.Name), Position: req.Position,
		})
		return toSize(r), err
	})
}

func (s *Service) UpdateSize(ctx context.Context, p *authz.Principal, id uuid.UUID, req sizeRequest) (Size, error) {
	return mutate(ctx, s, p, "size", "updated", sizeID, func(q *sqlc.Queries) (Size, error) {
		r, err := q.UpdateWorkItemSize(ctx, sqlc.UpdateWorkItemSizeParams{
			ID: id, Name: req.Name, NameNormalized: NormalizeName(req.Name), Position: req.Position,
		})
		return toSize(r), err
	})
}

func (s *Service) SetSizeRetired(ctx context.Context, p *authz.Principal, id uuid.UUID, retired bool) (Size, error) {
	return mutate(ctx, s, p, "size", retiredAction(retired), sizeID, func(q *sqlc.Queries) (Size, error) {
		r, err := q.SetWorkItemSizeRetired(ctx, sqlc.SetWorkItemSizeRetiredParams{ID: id, Retired: retired})
		return toSize(r), err
	})
}

// Conversiones --------------------------------------------------------------------------------

func toMethodology(r sqlc.Methodology) Methodology {
	return Methodology{ID: r.ID, Name: r.Name, Position: r.Position, Retired: r.RetiredAt != nil, Phases: []Phase{}}
}

func toPhase(r sqlc.PhaseDefinition) Phase {
	return Phase{ID: r.ID, MethodologyID: r.MethodologyID, Name: r.Name, Position: r.Position,
		HasStartGate: r.HasStartGate, HasFinishGate: r.HasFinishGate, Retired: r.RetiredAt != nil}
}

func toType(r sqlc.WorkItemType) Type {
	return Type{ID: r.ID, Name: r.Name, Color: r.Color, Position: r.Position, DefaultWeight: r.DefaultWeight, Retired: r.RetiredAt != nil}
}

func toStatus(r sqlc.Status) Status {
	return Status{ID: r.ID, Name: r.Name, Color: r.Color, Position: r.Position, IsClosed: r.IsClosed, IsDefault: r.IsDefault, Retired: r.RetiredAt != nil}
}

func toPriority(r sqlc.Priority) Priority {
	return Priority{ID: r.ID, Name: r.Name, Color: r.Color, Position: r.Position, IsDefault: r.IsDefault, Retired: r.RetiredAt != nil}
}

func toSize(r sqlc.WorkItemSize) Size {
	return Size{ID: r.ID, Name: r.Name, Position: r.Position, Retired: r.RetiredAt != nil}
}
