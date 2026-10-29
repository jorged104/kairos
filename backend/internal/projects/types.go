// Package projects manages projects, their hierarchy, lifecycle, enabled types
// and fields, phases and templates (RF-18 to RF-43, RF-128).
package projects

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"openscum/internal/apperr"
	"openscum/internal/customfields"
	"openscum/internal/workcore"
)

const (
	LifecycleActive   = "activo"
	LifecycleClosed   = "cerrado"
	LifecycleArchived = "archivado"

	maxDepth = 4 // 5 levels (RF-27)
)

var identifierRe = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}$`)

type ParentRef struct {
	ID         uuid.UUID `json:"id"`
	Identifier string    `json:"identifier"`
	Name       string    `json:"name"`
}

// Can lists what the caller may do with the project (RF-137).
type Can struct {
	Manage         bool `json:"manage"`
	WriteWorkItems bool `json:"write_work_items"`
	ManageTemplate bool `json:"manage_template"`
}

type Project struct {
	ID                uuid.UUID                     `json:"id"`
	Identifier        string                        `json:"identifier"`
	Name              string                        `json:"name"`
	Description       json.RawMessage               `json:"description"`
	PageVersion       int32                         `json:"page_version"` // compared by the autosave (spec 005)
	Team              workcore.Ref                  `json:"team"`
	Owner             workcore.Ref                  `json:"owner"`
	Methodology       *workcore.Ref                 `json:"methodology"`
	Parent            *ParentRef                    `json:"parent"`
	TemplateID        *uuid.UUID                    `json:"template_id"`
	IsTemplate        bool                          `json:"is_template"`
	Lifecycle         string                        `json:"lifecycle"`
	Health            string                        `json:"health"`
	HealthExplanation string                        `json:"health_explanation"`
	StartDate         *workcore.Date                `json:"start_date"`
	EndDate           *workcore.Date                `json:"end_date"`
	Depth             int16                         `json:"depth"`
	Version           int32                         `json:"version"`
	CreatedAt         time.Time                     `json:"created_at"`
	UpdatedAt         time.Time                     `json:"updated_at"`
	TypeIDs           []uuid.UUID                   `json:"work_item_type_ids"`
	CustomFieldIDs    []uuid.UUID                   `json:"custom_field_ids"`
	CustomValues      map[uuid.UUID]json.RawMessage `json:"custom_values"`
	Can               Can                           `json:"can"`
}

// Summary is a row of the project list.
type Summary struct {
	ID          uuid.UUID      `json:"id"`
	Identifier  string         `json:"identifier"`
	Name        string         `json:"name"`
	Team        workcore.Ref   `json:"team"`
	Owner       workcore.Ref   `json:"owner"`
	Methodology *workcore.Ref  `json:"methodology"`
	ParentID    *uuid.UUID     `json:"parent_id"`
	Lifecycle   string         `json:"lifecycle"`
	Health      string         `json:"health"`
	StartDate   *workcore.Date `json:"start_date"`
	EndDate     *workcore.Date `json:"end_date"`
	Depth       int16          `json:"depth"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Can         Can            `json:"can"`
}

type SubtreeNode struct {
	ID         uuid.UUID  `json:"id"`
	ParentID   *uuid.UUID `json:"parent_id"`
	Identifier string     `json:"identifier"`
	Name       string     `json:"name"`
	Lifecycle  string     `json:"lifecycle"`
	Health     string     `json:"health"`
	Depth      int16      `json:"depth"`
}

type Phase struct {
	ID                 uuid.UUID      `json:"id"`
	DefinitionID       uuid.UUID      `json:"definition_id"`
	Name               string         `json:"name"`
	Position           int32          `json:"position"`
	StartDate          *workcore.Date `json:"start_date"`
	FinishDate         *workcore.Date `json:"finish_date"`
	HasStartGate       bool           `json:"has_start_gate"`
	HasFinishGate      bool           `json:"has_finish_gate"`
	StartGatePassedAt  *time.Time     `json:"start_gate_passed_at"`
	StartGatePassedBy  *uuid.UUID     `json:"start_gate_passed_by"`
	FinishGatePassedAt *time.Time     `json:"finish_gate_passed_at"`
	FinishGatePassedBy *uuid.UUID     `json:"finish_gate_passed_by"`
}

