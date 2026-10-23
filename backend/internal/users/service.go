package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/attributes"
	"openscum/internal/audit"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/internal/config"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
)

// Invalidator drops a user's cached principal after a change.
type Invalidator interface{ Invalidate(id uuid.UUID) }

type Service struct {
	cfg    config.Config
	store  *db.Store
	auth   *auth.Service
	hasher *auth.Hasher
	cache  Invalidator
}

func NewService(cfg config.Config, store *db.Store, authSvc *auth.Service, hasher *auth.Hasher, cache Invalidator) *Service {
	return &Service{cfg: cfg, store: store, auth: authSvc, hasher: hasher, cache: cache}
}

// target is a user loaded for an authorization check.
type target struct {
	user   sqlc.User
	roles  []sqlc.ListUserRolesRow
	teams  []sqlc.ListUserTeamsRow
	authzT authz.Target
}

func loadTarget(ctx context.Context, q *sqlc.Queries, id uuid.UUID) (target, error) {
	u, err := q.GetUser(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return target{}, apperr.ErrNotFound
	}
	if err != nil {
		return target{}, fmt.Errorf("leer usuario: %w", err)
	}
	roles, err := q.ListUserRoles(ctx, id)
	if err != nil {
		return target{}, fmt.Errorf("leer roles: %w", err)
	}
	teams, err := q.ListUserTeams(ctx, id)
	if err != nil {
		return target{}, fmt.Errorf("leer equipos: %w", err)
	}
	t := target{user: u, roles: roles, teams: teams, authzT: authz.Target{UserID: id}}
	for _, r := range roles {
		t.authzT.Roles = append(t.authzT.Roles, authz.Role(r.Role))
	}
	for _, tm := range teams {
		t.authzT.TeamIDs = append(t.authzT.TeamIDs, tm.ID)
	}
	return t, nil
}

func (t target) hasRole(r authz.Role) bool {
	for _, x := range t.roles {
		if x.Role == string(r) {
			return true
		}
	}
	return false
}

// Get returns the user as the principal may see it (RF-115).
func (s *Service) Get(ctx context.Context, p *authz.Principal, id uuid.UUID) (User, error) {
	t, err := loadTarget(ctx, s.store.Queries, id)
	if err != nil {
		return User{}, err
	}
	return s.view(ctx, p, t)
}

func (s *Service) view(ctx context.Context, p *authz.Principal, t target) (User, error) {
	u := User{ID: t.user.ID, Name: t.user.Name, Status: t.user.Status, Teams: make([]TeamRef, 0, len(t.teams))}
	for _, tm := range t.teams {
		u.Teams = append(u.Teams, TeamRef{ID: tm.ID, Name: tm.Name, Source: tm.Source})
	}
	if !p.CanReadFull(t.authzT) {
		return u, nil
	}
	u.FullProfile = true
	u.Email = &t.user.Email
	u.Roles = make([]RoleRef, 0, len(t.roles))
	for _, r := range t.roles {
		u.Roles = append(u.Roles, RoleRef{Role: r.Role, Source: r.Source})
	}

	defs, err := attributes.Load(ctx, s.store.Queries)
	if err != nil {
		return u, err
	}
	values, err := s.store.ListUserAttributeValues(ctx, t.user.ID)
	if err != nil {
		return u, fmt.Errorf("leer atributos: %w", err)
	}
	retired := map[uuid.UUID]bool{}
	for _, d := range defs {
		retired[d.ID] = d.Retired()
	}
	u.Attributes = map[uuid.UUID]json.RawMessage{}
	for _, v := range values {
		switch {
		case !retired[v.AttributeID]:
			u.Attributes[v.AttributeID] = v.Value
		case p.Has(authz.AttributesManage):
			if u.RetiredAttributes == nil {
				u.RetiredAttributes = map[uuid.UUID]json.RawMessage{}
			}
			u.RetiredAttributes[v.AttributeID] = v.Value
		}
	}

	_, err = s.store.GetCredential(ctx, t.user.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return u, fmt.Errorf("leer credencial: %w", err)
	}
	hasPassword := err == nil
	u.HasPassword = &hasPassword
	u.Can = &Abilities{
		Edit:        p.CanActOn(authz.UsersUpdateAll, authz.UsersUpdateTeam, t.authzT),
		SetStatus:   p.CanActOn(authz.UsersStatusAll, authz.UsersStatusTeam, t.authzT),
		Offboard:    p.Has(authz.UsersOffboard),
		AssignRoles: p.CanAssignRole(authz.RoleViewer, t.authzT) || p.CanAssignRole(authz.RoleAdmin, t.authzT),
		ManageTeams: p.CanActOn(authz.TeamMembersAll, authz.TeamMembersTeam, t.authzT),
		SetPassword: p.Has(authz.PasswordsSet) && s.cfg.Auth.LocalEnabled(),
	}
	return u, nil
}

