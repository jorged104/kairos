package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/authz"
	"openscum/internal/config"
	"openscum/internal/db/sqlc"
)

// LoginOIDC resolves the user behind a validated id_token, creates it if
// needed and synchronizes its IdP roles and teams. The changes are committed
// even when the account turns out to be inactive (RF-46 is checked last).
func (s *Service) LoginOIDC(ctx context.Context, cfg config.OIDCConfig, c Claims) (uuid.UUID, error) {
	var userID uuid.UUID
	var status string
	// Two first logins of the same identity race on the unique keys; the
	// loser retries and finds the identity the winner created (RF-26).
	for attempt := 0; ; attempt++ {
		err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
			u, err := s.resolveIdentity(ctx, q, cfg, c)
			if err != nil {
				return err
			}
			if u, err = s.refreshProfile(ctx, q, u, c); err != nil {
				return err
			}
			if err := s.ApplyBootstrap(ctx, q, u.ID, u.EmailNormalized); err != nil {
				return err
			}
			if err := s.syncFromIdP(ctx, q, cfg, u.ID, c); err != nil {
				return err
			}
			userID, status = u.ID, u.Status
			return nil
		})
		if (IsUniqueViolation(err) || errors.Is(err, errIdentityRace)) && attempt == 0 {
			continue
		}
		if err != nil {
			return uuid.Nil, err
		}
		break
	}
	s.loader.Invalidate(userID)
	if status != "active" {
		return uuid.Nil, apperr.ErrAccountInactive
	}
	return userID, nil
}

// resolveIdentity finds the user by issuer + sub (RF-20, RF-21), links it by
// verified email (RF-24) or creates it just in time (RF-23).
func (s *Service) resolveIdentity(ctx context.Context, q *sqlc.Queries, cfg config.OIDCConfig, c Claims) (sqlc.User, error) {
	ident, err := q.GetIdentity(ctx, sqlc.GetIdentityParams{Issuer: c.Issuer, Subject: c.Subject})
	if err == nil {
		u, err := q.GetUser(ctx, ident.UserID)
		if err != nil {
			return u, fmt.Errorf("leer usuario: %w", err)
		}
		return u, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.User{}, fmt.Errorf("leer identidad: %w", err)
	}

	email := NormalizeEmail(c.Email)
	if email == "" {
		return sqlc.User{}, errEmailRequired // RF-27
	}
	u, err := q.GetUserByEmail(ctx, email)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		if u, err = s.createJIT(ctx, q, c); err != nil {
			return u, err
		}
	case err != nil:
		return u, fmt.Errorf("buscar usuario: %w", err)
	case !cfg.LinkByEmail || !c.EmailVerified:
		return u, errAccountConflict // RF-25
	}

	if _, err := q.CreateIdentity(ctx, sqlc.CreateIdentityParams{UserID: u.ID, Issuer: c.Issuer, Subject: c.Subject}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Someone else linked it first: let the retry pick it up.
			return u, errIdentityRace
		}
		return u, fmt.Errorf("vincular identidad: %w", err)
	}
	return u, audit.Record(ctx, q, audit.Entry{
		Actor: audit.IdP, Action: "user.identity_linked", TargetType: "user", TargetID: u.ID.String(),
		After: map[string]string{"issuer": c.Issuer, "subject": c.Subject},
	})
}

// errIdentityRace means another login linked the identity first; the login is retried.
var errIdentityRace = errors.New("identidad vinculada en paralelo")

// createJIT creates a user on its first SSO login; required attributes are
// not asked for (RF-108).
func (s *Service) createJIT(ctx context.Context, q *sqlc.Queries, c Claims) (sqlc.User, error) {
	u, err := q.CreateUser(ctx, sqlc.CreateUserParams{
		Name: displayName(c), Email: strings.TrimSpace(c.Email), EmailNormalized: NormalizeEmail(c.Email), Status: "active",
	})
	if err != nil {
		return u, fmt.Errorf("crear usuario JIT: %w", err)
	}
	return u, audit.Record(ctx, q, audit.Entry{
		Actor: audit.IdP, Action: "user.created", TargetType: "user", TargetID: u.ID.String(),
		After: map[string]string{"name": u.Name, "email": u.Email, "status": u.Status},
	})
}

