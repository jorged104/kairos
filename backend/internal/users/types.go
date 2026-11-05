// Package users manages user accounts, their roles and team memberships, and
// the directory (RF-61 to RF-87, RF-109 to RF-116).
package users

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"openscum/internal/accounts"
	"openscum/internal/apperr"
	"openscum/internal/attributes"
)

const (
	StatusActive     = "active"
	StatusInactive   = "inactive"
	StatusOffboarded = "offboarded"
)

type RoleRef struct {
	Role   string `json:"role"`
	Source string `json:"source"`
}

type TeamRef struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Source string    `json:"source"`
}

// Abilities tells the frontend which actions the viewer may take on the user.
type Abilities struct {
	Edit        bool `json:"edit"`
	SetStatus   bool `json:"set_status"`
	Offboard    bool `json:"offboard"`
	AssignRoles bool `json:"assign_roles"`
	ManageTeams bool `json:"manage_teams"`
	// Resend the invitation, and send a link to a new password (spec 019, RF-20, RF-32).
	Invite       bool `json:"invite"`
	PasswordLink bool `json:"password_link"`
}

// Invitation is a pending one: unused, of a person without a password (spec 019, RF-19).
type Invitation struct {
	ExpiresAt time.Time `json:"expires_at"`
	Expired   bool      `json:"expired"`
}

// User is what a viewer may see of a user (RF-115): the basic directory
// fields always; the rest only with a full-profile permission.
type User struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Teams       []TeamRef `json:"teams"`
	FullProfile bool      `json:"full_profile"`

	Email      *string                       `json:"email,omitempty"`
	Roles      []RoleRef                     `json:"roles,omitempty"`
	Attributes map[uuid.UUID]json.RawMessage `json:"attributes,omitempty"`
	// RetiredAttributes is read-only history, only for administrators (RF-105).
	RetiredAttributes map[uuid.UUID]json.RawMessage `json:"retired_attributes,omitempty"`
	HasPassword       *bool                         `json:"has_password,omitempty"`
	Invitation        *Invitation                   `json:"invitation,omitempty"`
	// Delivery says, on creation, whether the invitation went out, or its link (spec 019, RF-13).
	Delivery *accounts.Delivery `json:"delivery,omitempty"`
	// Weekly capacity in weight points (spec 008, RF-46).
	Capacity *int32     `json:"capacity,omitempty"`
	Can      *Abilities `json:"can,omitempty"`
}

type createRequest struct {
	Name  string `json:"name" validate:"required,max=200"`
	Email string `json:"email" validate:"required,email,max=320"`
	// No initial password: the person gets an invitation (spec 019, RF-10).
	Role       string            `json:"role" validate:"omitempty,oneof=admin lead member viewer"`
	TeamID     *uuid.UUID        `json:"team_id"`
	Attributes attributes.Values `json:"attributes"`
}

func (r *createRequest) Normalize() {
	r.Name = strings.TrimSpace(r.Name)
	r.Email = strings.TrimSpace(r.Email)
}

type updateRequest struct {
	Name       *string           `json:"name" validate:"omitempty,min=1,max=200"`
	Email      *string           `json:"email" validate:"omitempty,email,max=320"`
	Attributes attributes.Values `json:"attributes"`
}

func (r *updateRequest) Normalize() {
	for _, s := range []*string{r.Name, r.Email} {
		if s != nil {
			*s = strings.TrimSpace(*s)
		}
	}
}

type passwordRequest struct {
	Password string `json:"password" validate:"required"`
}

type roleRequest struct {
	Role string `json:"role" validate:"required,oneof=admin lead member viewer"`
}

type teamRequest struct {
	TeamID uuid.UUID `json:"team_id" validate:"required"`
}

var (
	errEmailConflict = apperr.New(http.StatusConflict, "email_conflict", "El email ya pertenece a otro usuario")
	errLastAdmin     = apperr.New(http.StatusConflict, "last_admin", "No puede quedarse el sistema sin ningún administrador activo")
	errIdPManaged    = apperr.New(http.StatusConflict, "idp_managed", "Esta asignación se gestiona en el proveedor de identidad")
	errOffboarded    = apperr.New(http.StatusConflict, "user_offboarded", "El usuario está dado de baja")
	errTeamArchived  = apperr.New(http.StatusConflict, "team_archived", "El equipo está archivado")
	errLocalDisabled = apperr.New(http.StatusForbidden, "local_auth_disabled", "El login con contraseña no está habilitado")
)
