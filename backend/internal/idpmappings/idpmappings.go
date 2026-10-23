// Package idpmappings manages how IdP roles and groups translate into Kairos
// roles and teams. Changes apply on each affected user's next login (RF-39).
package idpmappings

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
	"openscum/pkg/httpx"
)

type Mapping struct {
	ID            uuid.UUID `json:"id"`
	Issuer        string    `json:"issuer"`
	Claim         string    `json:"claim"`
	ExternalValue string    `json:"external_value"`
	TargetType    string    `json:"target_type"`
	Target        string    `json:"target"`
}

type request struct {
	Issuer        string `json:"issuer" validate:"max=500"`
	Claim         string `json:"claim" validate:"required,oneof=roles groups"`
	ExternalValue string `json:"external_value" validate:"required,max=500"`
	TargetType    string `json:"target_type" validate:"required,oneof=role team"`
	Target        string `json:"target" validate:"required"`
}

func (r *request) Normalize() {
	r.Issuer = strings.TrimSpace(r.Issuer)
	r.ExternalValue = strings.TrimSpace(r.ExternalValue)
	r.Target = strings.TrimSpace(r.Target)
}

var errDuplicate = apperr.New(http.StatusConflict, "idp_mapping_conflict", "La homologación ya existe")

type Handler struct {
	store *db.Store
	// issuer is used when a mapping is created without one: only one issuer
	// is configured in this iteration.
	issuer string
	logger *slog.Logger
}

func NewHandler(store *db.Store, issuer string, logger *slog.Logger) *Handler {
	return &Handler{store: store, issuer: issuer, logger: logger}
}

// Routes must be mounted behind the idp_mappings:manage permission.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/idp-mappings", h.List)
	r.Post("/idp-mappings", h.Create)
	r.Patch("/idp-mappings/{id}", h.Update)
	r.Delete("/idp-mappings/{id}", h.Delete)
}

func toMapping(m sqlc.IdpMapping) Mapping {
	return Mapping{ID: m.ID, Issuer: m.Issuer, Claim: m.Claim, ExternalValue: m.ExternalValue, TargetType: m.TargetType, Target: m.Target}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.store.ListIdpMappings(r.Context())
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	out := make([]Mapping, 0, len(rows))
	for _, m := range rows {
		out = append(out, toMapping(m))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": out, "default_issuer": h.issuer})
}

func (h *Handler) decode(r *http.Request) (request, error) {
	var req request
	if err := apperr.Decode(r, &req); err != nil {
		return req, err
	}
	if req.Issuer == "" {
		req.Issuer = h.issuer
	}
	if req.Issuer == "" {
		return req, apperr.ErrValidation.WithDetail("field", "issuer")
	}
	return req, checkTarget(r.Context(), h.store.Queries, req)
}

func checkTarget(ctx context.Context, q *sqlc.Queries, req request) error {
	invalid := apperr.ErrValidation.WithDetail("field", "target")
	if req.TargetType == "role" {
		if _, ok := authz.ParseRole(req.Target); !ok {
			return invalid
		}
		return nil
	}
	id, err := uuid.Parse(req.Target)
	if err != nil {
		return invalid
	}
	if _, err := q.GetTeam(ctx, id); errors.Is(err, pgx.ErrNoRows) {
		return invalid
	} else if err != nil {
		return fmt.Errorf("leer equipo: %w", err)
	}
	return nil
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	req, err := h.decode(r)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	var out Mapping
	err = h.store.InTx(r.Context(), func(_ pgx.Tx, q *sqlc.Queries) error {
		m, err := q.CreateIdpMapping(r.Context(), sqlc.CreateIdpMappingParams{
			Issuer: req.Issuer, Claim: req.Claim, ExternalValue: req.ExternalValue, TargetType: req.TargetType, Target: req.Target,
		})
		if auth.IsUniqueViolation(err) {
			return errDuplicate
		}
		if err != nil {
			return fmt.Errorf("crear homologación: %w", err)
		}
		out = toMapping(m)
		return audit.Record(r.Context(), q, audit.Entry{
			Actor: audit.ByUser(authz.FromContext(r.Context())), Action: "idp_mapping.created",
			TargetType: "idp_mapping", TargetID: m.ID.String(), After: out,
		})
	})
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, out)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrNotFound)
		return
	}
	req, err := h.decode(r)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	var out Mapping
	err = h.store.InTx(r.Context(), func(_ pgx.Tx, q *sqlc.Queries) error {
		before, err := q.GetIdpMapping(r.Context(), id)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("leer homologación: %w", err)
		}
		m, err := q.UpdateIdpMapping(r.Context(), sqlc.UpdateIdpMappingParams{
			ID: id, Claim: req.Claim, ExternalValue: req.ExternalValue, TargetType: req.TargetType, Target: req.Target,
		})
		if auth.IsUniqueViolation(err) {
			return errDuplicate
		}
		if err != nil {
			return fmt.Errorf("actualizar homologación: %w", err)
		}
		out = toMapping(m)
		return audit.Record(r.Context(), q, audit.Entry{
			Actor: audit.ByUser(authz.FromContext(r.Context())), Action: "idp_mapping.updated",
			TargetType: "idp_mapping", TargetID: id.String(), Before: toMapping(before), After: out,
		})
	})
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrNotFound)
		return
	}
	err := h.store.InTx(r.Context(), func(_ pgx.Tx, q *sqlc.Queries) error {
		before, err := q.GetIdpMapping(r.Context(), id)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("leer homologación: %w", err)
		}
		if _, err := q.DeleteIdpMapping(r.Context(), id); err != nil {
			return fmt.Errorf("borrar homologación: %w", err)
		}
		return audit.Record(r.Context(), q, audit.Entry{
			Actor: audit.ByUser(authz.FromContext(r.Context())), Action: "idp_mapping.deleted",
			TargetType: "idp_mapping", TargetID: id.String(), Before: toMapping(before),
		})
	})
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
