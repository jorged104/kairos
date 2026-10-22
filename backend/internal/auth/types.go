package auth

import (
	"strings"

	"github.com/google/uuid"

	"openscum/internal/authz"
)

type RoleAssignment struct {
	Role   string `json:"role"`
	Source string `json:"source"`
}

type TeamAssignment struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Source string    `json:"source"`
}

// MeResponse is the session user's own view (RF-50).
type MeResponse struct {
	ID          uuid.UUID          `json:"id"`
	Name        string             `json:"name"`
	Email       string             `json:"email"`
	Status      string             `json:"status"`
	AuthMethod  string             `json:"auth_method"`
	HasPassword bool               `json:"has_password"`
	Roles       []RoleAssignment   `json:"roles"`
	Teams       []TeamAssignment   `json:"teams"`
	Permissions []authz.Permission `json:"permissions"`
}

// ConfigResponse tells the login screen which methods are enabled (RF-6, RF-12).
type ConfigResponse struct {
	LocalEnabled        bool `json:"local_enabled"`
	OIDCEnabled         bool `json:"oidc_enabled"`
	RegistrationEnabled bool `json:"registration_enabled"`
}

type loginRequest struct {
	Email    string `json:"email" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type registerRequest struct {
	Name     string `json:"name" validate:"required,max=200"`
	Email    string `json:"email" validate:"required,email,max=320"`
	Password string `json:"password"`
}

func (r *registerRequest) Normalize() {
	r.Name = strings.TrimSpace(r.Name)
	r.Email = strings.TrimSpace(r.Email)
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password"`
}

type logoutResponse struct {
	RedirectURL string `json:"redirect_url"`
}