func displayName(c Claims) string {
	for _, v := range []string{c.Name, c.PreferredUsername, c.Email} {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return c.Subject
}

// refreshProfile makes the IdP the source of name and email (RF-22). An email
// that belongs to someone else is not taken (RF-22a).
func (s *Service) refreshProfile(ctx context.Context, q *sqlc.Queries, u sqlc.User, c Claims) (sqlc.User, error) {
	name, email := displayName(c), strings.TrimSpace(c.Email)
	if email == "" {
		email = u.Email
	}
	if NormalizeEmail(email) != u.EmailNormalized {
		other, err := q.GetUserByEmail(ctx, NormalizeEmail(email))
		if err == nil && other.ID != u.ID {
			s.logger.Warn("el email del IdP pertenece a otro usuario; se conserva el actual", slog.String("user_id", u.ID.String()))
			email = u.Email
		} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return u, fmt.Errorf("buscar email: %w", err)
		}
	}
	if name == u.Name && email == u.Email {
		return u, nil
	}
	updated, err := q.UpdateUserProfile(ctx, sqlc.UpdateUserProfileParams{ID: u.ID, Name: name, Email: email, EmailNormalized: NormalizeEmail(email)})
	if err != nil {
		return u, fmt.Errorf("actualizar perfil: %w", err)
	}
	return updated, nil
}

// MatchesClaim applies a mapping value to a claim value; "/path/*" matches
// every subgroup of /path at any depth, never /path itself (RF-29).
func MatchesClaim(pattern, value string) bool {
	if prefix, ok := strings.CutSuffix(pattern, "/*"); ok {
		return strings.HasPrefix(value, prefix+"/")
	}
	return pattern == value
}

// syncFromIdP replaces the user's IdP roles and teams with the ones computed
// from the claims and the mappings; manual ones are never touched (RF-28 to RF-38).
func (s *Service) syncFromIdP(ctx context.Context, q *sqlc.Queries, cfg config.OIDCConfig, userID uuid.UUID, c Claims) error {
	mappings, err := q.ListIdpMappingsByIssuer(ctx, c.Issuer)
	if err != nil {
		return fmt.Errorf("leer homologaciones: %w", err)
	}
	claimValues := map[string][]string{"roles": c.Roles, "groups": c.Groups}
	wantRoles, wantTeams := map[string]bool{}, map[uuid.UUID]bool{}
	for _, m := range mappings {
		if !slices.ContainsFunc(claimValues[m.Claim], func(v string) bool { return MatchesClaim(m.ExternalValue, v) }) {
			continue // values without a mapping are ignored (RF-37)
		}
		switch m.TargetType {
		case "role":
			if _, ok := authz.ParseRole(m.Target); ok {
				wantRoles[m.Target] = true
			}
		case "team":
			id, err := uuid.Parse(m.Target)
			if err != nil {
				continue
			}
			team, err := q.GetTeam(ctx, id)
			if err != nil || team.ArchivedAt != nil {
				s.logger.Warn("homologación hacia un equipo inexistente o archivado", slog.String("team_id", m.Target))
				continue // RF-93
			}
			wantTeams[id] = true
		}
	}

	if cfg.RoleSource == config.RoleSourceIdP { // RF-33
		if err := s.syncRoles(ctx, q, userID, wantRoles); err != nil {
			return err
		}
	}
	if err := s.syncTeams(ctx, q, userID, wantTeams); err != nil { // RF-34
		return err
	}
	return s.ensureRole(ctx, q, cfg, userID)
}