// Create adds a user by hand (RF-66 to RF-71).
func (s *Service) Create(ctx context.Context, p *authz.Principal, req createRequest) (User, error) {
	if err := s.checkCreate(p, req); err != nil {
		return User{}, err
	}
	var hash string
	if req.Password != "" {
		if err := auth.CheckPasswordLength(req.Password); err != nil {
			return User{}, err
		}
		var err error
		if hash, err = s.hasher.Hash(req.Password); err != nil {
			return User{}, err
		}
	}
	role := req.Role
	if role == "" {
		role = s.cfg.Auth.DefaultRole
	}

	var id uuid.UUID
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		u, err := q.CreateUser(ctx, sqlc.CreateUserParams{
			Name: req.Name, Email: req.Email, EmailNormalized: auth.NormalizeEmail(req.Email), Status: StatusActive,
		})
		if auth.IsUniqueViolation(err) {
			return errEmailConflict
		}
		if err != nil {
			return fmt.Errorf("crear usuario: %w", err)
		}
		id = u.ID
		actor := audit.ByUser(p)
		if err := audit.Record(ctx, q, audit.Entry{
			Actor: actor, Action: "user.created", TargetType: "user", TargetID: id.String(),
			After: map[string]string{"name": u.Name, "email": u.Email, "status": u.Status},
		}); err != nil {
			return err
		}
		if hash != "" {
			if err := q.UpsertCredential(ctx, sqlc.UpsertCredentialParams{UserID: id, PasswordHash: hash}); err != nil {
				return fmt.Errorf("guardar credencial: %w", err)
			}
		}
		if role != "" {
			if err := auth.AddRole(ctx, q, actor, id, role, "manual"); err != nil {
				return err
			}
		}
		if req.TeamID != nil {
			if err := addTeam(ctx, q, actor, id, *req.TeamID); err != nil {
				return err
			}
		}
		if err := attributes.Apply(ctx, q, id, req.Attributes, true); err != nil {
			return err
		}
		return s.auth.ApplyBootstrap(ctx, q, id, u.EmailNormalized)
	})
	if err != nil {
		return User{}, err
	}
	return s.Get(ctx, p, id)
}

func (s *Service) checkCreate(p *authz.Principal, req createRequest) error {
	if req.Password != "" {
		if !s.cfg.Auth.LocalEnabled() {
			return errLocalDisabled
		}
		if !p.Has(authz.PasswordsSet) {
			return apperr.ErrForbidden
		}
	}
	if p.Has(authz.UsersCreateAll) {
		return nil
	}
	if !p.Has(authz.UsersCreateTeam) {
		return apperr.ErrForbidden
	}
	// A lead creates users in one of the teams it leads (RF-67), with a basic role.
	if req.TeamID == nil {
		return apperr.ErrValidation.WithDetail("field", "team_id")
	}
	if !p.ScopeTeams[*req.TeamID] {
		return apperr.ErrForbidden
	}
	if req.Role != "" && req.Role != string(authz.RoleMember) && req.Role != string(authz.RoleViewer) {
		return apperr.ErrForbidden
	}
	return nil
}

// Update edits name, email and attributes (RF-72, RF-73, RF-98).
func (s *Service) Update(ctx context.Context, p *authz.Principal, id uuid.UUID, req updateRequest) (User, error) {
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		t, err := loadTarget(ctx, q, id)
		if err != nil {
			return err
		}
		if !p.CanActOn(authz.UsersUpdateAll, authz.UsersUpdateTeam, t.authzT) {
			return apperr.ErrForbidden
		}
		name, email := t.user.Name, t.user.Email
		if req.Name != nil {
			name = *req.Name
		}
		if req.Email != nil {
			email = *req.Email
		}
		if name != t.user.Name || email != t.user.Email {
			_, err := q.UpdateUserProfile(ctx, sqlc.UpdateUserProfileParams{ID: id, Name: name, Email: email, EmailNormalized: auth.NormalizeEmail(email)})
			if auth.IsUniqueViolation(err) {
				return errEmailConflict
			}
			if err != nil {
				return fmt.Errorf("actualizar usuario: %w", err)
			}
			if err := audit.Record(ctx, q, audit.Entry{
				Actor: audit.ByUser(p), Action: "user.updated", TargetType: "user", TargetID: id.String(),
				Before: map[string]string{"name": t.user.Name, "email": t.user.Email},
				After:  map[string]string{"name": name, "email": email},
			}); err != nil {
				return err
			}
		}
		return attributes.Apply(ctx, q, id, req.Attributes, true)
	})
	if err != nil {
		return User{}, err
	}
	return s.Get(ctx, p, id)
}

