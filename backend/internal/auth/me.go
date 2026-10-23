package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"openscum/internal/authz"
	"openscum/internal/db"
)

// loadMe builds the session user's profile, roles, teams and effective permissions.
func loadMe(ctx context.Context, store *db.Store, p *authz.Principal, method string) (MeResponse, error) {
	u, err := store.GetUser(ctx, p.UserID)
	if err != nil {
		return MeResponse{}, fmt.Errorf("cargar usuario: %w", err)
	}
	roles, err := store.ListUserRoles(ctx, p.UserID)
	if err != nil {
		return MeResponse{}, fmt.Errorf("cargar roles: %w", err)
	}
	teams, err := store.ListUserTeams(ctx, p.UserID)
	if err != nil {
		return MeResponse{}, fmt.Errorf("cargar equipos: %w", err)
	}
	_, err = store.GetCredential(ctx, p.UserID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return MeResponse{}, fmt.Errorf("cargar credencial: %w", err)
	}

	me := MeResponse{
		ID: u.ID, Name: u.Name, Email: u.Email, Status: u.Status, AuthMethod: method,
		HasPassword: err == nil,
		Roles:       make([]RoleAssignment, 0, len(roles)),
		Teams:       make([]TeamAssignment, 0, len(teams)),
		Permissions: p.Permissions,
	}
	for _, r := range roles {
		me.Roles = append(me.Roles, RoleAssignment{Role: r.Role, Source: r.Source})
	}
	for _, t := range teams {
		me.Teams = append(me.Teams, TeamAssignment{ID: t.ID, Name: t.Name, Source: t.Source})
	}
	if me.Permissions == nil {
		me.Permissions = []authz.Permission{}
	}
	return me, nil
}
