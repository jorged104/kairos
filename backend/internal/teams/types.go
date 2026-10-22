// Package teams manages the team hierarchy and its leaders (RF-88 to RF-93).
package teams

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"openscum/internal/apperr"
)

type Leader struct {
	UserID uuid.UUID `json:"user_id"`
	Name   string    `json:"name"`
}

type Team struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	ParentID    *uuid.UUID `json:"parent_id"`
	ExternalRef *string    `json:"external_ref"`
	ArchivedAt  *time.Time `json:"archived_at"`
	Leaders     []Leader   `json:"leaders"`
	MemberCount int64      `json:"member_count"`
}

// teamRequest is used to create a team and to replace its editable fields.
type teamRequest struct {
	Name        string     `json:"name" validate:"required,max=200"`
	ParentID    *uuid.UUID `json:"parent_id"`
	ExternalRef *string    `json:"external_ref" validate:"omitempty,max=500"`
}

func (r *teamRequest) Normalize() {
	r.Name = strings.TrimSpace(r.Name)
	if r.ExternalRef != nil {
		ref := strings.TrimSpace(*r.ExternalRef)
		r.ExternalRef = &ref
		if ref == "" {
			r.ExternalRef = nil
		}
	}
}

type leaderRequest struct {
	UserID uuid.UUID `json:"user_id" validate:"required"`
}

var (
	errNameConflict  = apperr.New(http.StatusConflict, "team_name_conflict", "Ya existe un equipo con ese nombre")
	errCycle         = apperr.New(http.StatusConflict, "team_cycle", "Un equipo no puede colgar de sí mismo ni de un subequipo suyo")
	errArchived      = apperr.New(http.StatusConflict, "team_archived", "El equipo está archivado")
	errHasChildren   = apperr.New(http.StatusConflict, "team_has_children", "El equipo tiene subequipos sin archivar")
	errLeaderNotLead = apperr.New(http.StatusConflict, "leader_not_lead", "Solo un usuario con rol lead puede liderar un equipo")
)

// NormalizeName compares team names ignoring case and outer spaces (RF-89).
func NormalizeName(name string) string { return strings.ToLower(strings.TrimSpace(name)) }