// Transition changes a user's status: deactivate, reactivate, offboard or
// restore an offboarded user (RF-74 to RF-79).
type Transition string

const (
	Deactivate Transition = "deactivate"
	Reactivate Transition = "reactivate"
	Offboard   Transition = "offboard"
	Restore    Transition = "restore"
)

func (s *Service) SetStatus(ctx context.Context, p *authz.Principal, id uuid.UUID, tr Transition) (User, error) {
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		if err := q.LockAdmins(ctx); err != nil {
			return fmt.Errorf("bloquear administradores: %w", err)
		}
		t, err := loadTarget(ctx, q, id)
		if err != nil {
			return err
		}
		next, err := s.nextStatus(p, t, tr)
		if err != nil {
			return err
		}
		if next == t.user.Status {
			return nil
		}
		if next != StatusActive && t.user.Status == StatusActive && t.hasRole(authz.RoleAdmin) {
			if err := ensureAnotherAdmin(ctx, q); err != nil {
				return err
			}
		}
		if _, err := q.SetUserStatus(ctx, sqlc.SetUserStatusParams{ID: id, Status: next}); err != nil {
			return fmt.Errorf("cambiar estado: %w", err)
		}
		return audit.Record(ctx, q, audit.Entry{
			Actor: audit.ByUser(p), Action: "user.status_changed", TargetType: "user", TargetID: id.String(),
			Before: map[string]string{"status": t.user.Status}, After: map[string]string{"status": next},
		})
	})
	if err != nil {
		return User{}, err
	}
	s.cache.Invalidate(id)
	return s.Get(ctx, p, id)
}

func (s *Service) nextStatus(p *authz.Principal, t target, tr Transition) (string, error) {
	canStatus := p.CanActOn(authz.UsersStatusAll, authz.UsersStatusTeam, t.authzT)
	offboarded := t.user.Status == StatusOffboarded
	switch tr {
	case Deactivate, Reactivate:
		if !canStatus || (offboarded && !p.Has(authz.UsersOffboard)) {
			return "", apperr.ErrForbidden // RF-78a
		}
		if offboarded {
			return "", errOffboarded
		}
		if tr == Deactivate {
			return StatusInactive, nil
		}
		return StatusActive, nil
	case Offboard:
		if !p.Has(authz.UsersOffboard) {
			return "", apperr.ErrForbidden
		}
		return StatusOffboarded, nil
	case Restore:
		if !p.Has(authz.UsersOffboard) {
			return "", apperr.ErrForbidden
		}
		if !offboarded {
			return t.user.Status, nil
		}
		// Back as inactive, with everything it had (RF-78).
		return StatusInactive, nil
	}
	return "", apperr.ErrNotFound
}

// ensureAnotherAdmin must run after LockAdmins in the same transaction (RF-79).
func ensureAnotherAdmin(ctx context.Context, q *sqlc.Queries) error {
	n, err := q.CountActiveAdmins(ctx)
	if err != nil {
		return fmt.Errorf("contar administradores: %w", err)
	}
	if n <= 1 {
		return errLastAdmin
	}
	return nil
}

// SetPassword lets an administrator replace another user's local password (RF-81).
func (s *Service) SetPassword(ctx context.Context, p *authz.Principal, id uuid.UUID, password string) error {
	if !p.Has(authz.PasswordsSet) {
		return apperr.ErrForbidden
	}
	if !s.cfg.Auth.LocalEnabled() {
		return errLocalDisabled
	}
	if err := auth.CheckPasswordLength(password); err != nil {
		return err
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return err
	}
	return s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		if _, err := loadTarget(ctx, q, id); err != nil {
			return err
		}
		if err := q.UpsertCredential(ctx, sqlc.UpsertCredentialParams{UserID: id, PasswordHash: hash}); err != nil {
			return fmt.Errorf("guardar credencial: %w", err)
		}
		return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "user.password_set", TargetType: "user", TargetID: id.String()})
	})
}

// AddRole assigns a role by hand (RF-64, RF-65, RF-83).
func (s *Service) AddRole(ctx context.Context, p *authz.Principal, id uuid.UUID, role string) (User, error) {
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		t, err := loadTarget(ctx, q, id)
		if err != nil {
			return err
		}
		if !p.CanAssignRole(authz.Role(role), t.authzT) {
			return apperr.ErrForbidden
		}
		return auth.AddRole(ctx, q, audit.ByUser(p), id, role, "manual")
	})
	if err != nil {
		return User{}, err
	}
	s.cache.Invalidate(id)
	return s.Get(ctx, p, id)
}

