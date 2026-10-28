// Package workitems manages work items: numbering, assignment, composition
// with derived values, workflow and events, and dependencies (RF-61 to
// RF-96, RF-116 to RF-118, RF-129, RF-130, RF-133).
package workitems

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"openscum/internal/apperr"
	"openscum/internal/customfields"
	"openscum/internal/workcore"
)

const maxDepth = 9 // 10 levels (RF-78)

// Ref points at another work item by key.
type Ref struct {
	ID    uuid.UUID `json:"id"`
	Key   string    `json:"key"`
	Title string    `json:"title"`
}

type Derived struct {
	EstimatedHours float64 `json:"estimated_hours"`
	RemainingHours float64 `json:"remaining_hours"`
	DoneRatio      int16   `json:"done_ratio"`
}

type Can struct {
	Write  bool `json:"write"`
	Delete bool `json:"delete"`
}

type WorkItem struct {
	ID               uuid.UUID                     `json:"id"`
	Key              string                        `json:"key"`
	SequenceNumber   int32                         `json:"sequence_number"`
	Project          ProjectRef                    `json:"project"`
	TypeID           uuid.UUID                     `json:"type_id"`
	StatusID         uuid.UUID                     `json:"status_id"`
	PriorityID       uuid.UUID                     `json:"priority_id"`
	Title            string                        `json:"title"`
	Description      json.RawMessage               `json:"description"`
	Author           workcore.Ref                  `json:"author"`
	Assignee         *workcore.Ref                 `json:"assignee"`
	Parent           *Ref                          `json:"parent"`
	StartDate        *workcore.Date                `json:"start_date"`
	DueDate          *workcore.Date                `json:"due_date"`
	EstimatedHours   *float64                      `json:"estimated_hours"`
	RemainingHours   *float64                      `json:"remaining_hours"`
	DoneRatio        int16                         `json:"done_ratio"`
	Weight           int16                         `json:"weight"`
	Derived          Derived                       `json:"derived"`
	Position         int32                         `json:"position"`
	Depth            int16                         `json:"depth"`
	ChildCount       int64                         `json:"child_count"`
	Version          int32                         `json:"version"`
	ClosedAt         *time.Time                    `json:"closed_at"`
	CreatedAt        time.Time                     `json:"created_at"`
	UpdatedAt        time.Time                     `json:"updated_at"`
	CustomValues     map[uuid.UUID]json.RawMessage `json:"custom_values"`
	AllowedStatusIDs []uuid.UUID                   `json:"allowed_status_ids"`
	Dependencies     []Dependency                  `json:"dependencies"`
	Can              Can                           `json:"can"`
}

type ProjectRef struct {
	ID         uuid.UUID `json:"id"`
	Identifier string    `json:"identifier"`
	Name       string    `json:"name"`
}

// Dependency is seen from the work item that lists it: Direction is
// "outgoing" when it is the source and "incoming" when it is the target.
type Dependency struct {
	ID        uuid.UUID `json:"id"`
	Type      string    `json:"type"`
	Direction string    `json:"direction"`
	LagDays   int16     `json:"lag_days"`
	Other     Ref       `json:"other"`
	// DatesViolated is set for precedes when the dates break the order (RF-96).
	DatesViolated bool `json:"dates_violated"`
}

// Node is a row of the project tree (RF-130).
type Node struct {
	ID         uuid.UUID      `json:"id"`
	ParentID   *uuid.UUID     `json:"parent_id"`
	Key        string         `json:"key"`
	Title      string         `json:"title"`
	TypeID     uuid.UUID      `json:"type_id"`
	StatusID   uuid.UUID      `json:"status_id"`
	PriorityID uuid.UUID      `json:"priority_id"`
	Assignee   *workcore.Ref  `json:"assignee"`
	DoneRatio  int16          `json:"derived_done_ratio"`
	Estimated  float64        `json:"derived_estimated_hours"`
	Remaining  float64        `json:"derived_remaining_hours"`
	Depth      int16          `json:"depth"`
	Position   int32          `json:"position"`
	DueDate    *workcore.Date `json:"due_date"`
	Weight     int16          `json:"weight"`
}

