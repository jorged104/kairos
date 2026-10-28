// Package catalog manages the global configuration of spec 004: methodologies
// and their phases, work item types, statuses, priorities and workflows
// (RF-1 to RF-11).
package catalog

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"openscum/internal/apperr"
)

type Phase struct {
	ID            uuid.UUID `json:"id"`
	MethodologyID uuid.UUID `json:"methodology_id"`
	Name          string    `json:"name"`
	Position      int32     `json:"position"`
	HasStartGate  bool      `json:"has_start_gate"`
	HasFinishGate bool      `json:"has_finish_gate"`
	Retired       bool      `json:"retired"`
}

type Methodology struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Position int32     `json:"position"`
	Retired  bool      `json:"retired"`
	Phases   []Phase   `json:"phases"`
}

type Type struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Color         string    `json:"color"`
	Position      int32     `json:"position"`
	DefaultWeight int16     `json:"default_weight"`
	Retired       bool      `json:"retired"`
}

type Status struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	Position  int32     `json:"position"`
	IsClosed  bool      `json:"is_closed"`
	IsDefault bool      `json:"is_default"`
	Retired   bool      `json:"retired"`
}

type Priority struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	Position  int32     `json:"position"`
	IsDefault bool      `json:"is_default"`
	Retired   bool      `json:"retired"`
}

type Transition struct {
	FromStatusID uuid.UUID `json:"from_status_id" validate:"required"`
	ToStatusID   uuid.UUID `json:"to_status_id" validate:"required"`
	Role         *string   `json:"role" validate:"omitempty,oneof=admin lead member viewer"`
}

// Catalog is everything the forms need, retired elements included.
type Catalog struct {
	Methodologies []Methodology `json:"methodologies"`
	Types         []Type        `json:"work_item_types"`
	Statuses      []Status      `json:"statuses"`
	Priorities    []Priority    `json:"priorities"`
}

type methodologyRequest struct {
	Name     string `json:"name" validate:"required,max=100"`
	Position int32  `json:"position" validate:"min=0"`
}

func (r *methodologyRequest) Normalize() { r.Name = strings.TrimSpace(r.Name) }

type phaseRequest struct {
	Name          string `json:"name" validate:"required,max=100"`
	Position      int32  `json:"position" validate:"min=0"`
	HasStartGate  bool   `json:"has_start_gate"`
	HasFinishGate bool   `json:"has_finish_gate"`
}

func (r *phaseRequest) Normalize() { r.Name = strings.TrimSpace(r.Name) }

type typeRequest struct {
	Name     string `json:"name" validate:"required,max=100"`
	Color    string `json:"color" validate:"required,len=7,hexcolor"`
	Position int32  `json:"position" validate:"min=0"`
	// Copied to new work items of the type (RF-150, RF-151); absent keeps it.
	DefaultWeight *int16 `json:"default_weight" validate:"omitempty,min=0,max=100"`
}

func (r *typeRequest) Normalize() {
	r.Name = strings.TrimSpace(r.Name)
	r.Color = strings.ToLower(strings.TrimSpace(r.Color))
}

type statusRequest struct {
	Name      string `json:"name" validate:"required,max=100"`
	Color     string `json:"color" validate:"required,len=7,hexcolor"`
	Position  int32  `json:"position" validate:"min=0"`
	IsClosed  bool   `json:"is_closed"`
	IsDefault bool   `json:"is_default"`
}

func (r *statusRequest) Normalize() {
	r.Name = strings.TrimSpace(r.Name)
	r.Color = strings.ToLower(strings.TrimSpace(r.Color))
}

type priorityRequest struct {
	Name      string `json:"name" validate:"required,max=100"`
	Color     string `json:"color" validate:"required,len=7,hexcolor"`
	Position  int32  `json:"position" validate:"min=0"`
	IsDefault bool   `json:"is_default"`
}

func (r *priorityRequest) Normalize() {
	r.Name = strings.TrimSpace(r.Name)
	r.Color = strings.ToLower(strings.TrimSpace(r.Color))
}

type transitionsRequest struct {
	Transitions []Transition `json:"transitions" validate:"max=500,dive"`
}

var (
	errNameConflict    = apperr.New(http.StatusConflict, "catalog_name_conflict", "Ya existe un elemento activo con ese nombre")
	errDefaultRequired = apperr.New(http.StatusConflict, "catalog_default_required", "Debe haber un elemento por defecto activo")
	errClosedLocked    = apperr.New(http.StatusConflict, "status_closed_locked", "No se puede cambiar si un estado en uso está cerrado")
)

// NormalizeName compares names ignoring case and outer spaces (RF-8).
func NormalizeName(name string) string { return strings.ToLower(strings.TrimSpace(name)) }
