// Package apperr carries domain errors with their HTTP status and error code
// from the services to the handlers.
package apperr

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"

	"openscum/pkg/httpx"
)

// Error is an expected failure the client can act on.
type Error struct {
	Status  int
	Code    string
	Message string
	Details map[string]string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// WithDetail returns a copy of e with one more detail.
func (e *Error) WithDetail(key, value string) *Error {
	c := *e
	c.Details = maps.Clone(e.Details)
	if c.Details == nil {
		c.Details = map[string]string{}
	}
	c.Details[key] = value
	return &c
}

var (
	ErrUnauthenticated = New(http.StatusUnauthorized, "unauthenticated", "Se requiere iniciar sesión")
	ErrForbidden       = New(http.StatusForbidden, "forbidden", "No tienes permiso para esta operación")
	ErrAccountInactive = New(http.StatusForbidden, "account_inactive", "La cuenta no está activa")
	ErrNotFound        = New(http.StatusNotFound, "not_found", "Recurso no encontrado")
	ErrValidation      = New(http.StatusBadRequest, "validation_failed", "Datos no válidos")
	ErrInvalidFilter   = New(http.StatusBadRequest, "invalid_filter", "Filtro no válido")
)

// Write sends err in the common format; unexpected errors become a logged 500.
func Write(w http.ResponseWriter, logger *slog.Logger, err error) {
	var e *Error
	if errors.As(err, &e) {
		httpx.WriteErrorDetails(w, e.Status, e.Code, e.Message, e.Details)
		return
	}
	logger.Error("error interno", slog.String("error", err.Error()))
	httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "Error interno del servidor")
}

const maxBodyBytes = 1 << 20

var validate = newValidator()

func newValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" {
			return ""
		}
		return name
	})
	return v
}

// Decode reads a JSON body into v and validates its `validate` tags.
func Decode(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes))
	if err := dec.Decode(v); err != nil {
		return ErrValidation.WithDetail("body", "json")
	}
	return Validate(v)
}

// Validate checks the `validate` tags of v and reports the first bad field.
func Validate(v any) error {
	err := validate.Struct(v)
	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) && len(verrs) > 0 {
		return ErrValidation.WithDetail("field", verrs[0].Field())
	}
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	return nil
}
