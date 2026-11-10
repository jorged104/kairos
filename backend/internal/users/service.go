package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/accounts"
	"openscum/internal/apperr"
	"openscum/internal/attributes"
	"openscum/internal/audit"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/internal/config"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
	"openscum/internal/loose"
)

// Invalidator drops a user's cached principal after a change.
type Invalidator interface{ Invalidate(id uuid.UUID) }

type Service struct {
	cfg      func() config.Config // the settings of the system in force (spec 015)
	store    *db.Store
	auth     *auth.Service
	accounts *accounts.Service // invitations and links (spec 019)
	cache    Invalidator
}

func NewService(cfg func() config.Config, store *db.Store, authSvc *auth.Service, accountsSvc *accounts.Service, cache Invalidator) *Service {
	return &Service{cfg: cfg, store: store, auth: authSvc, accounts: accountsSvc, cache: cache}
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
	if !hasPassword {
		inv, err := s.store.PendingInvitation(ctx, t.user.ID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return u, fmt.Errorf("leer invitación: %w", err)
		}
		if err == nil {
			u.Invitation = &Invitation{ExpiresAt: inv, Expired: !time.Now().Before(inv)}
		}
	}
	u.Capacity = t.user.WeeklyCapacity
	u.Can = &Abilities{
		Edit:        p.CanActOn(authz.UsersUpdateAll, authz.UsersUpdateTeam, t.authzT),
		SetStatus:   p.CanActOn(authz.UsersStatusAll, authz.UsersStatusTeam, t.authzT),
		Offboard:    p.Has(authz.UsersOffboard),
		AssignRoles: p.CanAssignRole(authz.RoleViewer, t.authzT) || p.CanAssignRole(authz.RoleAdmin, t.authzT),
		ManageTeams: p.CanActOn(authz.TeamMembersAll, authz.TeamMembersTeam, t.authzT),
		Invite:      p.CanActOn(authz.UsersCreateAll, authz.UsersCreateTeam, t.authzT) && !hasPassword && t.user.Status == StatusActive,
		PasswordLink: p.Has(authz.PasswordsSet) && t.user.Status == StatusActive &&
			(s.cfg().Auth.LocalEnabled() || auth.NormalizeEmail(t.user.Email) == auth.NormalizeEmail(s.cfg().Auth.BootstrapAdminEmail)),
	}
	return u, nil
}

// Create adds a user by hand (RF-66 to RF-71).
func (s *Service) Create(ctx context.Context, p *authz.Principal, req createRequest) (User, error) {
	if err := s.checkCreate(p, req); err != nil {
		return User{}, err
	}
	role := req.Role
	if role == "" {
		role = s.cfg().Auth.DefaultRole
	}

	var id uuid.UUID
	var invitation accounts.Outgoing
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
		// The person creates their password from the invitation (spec 019, RF-11).
		if invitation, err = s.accounts.Invite(ctx, q, p.UserID, u); err != nil {
			return err
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
	delivery := s.accounts.Deliver(ctx, invitation) // RF-12, RF-13
	out, err := s.Get(ctx, p, id)
	out.Delivery = &delivery
	return out, err
}

func (s *Service) checkCreate(p *authz.Principal, req createRequest) error {
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

// SetCapacity changes someone's weekly capacity, or clears it (spec 008,
// RF-46, RF-47): administrators, and leaders of one of their teams.
func (s *Service) SetCapacity(ctx context.Context, p *authz.Principal, id uuid.UUID, capacity *int32) (User, error) {
	if capacity != nil && (*capacity < 0 || *capacity > 1000) {
		return User{}, apperr.ErrValidation.WithDetail("field", "capacity")
	}
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		t, err := loadTarget(ctx, q, id)
		if err != nil {
			return err
		}
		if !p.CanActOn(authz.UsersUpdateAll, authz.UsersUpdateTeam, t.authzT) {
			return apperr.ErrForbidden
		}
		if _, err := q.SetUserCapacity(ctx, sqlc.SetUserCapacityParams{ID: id, Capacity: capacity}); err != nil {
			return fmt.Errorf("guardar capacidad: %w", err)
		}
		return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "user.capacity_changed", TargetType: "user",
			TargetID: id.String(), Before: map[string]any{"capacity": t.user.WeeklyCapacity}, After: map[string]any{"capacity": capacity}})
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
		if next != StatusActive {
			if err := accounts.Revoke(ctx, q, id); err != nil { // spec 019, RF-42
				return err
			}
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

var errHasPassword = apperr.New(http.StatusConflict, "user_has_password", "La persona ya tiene contraseña; envíale un enlace de nueva contraseña")

// ResendInvitation voids the last invitation and sends a new one of 7 days
// (spec 019, RF-20, RF-21).
func (s *Service) ResendInvitation(ctx context.Context, p *authz.Principal, id uuid.UUID) (accounts.Delivery, error) {
	var out accounts.Outgoing
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		t, err := loadTarget(ctx, q, id)
		if err != nil {
			return err
		}
		if !p.CanActOn(authz.UsersCreateAll, authz.UsersCreateTeam, t.authzT) {
			return apperr.ErrForbidden
		}
		if t.user.Status != StatusActive {
			return apperr.ErrAccountInactive
		}
		if _, err := q.GetCredential(ctx, id); err == nil {
			return errHasPassword
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("leer credencial: %w", err)
		}
		out, err = s.accounts.Invite(ctx, q, p.UserID, t.user)
		return err
	})
	if err != nil {
		return accounts.Delivery{}, err
	}
	return s.accounts.Deliver(ctx, out), nil
}

// PasswordLink sends a person a link to a new password, which replaces
// setting it by hand (spec 019, RF-32 to RF-35).
func (s *Service) PasswordLink(ctx context.Context, p *authz.Principal, id uuid.UUID) (accounts.Delivery, error) {
	if !p.Has(authz.PasswordsSet) {
		return accounts.Delivery{}, apperr.ErrForbidden
	}
	var out accounts.Outgoing
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		t, err := loadTarget(ctx, q, id)
		if err != nil {
			return err
		}
		if t.user.Status != StatusActive {
			return apperr.ErrAccountInactive
		}
		cfg := s.cfg().Auth
		if !cfg.LocalEnabled() && auth.NormalizeEmail(t.user.Email) != auth.NormalizeEmail(cfg.BootstrapAdminEmail) {
			return errLocalDisabled
		}
		out, err = s.accounts.ResetLink(ctx, q, p.UserID, t.user)
		return err
	})
	if err != nil {
		return accounts.Delivery{}, err
	}
	return s.accounts.Deliver(ctx, out), nil
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
		if current.Source == "idp" && s.cfg().OIDC.RoleSource != config.RoleSourceLocal {
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
	// Their task of the team's «each person» tasks (spec 022, RF-25).
	if err := loose.Join(ctx, q, actor, teamID, userID); err != nil {
		return err
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