// RemoveRole removes a manual role; IdP roles are managed in the IdP while
// roles come from it (RF-84, RF-85). The last active admin is kept (RF-79).
func (s *Service) RemoveRole(ctx context.Context, p *authz.Principal, id uuid.UUID, role string) (User, error) {
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		if err := q.LockAdmins(ctx); err != nil {
			return fmt.Errorf("bloquear administradores: %w", err)
		}
		t, err := loadTarget(ctx, q, id)
		if err != nil {
			return err
		}
		if !p.CanAssignRole(authz.Role(role), t.authzT) {
			return apperr.ErrForbidden
		}
		current, err := q.GetUserRole(ctx, sqlc.GetUserRoleParams{UserID: id, Role: role})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("leer rol: %w", err)
		}
		if current.Source == "idp" && s.cfg.OIDC.RoleSource != config.RoleSourceLocal {
			return errIdPManaged
		}
		if role == string(authz.RoleAdmin) && t.user.Status == StatusActive {
			if err := ensureAnotherAdmin(ctx, q); err != nil {
				return err
			}
		}
		if _, err := q.DeleteRole(ctx, sqlc.DeleteRoleParams{UserID: id, Role: role}); err != nil {
			return fmt.Errorf("quitar rol: %w", err)
		}
		return audit.Record(ctx, q, audit.Entry{
			Actor: audit.ByUser(p), Action: "user.role_removed", TargetType: "user", TargetID: id.String(),
			Before: map[string]string{"role": role, "source": current.Source},
		})
	})
	if err != nil {
		return User{}, err
	}
	s.cache.Invalidate(id)
	return s.Get(ctx, p, id)
}

// canManageTeam checks the membership permission for one team and one user.
func canManageTeam(p *authz.Principal, t target, teamID uuid.UUID) bool {
	if p.Has(authz.TeamMembersAll) {
		return true
	}
	return p.Has(authz.TeamMembersTeam) && p.ScopeTeams[teamID] && p.InTeamScope(t.authzT)
}

// AddTeam adds a manual membership (RF-83, RF-87).
func (s *Service) AddTeam(ctx context.Context, p *authz.Principal, id, teamID uuid.UUID) (User, error) {
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		t, err := loadTarget(ctx, q, id)
		if err != nil {
			return err
		}
		if !canManageTeam(p, t, teamID) {
			return apperr.ErrForbidden
		}
		return addTeam(ctx, q, audit.ByUser(p), id, teamID)
	})
	if err != nil {
		return User{}, err
	}
	s.cache.Invalidate(id)
	return s.Get(ctx, p, id)
}

func addTeam(ctx context.Context, q *sqlc.Queries, actor audit.Actor, userID, teamID uuid.UUID) error {
	team, err := q.GetTeam(ctx, teamID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.ErrValidation.WithDetail("field", "team_id")
	}
	if err != nil {
		return fmt.Errorf("leer equipo: %w", err)
	}
	if team.ArchivedAt != nil {
		return errTeamArchived
	}
	if err := q.AddManualTeamMember(ctx, sqlc.AddManualTeamMemberParams{TeamID: teamID, UserID: userID}); err != nil {
		return fmt.Errorf("añadir al equipo: %w", err)
	}
	return audit.Record(ctx, q, audit.Entry{
		Actor: actor, Action: "user.team_added", TargetType: "user", TargetID: userID.String(),
		After: map[string]string{"team_id": teamID.String(), "team": team.Name, "source": "manual"},
	})
}

// RemoveTeam removes a manual membership; IdP memberships come back on the
// next login, so they are rejected (RF-84, RF-85).
func (s *Service) RemoveTeam(ctx context.Context, p *authz.Principal, id, teamID uuid.UUID) (User, error) {
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		t, err := loadTarget(ctx, q, id)
		if err != nil {
			return err
		}
		if !canManageTeam(p, t, teamID) {
			return apperr.ErrForbidden
		}
		m, err := q.GetTeamMember(ctx, sqlc.GetTeamMemberParams{TeamID: teamID, UserID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("leer pertenencia: %w", err)
		}
		if m.Source == "idp" {
			return errIdPManaged
		}
		if _, err := q.DeleteTeamMember(ctx, sqlc.DeleteTeamMemberParams{TeamID: teamID, UserID: id}); err != nil {
			return fmt.Errorf("quitar del equipo: %w", err)
		}
		return audit.Record(ctx, q, audit.Entry{
			Actor: audit.ByUser(p), Action: "user.team_removed", TargetType: "user", TargetID: id.String(),
			Before: map[string]string{"team_id": teamID.String(), "source": m.Source},
		})
	})
	if err != nil {
		return User{}, err
	}
	s.cache.Invalidate(id)
	return s.Get(ctx, p, id)
}
