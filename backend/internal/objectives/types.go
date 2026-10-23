// Package objectives manages objectives and key results (OKR), their links to
// projects, milestones and work items, and their progress (RF-97 to RF-115).
package objectives

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"openscum/internal/apperr"
	"openscum/internal/workcore"
)

const (
	ScopeOrganization = "organizacion"
	ScopeTeam         = "equipo"
	ScopeProject      = "proyecto"

	StatusDraft     = "borrador"
	StatusActive    = "activo"
	StatusClosed    = "cerrado"
	StatusCancelled = "cancelado"

	ModeManual  = "manual"
	ModeDerived = "derivado"
)

type Link struct {
	Kind  string    `json:"kind"` // work_item or milestone
	ID    uuid.UUID `json:"id"`
	Label string    `json:"label"` // KAI-42, or the project identifier for milestones
	Name  string    `json:"name"`
}

type KeyResult struct {
	ID           uuid.UUID `json:"id"`
	Title        string    `json:"title"`
	Unit         string    `json:"unit"`
	StartValue   float64   `json:"start_value"`
	TargetValue  float64   `json:"target_value"`
	CurrentValue float64   `json:"current_value"`
	CalcMode     string    `json:"calc_mode"`
	Position     int32     `json:"position"`
	Version      int32     `json:"version"`
	Progress     int       `json:"progress"`
	LinkedTotal  int32     `json:"linked_total"`
	LinkedDone   int32     `json:"linked_done"`
	Links        []Link    `json:"links"`
}

type ProjectLink struct {
	ID         uuid.UUID `json:"id"`
	Identifier string    `json:"identifier"`
	Name       string    `json:"name"`
	Lifecycle  string    `json:"lifecycle"`
}

type MilestoneLink struct {
	ID          uuid.UUID      `json:"id"`
	Name        string         `json:"name"`
	Status      string         `json:"status"`
	DueDate     *workcore.Date `json:"due_date"`
	CompletedOn *workcore.Date `json:"completed_on"`
	Project     ProjectLink    `json:"project"`
}

type Objective struct {
	ID           uuid.UUID       `json:"id"`
	Title        string          `json:"title"`
	Description  json.RawMessage `json:"description"`
	Owner        workcore.Ref    `json:"owner"`
	Scope        string          `json:"scope"`
	ScopeTeam    *workcore.Ref   `json:"scope_team"`
	ScopeProject *workcore.Ref   `json:"scope_project"`
	PeriodStart  workcore.Date   `json:"period_start"`
	PeriodEnd    workcore.Date   `json:"period_end"`
	Status       string          `json:"status"`
	Version      int32           `json:"version"`
	Progress     int             `json:"progress"`
	CreatedAt    time.Time       `json:"created_at"`
	KeyResults   []KeyResult     `json:"key_results"`
	Projects     []ProjectLink   `json:"projects"`
	Milestones   []MilestoneLink `json:"milestones"`
	Can          struct {
		Manage bool `json:"manage"`
	} `json:"can"`
}

// Summary is a row of the objective list.
type Summary struct {
	ID          uuid.UUID     `json:"id"`
	Title       string        `json:"title"`
	Owner       workcore.Ref  `json:"owner"`
	Scope       string        `json:"scope"`
	PeriodStart workcore.Date `json:"period_start"`
	PeriodEnd   workcore.Date `json:"period_end"`
	Status      string        `json:"status"`
	Progress    int           `json:"progress"`
}

type createRequest struct {
	Title          string          `json:"title" validate:"required,max=255"`
	Description    json.RawMessage `json:"description"`
	OwnerID        *uuid.UUID      `json:"owner_id"`
	Scope          string          `json:"scope" validate:"required,oneof=organizacion equipo proyecto"`
	ScopeTeamID    *uuid.UUID      `json:"scope_team_id"`
	ScopeProjectID *uuid.UUID      `json:"scope_project_id"`
	PeriodStart    workcore.Date   `json:"period_start" validate:"required"`
	PeriodEnd      workcore.Date   `json:"period_end" validate:"required"`
}

func (r *createRequest) Normalize() { r.Title = strings.TrimSpace(r.Title) }

type updateRequest struct {
	Version     int32           `json:"version" validate:"required,min=1"`
	Title       string          `json:"title" validate:"required,max=255"`
	Description json.RawMessage `json:"description"`
	OwnerID     uuid.UUID       `json:"owner_id" validate:"required"`
	PeriodStart workcore.Date   `json:"period_start" validate:"required"`
	PeriodEnd   workcore.Date   `json:"period_end" validate:"required"`
}

func (r *updateRequest) Normalize() { r.Title = strings.TrimSpace(r.Title) }

type versionRequest struct {
	Version int32 `json:"version" validate:"required,min=1"`
}

type keyResultRequest struct {
	Version      int32    `json:"version"`
	Title        string   `json:"title" validate:"required,max=255"`
	Unit         string   `json:"unit" validate:"max=50"`
	StartValue   float64  `json:"start_value"`
	TargetValue  float64  `json:"target_value"`
	CurrentValue *float64 `json:"current_value"`
	CalcMode     string   `json:"calc_mode" validate:"required,oneof=manual derivado"`
	Position     int32    `json:"position"`
}

func (r *keyResultRequest) Normalize() {
	r.Title = strings.TrimSpace(r.Title)
	r.Unit = strings.TrimSpace(r.Unit)
}

type linkRequest struct {
	ID uuid.UUID `json:"id" validate:"required"`
}

var (
	errInvalidOwner   = apperr.New(http.StatusBadRequest, "invalid_owner", "El responsable debe estar activo y no ser solo observador")
	errDateRange      = apperr.New(http.StatusBadRequest, "invalid_date_range", "La fecha de fin es anterior a la de inicio")
	errNoKeyResults   = apperr.New(http.StatusConflict, "key_result_required", "Un objetivo activo necesita al menos un resultado clave")
	errTargetEqual    = apperr.New(http.StatusBadRequest, "key_result_target_equal", "La meta no puede ser igual al valor inicial")
	errDerived        = apperr.New(http.StatusBadRequest, "key_result_derived", "El valor actual de un resultado clave derivado se calcula solo")
	errNotInObjective = apperr.New(http.StatusBadRequest, "milestone_not_in_objective", "El hito no contribuye al objetivo")
	errStatus         = apperr.New(http.StatusConflict, "invalid_objective_status", "El objetivo no está en un estado que permita esa acción")
	errReadOnly       = apperr.New(http.StatusConflict, "objective_closed", "El objetivo está cerrado o cancelado")
	errTemplate       = apperr.New(http.StatusBadRequest, "not_a_project", "Las plantillas no se ligan a objetivos")
)