func (s *Service) syncRoles(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, want map[string]bool) error {
	current, err := q.ListUserRoles(ctx, userID)
	if err != nil {
		return fmt.Errorf("leer roles: %w", err)
	}
	for _, r := range current {
		if r.Source != "idp" || want[r.Role] {
			continue
		}
		if r.Role == string(authz.RoleAdmin) {
			keep, err := s.isLastActiveAdmin(ctx, q, userID)
			if err != nil {
				return err
			}
			if keep { // RF-38
				if err := audit.Record(ctx, q, audit.Entry{
					Actor: audit.IdP, Action: "user.role_kept_last_admin", TargetType: "user", TargetID: userID.String(),
					After: map[string]string{"role": r.Role},
				}); err != nil {
					return err
				}
				continue
			}
		}
		if _, err := q.DeleteRole(ctx, sqlc.DeleteRoleParams{UserID: userID, Role: r.Role}); err != nil {
			return fmt.Errorf("quitar rol: %w", err)
		}
		if err := audit.Record(ctx, q, audit.Entry{
			Actor: audit.IdP, Action: "user.role_removed", TargetType: "user", TargetID: userID.String(),
			Before: map[string]string{"role": r.Role, "source": "idp"},
		}); err != nil {
			return err
		}
	}
	for role := range want {
		if err := AddRole(ctx, q, audit.IdP, userID, role, "idp"); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) isLastActiveAdmin(ctx context.Context, q *sqlc.Queries, userID uuid.UUID) (bool, error) {
	if err := q.LockAdmins(ctx); err != nil {
		return false, fmt.Errorf("bloquear administradores: %w", err)
	}
	u, err := q.GetUser(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("leer usuario: %w", err)
	}
	if u.Status != "active" {
		return false, nil
	}
	n, err := q.CountActiveAdmins(ctx)
	if err != nil {
		return false, fmt.Errorf("contar administradores: %w", err)
	}
	return n <= 1, nil
}

func (s *Service) syncTeams(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, want map[uuid.UUID]bool) error {
	current, err := q.ListUserTeams(ctx, userID)
	if err != nil {
		return fmt.Errorf("leer equipos: %w", err)
	}
	have := map[uuid.UUID]bool{}
	for _, t := range current {
		have[t.ID] = true
		if t.Source != "idp" || want[t.ID] {
			continue
		}
		if _, err := q.DeleteTeamMember(ctx, sqlc.DeleteTeamMemberParams{TeamID: t.ID, UserID: userID}); err != nil {
			return fmt.Errorf("quitar del equipo: %w", err)
		}
		if err := audit.Record(ctx, q, audit.Entry{
			Actor: audit.IdP, Action: "user.team_removed", TargetType: "user", TargetID: userID.String(),
			Before: map[string]string{"team_id": t.ID.String(), "team": t.Name, "source": "idp"},
		}); err != nil {
			return err
		}
	}
	for id := range want {
		if have[id] {
			continue // already there, manual or IdP (RF-32)
		}
		if _, err := q.AddIdpTeamMember(ctx, sqlc.AddIdpTeamMemberParams{TeamID: id, UserID: userID}); err != nil {
			return fmt.Errorf("añadir al equipo: %w", err)
		}
		if err := audit.Record(ctx, q, audit.Entry{
			Actor: audit.IdP, Action: "user.team_added", TargetType: "user", TargetID: userID.String(),
			After: map[string]string{"team_id": id.String(), "source": "idp"},
		}); err != nil {
			return err
		}
	}
	return nil
}

// ensureRole gives DEFAULT_ROLE to a user left without roles, or rejects the
// login when there is none (RF-35, RF-36).
func (s *Service) ensureRole(ctx context.Context, q *sqlc.Queries, cfg config.OIDCConfig, userID uuid.UUID) error {
	roles, err := q.ListUserRoles(ctx, userID)
	if err != nil {
		return fmt.Errorf("leer roles: %w", err)
	}
	if len(roles) > 0 {
		return nil
	}
	if s.cfg.DefaultRole == "" {
		return errNoRole
	}
	source := "idp"
	if cfg.RoleSource == config.RoleSourceLocal {
		source = "manual" // no IdP roles are created in local mode (RF-33)
	}
	return AddRole(ctx, q, audit.IdP, userID, s.cfg.DefaultRole, source)
}