type createRequest struct {
	TypeID         uuid.UUID           `json:"type_id" validate:"required"`
	Title          string              `json:"title" validate:"required,max=255"`
	Description    json.RawMessage     `json:"description"`
	ParentID       *uuid.UUID          `json:"parent_id"`
	StatusID       *uuid.UUID          `json:"status_id"`
	PriorityID     *uuid.UUID          `json:"priority_id"`
	AssigneeID     *uuid.UUID          `json:"assignee_id"`
	StartDate      *workcore.Date      `json:"start_date"`
	DueDate        *workcore.Date      `json:"due_date"`
	EstimatedHours *float64            `json:"estimated_hours"`
	RemainingHours *float64            `json:"remaining_hours"`
	DoneRatio      int16               `json:"done_ratio"`
	Position       int32               `json:"position"`
	CustomValues   customfields.Values `json:"custom_values" validate:"max=200"`
	// Absent takes the default weight of the type (create) or keeps it (update), RF-151.
	Weight *int16 `json:"weight"`
}

func (r *createRequest) Normalize() { r.Title = strings.TrimSpace(r.Title) }

// updateRequest replaces every editable field (the status change goes here too).
type updateRequest struct {
	Version        int32               `json:"version" validate:"required,min=1"`
	TypeID         uuid.UUID           `json:"type_id" validate:"required"`
	StatusID       uuid.UUID           `json:"status_id" validate:"required"`
	PriorityID     uuid.UUID           `json:"priority_id" validate:"required"`
	Title          string              `json:"title" validate:"required,max=255"`
	Description    json.RawMessage     `json:"description"`
	ParentID       *uuid.UUID          `json:"parent_id"`
	AssigneeID     *uuid.UUID          `json:"assignee_id"`
	StartDate      *workcore.Date      `json:"start_date"`
	DueDate        *workcore.Date      `json:"due_date"`
	EstimatedHours *float64            `json:"estimated_hours"`
	RemainingHours *float64            `json:"remaining_hours"`
	DoneRatio      int16               `json:"done_ratio"`
	Position       int32               `json:"position"`
	CustomValues   customfields.Values `json:"custom_values" validate:"max=200"`
	// Absent takes the default weight of the type (create) or keeps it (update), RF-151.
	Weight *int16 `json:"weight"`
}

func (r *updateRequest) Normalize() { r.Title = strings.TrimSpace(r.Title) }

type dependencyRequest struct {
	ToID    uuid.UUID `json:"to_id" validate:"required"`
	Type    string    `json:"type" validate:"required,oneof=blocks precedes relates duplicates"`
	LagDays int16     `json:"lag_days" validate:"min=-365,max=365"`
}

var (
	errDateRange       = apperr.New(http.StatusBadRequest, "invalid_date_range", "La fecha de fin es anterior a la de inicio")
	errOutOfRange      = apperr.New(http.StatusBadRequest, "field_out_of_range", "Valor fuera de rango")
	errInvalidAssignee = apperr.New(http.StatusBadRequest, "invalid_assignee", "El asignado debe estar activo y no ser solo observador")
	errTypeNotEnabled  = apperr.New(http.StatusBadRequest, "type_not_enabled", "El tipo no está activo en el proyecto")
	errCrossProject    = apperr.New(http.StatusBadRequest, "cross_project", "El elemento pertenece a otro proyecto")
	errCycle           = apperr.New(http.StatusConflict, "work_item_cycle", "Un work item no puede colgar de sí mismo ni de un descendiente")
	errTooDeep         = apperr.New(http.StatusConflict, "work_item_too_deep", "La jerarquía admite como máximo 10 niveles")
	errHasChildren     = apperr.New(http.StatusConflict, "has_children", "El work item tiene hijos")
	errTransition      = apperr.New(http.StatusConflict, "transition_not_allowed", "El flujo no permite ese cambio de estado")
	errBlocked         = apperr.New(http.StatusConflict, "blocked_by_open_item", "Lo bloquea un work item abierto")
	errDepConflict     = apperr.New(http.StatusConflict, "dependency_conflict", "La dependencia ya existe")
	errDepCycle        = apperr.New(http.StatusConflict, "dependency_cycle", "La dependencia cerraría un ciclo")
	errDepHierarchy    = apperr.New(http.StatusBadRequest, "dependency_hierarchy", "No se admiten dependencias con uno mismo, ancestros ni descendientes")
)
