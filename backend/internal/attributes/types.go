// Package attributes manages the dynamic user attributes (RF-94 to RF-108).
package attributes

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"openscum/internal/apperr"
	"openscum/internal/fields"
)

const (
	TypeText    = fields.TypeText
	TypeNumber  = fields.TypeNumber
	TypeDate    = fields.TypeDate
	TypeBoolean = fields.TypeBoolean
	TypeSelect  = fields.TypeSelect
)

// Option is a choice of a select attribute. Retired options are kept for the
// users that already have them but cannot be chosen again (RF-107).
type Option = fields.Option

type Attribute struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	Required  bool       `json:"required"`
	Options   []Option   `json:"options"`
	RetiredAt *time.Time `json:"retired_at"`
}

func (a Attribute) Retired() bool { return a.RetiredAt != nil }

type createRequest struct {
	Name     string   `json:"name" validate:"required,max=200"`
	Type     string   `json:"type" validate:"required,oneof=text number date boolean select"`
	Required bool     `json:"required"`
	Options  []Option `json:"options" validate:"dive"`
}

func (r *createRequest) Normalize() {
	r.Name = strings.TrimSpace(r.Name)
	for i := range r.Options {
		r.Options[i].Value = strings.TrimSpace(r.Options[i].Value)
	}
}

type updateRequest = createRequest

var (
	errNameConflict = apperr.New(http.StatusConflict, "attribute_name_conflict", "Ya existe un atributo activo con ese nombre")
	errTypeLocked   = apperr.New(http.StatusConflict, "attribute_type_locked", "No se puede cambiar el tipo de un atributo con valores")
	errOptionNeeded = fields.ErrOptionNeeded
	errRetired      = apperr.New(http.StatusConflict, "attribute_retired", "El atributo está retirado")
	ErrInvalidValue = apperr.New(http.StatusBadRequest, "attribute_invalid_value", "El valor no es válido para el atributo")
	ErrRequired     = apperr.New(http.StatusBadRequest, "attribute_required", "Falta un atributo obligatorio")
)

// NormalizeName compares attribute names ignoring case and outer spaces (RF-101).
func NormalizeName(name string) string { return fields.NormalizeName(name) }
