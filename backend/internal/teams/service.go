package teams

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
)

// Invalidator drops cached principals whose lead scope may have changed.
type Invalidator interface{ InvalidateAll() }

type Service struct {
	store *db.Store
	cache Invalidator
}

func NewService(store *db.Store, cache Invalidator) *Service {
	return &Service{store: store, cache: cache}
}

func (s *Service) List(ctx context.Context) ([]Team, error) {
	rows, err := s.store.ListTeams(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar equipos: %w", err)
	}
	leaders, err := s.store.ListTeamLeaders(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar líderes: %w", err)
	}
	counts, err := s.store.CountTeamMembers(ctx)
	if err != nil {
		return nil, fmt.Errorf("contar miembros: %w", err)
	}
	out := make([]Team, 0, len(rows))
	for _, r := range rows {
		t := toTeam(r)
		for _, l := range leaders {
			if l.TeamID == r.ID {
				t.Leaders = append(t.Leaders, Leader{UserID: l.UserID, Name: l.Name})
			}
		}
		for _, c := range counts {
			if c.TeamID == r.ID {
				t.MemberCount = c.Members
			}
		}
		out = append(out, t)
	}
	return out, nil
}

func toTeam(r sqlc.Team) Team {
	return Team{ID: r.ID, Name: r.Name, ParentID: r.ParentID, ExternalRef: r.ExternalRef, ArchivedAt: r.ArchivedAt, Leaders: []Leader{}}
}

// Create saves a new team, optionally under a parent (RF-88 to RF-90b).
func (s *Service) Create(ctx context.Context, p *authz.Principal, req teamRequest) (Team, error) {
	var out Team
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		if err := checkParent(ctx, q, uuid.Nil, req.ParentID); err != nil {
			return err
		}
		t, err := q.CreateTeam(ctx, sqlc.CreateTeamParams{
			Name: req.Name, NameNormalized: NormalizeName(req.Name), ParentID: req.ParentID, ExternalRef: req.ExternalRef,
		})
		if auth.IsUniqueViolation(err) {
			return errNameConflict
		}
		if err != nil {
			return fmt.Errorf("crear equipo: %w", err)
		}
		out = toTeam(t)
		return audit.Record(ctx, q, audit.Entry{
			Actor: audit.ByUser(p), Action: "team.created", TargetType: "team", TargetID: t.ID.String(), After: out,
		})
	})
	return out, err
}

// Update renames a team, moves it to another parent or changes its IdP reference.
func (s *Service) Update(ctx context.Context, p *authz.Principal, id uuid.UUID, req teamRequest) (Team, error) {
	var out Team
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		before, err := getTeam(ctx, q, id)
		if err != nil {
			return err
		}
		if err := checkParent(ctx, q, id, req.ParentID); err != nil {
			return err
		}
		t, err := q.UpdateTeam(ctx, sqlc.UpdateTeamParams{
			ID: id, Name: req.Name, NameNormalized: NormalizeName(req.Name), ParentID: req.ParentID, ExternalRef: req.ExternalRef,
		})
		if auth.IsUniqueViolation(err) {
			return errNameConflict
		}
		if err != nil {
			return fmt.Errorf("actualizar equipo: %w", err)
		}
		out = toTeam(t)
		return audit.Record(ctx, q, audit.Entry{
			Actor: audit.ByUser(p), Action: "team.updated", TargetType: "team", TargetID: id.String(), Before: toTeam(before), After: out,
		})
	})
	if err == nil {
		s.cache.InvalidateAll()
	}
	return out, err
}

