// Package authz holds the roles, the permissions and the single place where
// they are mapped to each other (RF-57 to RF-60). Endpoints check permissions,
// never role names.
package authz

import (
	"context"
	"slices"

	"github.com/google/uuid"
)

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleLead   Role = "lead"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

// Roles lists every role in descending priority (RF-57).
var Roles = []Role{RoleAdmin, RoleLead, RoleMember, RoleViewer}

// ParseRole validates a role name.
func ParseRole(s string) (Role, bool) {
	r := Role(s)
	return r, slices.Contains(Roles, r)
}

// TopRole returns the highest-priority role, or "" when there is none.
func TopRole(roles []Role) Role {
	for _, r := range Roles {
		if slices.Contains(roles, r) {
			return r
		}
	}
	return ""
}

type Permission string

// Permissions ending in "all" apply to every user; those ending in "team" only
// to users in the actor's lead scope (see Principal.InTeamScope).
const (
	DirectoryRead     Permission = "directory:read"
	UsersReadFullAll  Permission = "users:read_full:all"
	UsersReadFullTeam Permission = "users:read_full:team"
	UsersCreateAll    Permission = "users:create:all"
	UsersCreateTeam   Permission = "users:create:team"
	UsersUpdateAll    Permission = "users:update:all"
	UsersUpdateTeam   Permission = "users:update:team"
	UsersStatusAll    Permission = "users:status:all"
	UsersStatusTeam   Permission = "users:status:team"
	UsersOffboard     Permission = "users:offboard"
	RolesAssignAll    Permission = "roles:assign:all"
	// RolesAssignBasic allows member and viewer only, inside the team scope.
	RolesAssignBasic  Permission = "roles:assign:basic"
	TeamMembersAll    Permission = "teams:members:all"
	TeamMembersTeam   Permission = "teams:members:team"
	TeamsManage       Permission = "teams:manage"
	AttributesManage  Permission = "attributes:manage"
	IdpMappingsManage Permission = "idp_mappings:manage"
	PasswordsSet      Permission = "passwords:set"
	AuditRead         Permission = "audit:read"
	SelfPassword      Permission = "self:password"
)

// rolePermissions is the permission matrix of spec 003.
var rolePermissions = map[Role][]Permission{
	RoleAdmin: {
		DirectoryRead, UsersReadFullAll, UsersCreateAll, UsersUpdateAll, UsersStatusAll,
		UsersOffboard, RolesAssignAll, TeamMembersAll, TeamsManage, AttributesManage,
		IdpMappingsManage, PasswordsSet, AuditRead, SelfPassword,
	},
	RoleLead: {
		DirectoryRead, UsersReadFullTeam, UsersCreateTeam, UsersUpdateTeam, UsersStatusTeam,
		RolesAssignBasic, TeamMembersTeam, SelfPassword,
	},
	RoleMember: {DirectoryRead, SelfPassword},
	RoleViewer: {DirectoryRead, SelfPassword},
}

// PermissionsFor returns the union of the permissions of the roles (RF-58), sorted.
func PermissionsFor(roles []Role) []Permission {
	var out []Permission
	for _, r := range roles {
		for _, p := range rolePermissions[r] {
			if !slices.Contains(out, p) {
				out = append(out, p)
			}
		}
	}
	slices.Sort(out)
	return out
}

// Principal is the authenticated user as seen by the authorization checks.
type Principal struct {
	UserID      uuid.UUID
	Status      string
	Roles       []Role
	Permissions []Permission
	// ScopeTeams holds the teams the user leads plus all their subteams.
	ScopeTeams map[uuid.UUID]bool
}

func NewPrincipal(id uuid.UUID, status string, roles []Role, scope []uuid.UUID) *Principal {
	p := &Principal{UserID: id, Status: status, Roles: roles, Permissions: PermissionsFor(roles), ScopeTeams: map[uuid.UUID]bool{}}
	for _, t := range scope {
		p.ScopeTeams[t] = true
	}
	return p
}

func (p *Principal) Has(perm Permission) bool { return slices.Contains(p.Permissions, perm) }

func (p *Principal) HasRole(r Role) bool { return slices.Contains(p.Roles, r) }

// Target describes the user an operation acts on.
type Target struct {
	UserID  uuid.UUID
	Roles   []Role
	TeamIDs []uuid.UUID
}

// InTeamScope reports whether the target belongs to a team the principal
// leads (directly or through a subteam) and is a member or viewer (RF-63).
func (p *Principal) InTeamScope(t Target) bool {
	if t.UserID == p.UserID {
		return false
	}
	if top := TopRole(t.Roles); top == RoleAdmin || top == RoleLead {
		return false
	}
	for _, id := range t.TeamIDs {
		if p.ScopeTeams[id] {
			return true
		}
	}
	return false
}

// CanActOn checks a pair of all/team permissions against a target.
func (p *Principal) CanActOn(all, team Permission, t Target) bool {
	if p.Has(all) {
		return true
	}
	return team != "" && p.Has(team) && p.InTeamScope(t)
}

// CanReadFull reports whether the principal sees the full profile of the target.
func (p *Principal) CanReadFull(t Target) bool {
	return t.UserID == p.UserID || p.CanActOn(UsersReadFullAll, UsersReadFullTeam, t)
}

// CanAssignRole checks RF-64 and RF-65 on top of the team scope.
func (p *Principal) CanAssignRole(role Role, t Target) bool {
	if p.Has(RolesAssignAll) {
		return true
	}
	if t.UserID == p.UserID {
		return false
	}
	return p.Has(RolesAssignBasic) && (role == RoleMember || role == RoleViewer) && p.InTeamScope(t)
}

type ctxKey struct{}

func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// FromContext returns the principal set by the session middleware, or nil.
func FromContext(ctx context.Context) *Principal {
	p, _ := ctx.Value(ctxKey{}).(*Principal)
	return p
}