type createRequest struct {
	Identifier        string              `json:"identifier" validate:"required,max=10"`
	Name              string              `json:"name" validate:"required,max=255"`
	Description       json.RawMessage     `json:"description"`
	TeamID            uuid.UUID           `json:"team_id" validate:"required"`
	OwnerID           *uuid.UUID          `json:"owner_id"`
	MethodologyID     *uuid.UUID          `json:"methodology_id"`
	ParentID          *uuid.UUID          `json:"parent_id"`
	StartDate         *workcore.Date      `json:"start_date"`
	EndDate           *workcore.Date      `json:"end_date"`
	Health            string              `json:"health" validate:"omitempty,oneof=en_curso en_riesgo fuera_de_curso"`
	HealthExplanation string              `json:"health_explanation" validate:"max=2000"`
	TypeIDs           []uuid.UUID         `json:"work_item_type_ids" validate:"max=100"`
	CustomFieldIDs    []uuid.UUID         `json:"custom_field_ids" validate:"max=200"`
	CustomValues      customfields.Values `json:"custom_values" validate:"max=200"`
}

func (r *createRequest) Normalize() {
	r.Identifier = strings.ToUpper(strings.TrimSpace(r.Identifier))
	r.Name = strings.TrimSpace(r.Name)
	r.HealthExplanation = strings.TrimSpace(r.HealthExplanation)
}

// updateRequest replaces every editable field; the identifier never changes (RF-21).
type updateRequest struct {
	Version           int32               `json:"version" validate:"required,min=1"`
	Identifier        *string             `json:"identifier"`
	Name              string              `json:"name" validate:"required,max=255"`
	Description       json.RawMessage     `json:"description"`
	TeamID            uuid.UUID           `json:"team_id" validate:"required"`
	OwnerID           uuid.UUID           `json:"owner_id" validate:"required"`
	ParentID          *uuid.UUID          `json:"parent_id"`
	StartDate         *workcore.Date      `json:"start_date"`
	EndDate           *workcore.Date      `json:"end_date"`
	Health            string              `json:"health" validate:"required,oneof=en_curso en_riesgo fuera_de_curso"`
	HealthExplanation string              `json:"health_explanation" validate:"max=2000"`
	CustomValues      customfields.Values `json:"custom_values" validate:"max=200"`
}

func (r *updateRequest) Normalize() {
	r.Name = strings.TrimSpace(r.Name)
	r.HealthExplanation = strings.TrimSpace(r.HealthExplanation)
}

type versionRequest struct {
	Version int32 `json:"version" validate:"required,min=1"`
}

type idsRequest struct {
	IDs []uuid.UUID `json:"ids" validate:"max=200"`
}

type phaseRequest struct {
	StartDate  *workcore.Date `json:"start_date"`
	FinishDate *workcore.Date `json:"finish_date"`
}

type gateRequest struct {
	Passed bool `json:"passed"`
}

var (
	errIdentifierInvalid   = apperr.New(http.StatusBadRequest, "identifier_invalid", "El identificador debe tener de 2 a 10 mayúsculas o dígitos y empezar por letra")
	errIdentifierConflict  = apperr.New(http.StatusConflict, "identifier_conflict", "Ya existe un proyecto con ese identificador")
	errIdentifierImmutable = apperr.New(http.StatusBadRequest, "identifier_immutable", "El identificador no se puede cambiar")
	errDateRange           = apperr.New(http.StatusBadRequest, "invalid_date_range", "La fecha de fin es anterior a la de inicio")
	errInvalidOwner        = apperr.New(http.StatusBadRequest, "invalid_owner", "El responsable debe estar activo y no ser solo observador")
	errCycle               = apperr.New(http.StatusConflict, "project_cycle", "El proyecto no puede colgar de sí mismo ni de un descendiente")
	errTooDeep             = apperr.New(http.StatusConflict, "project_too_deep", "La jerarquía de proyectos admite como máximo 5 niveles")
	errLifecycle           = apperr.New(http.StatusConflict, "invalid_lifecycle", "El proyecto no está en un estado que permita esa acción")
	errTypeInUse           = apperr.New(http.StatusConflict, "type_in_use", "El tipo tiene work items en el proyecto")
	// ErrClosed and ErrArchived are shared with the modules that write inside a project.
	ErrClosed   = apperr.New(http.StatusConflict, "project_closed", "El proyecto está cerrado")
	ErrArchived = apperr.New(http.StatusConflict, "project_archived", "El proyecto está archivado")
)