// checkParent rejects a missing or archived parent and cycles (RF-90a, RF-90b).
func checkParent(ctx context.Context, q *sqlc.Queries, id uuid.UUID, parentID *uuid.UUID) error {
	if parentID == nil {
		return nil
	}
	parent, err := getTeam(ctx, q, *parentID)
	if errors.Is(err, apperr.ErrNotFound) {
		return apperr.ErrValidation.WithDetail("field", "parent_id")
	}
	if err != nil {
		return err
	}
	if parent.ArchivedAt != nil {
		return errArchived.WithDetail("field", "parent_id")
	}
	if id == uuid.Nil {
		return nil
	}
	descendants, err := q.TeamDescendantIDs(ctx, id)
	if err != nil {
		return fmt.Errorf("leer subequipos: %w", err)
	}
	if slices.Contains(descendants, *parentID) {
		return errCycle
	}
	return nil
}

// SetArchived archives (RF-92, RF-90c) or restores a team.
func (s *Service) SetArchived(ctx context.Context, p *authz.Principal, id uuid.UUID, archived bool) (Team, error) {
	var out Team
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		before, err := getTeam(ctx, q, id)
		if err != nil {
			return err
		}
		if archived {
			n, err := q.CountActiveChildren(ctx, &id)
			if err != nil {
				return fmt.Errorf("contar subequipos: %w", err)
			}
			if n > 0 {
				return errHasChildren
			}
		} else if before.ParentID != nil {
			parent, err := getTeam(ctx, q, *before.ParentID)
			if err != nil {
				return err
			}
			if parent.ArchivedAt != nil {
				return errArchived.WithDetail("field", "parent_id")
			}
		}
		t, err := q.SetTeamArchived(ctx, sqlc.SetTeamArchivedParams{ID: id, Archived: archived})
		if err != nil {
			return fmt.Errorf("archivar equipo: %w", err)
		}
		out = toTeam(t)
		action := "team.archived"
		if !archived {
			action = "team.unarchived"
		}
		return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: action, TargetType: "team", TargetID: id.String()})
	})
	if err == nil {
		s.cache.InvalidateAll()
	}
	return out, err
}

// AddLeader gives a lead scope over the team and its subteams (RF-91).
func (s *Service) AddLeader(ctx context.Context, p *authz.Principal, teamID, userID uuid.UUID) error {
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		t, err := getTeam(ctx, q, teamID)
		if err != nil {
			return err
		}
		if t.ArchivedAt != nil {
			return errArchived
		}
		if _, err := q.GetUserRole(ctx, sqlc.GetUserRoleParams{UserID: userID, Role: string(authz.RoleLead)}); errors.Is(err, pgx.ErrNoRows) {
			return errLeaderNotLead
		} else if err != nil {
			return fmt.Errorf("leer rol: %w", err)
		}
		if err := q.AddTeamLeader(ctx, sqlc.AddTeamLeaderParams{TeamID: teamID, UserID: userID}); err != nil {
			return fmt.Errorf("nombrar líder: %w", err)
		}
		return audit.Record(ctx, q, audit.Entry{
			Actor: audit.ByUser(p), Action: "team.leader_added", TargetType: "team", TargetID: teamID.String(),
			After: map[string]string{"user_id": userID.String()},
		})
	})
	if err == nil {
		s.cache.InvalidateAll()
	}
	return err
}

func (s *Service) RemoveLeader(ctx context.Context, p *authz.Principal, teamID, userID uuid.UUID) error {
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		n, err := q.RemoveTeamLeader(ctx, sqlc.RemoveTeamLeaderParams{TeamID: teamID, UserID: userID})
		if err != nil {
			return fmt.Errorf("quitar líder: %w", err)
		}
		if n == 0 {
			return apperr.ErrNotFound
		}
		return audit.Record(ctx, q, audit.Entry{
			Actor: audit.ByUser(p), Action: "team.leader_removed", TargetType: "team", TargetID: teamID.String(),
			Before: map[string]string{"user_id": userID.String()},
		})
	})
	if err == nil {
		s.cache.InvalidateAll()
	}
	return err
}

func getTeam(ctx context.Context, q *sqlc.Queries, id uuid.UUID) (sqlc.Team, error) {
	t, err := q.GetTeam(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, apperr.ErrNotFound
	}
	if err != nil {
		return t, fmt.Errorf("leer equipo: %w", err)
	}
	return t, nil
}
