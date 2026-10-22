package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// PathUUID reads a UUID route parameter; ok is false if it is malformed.
func PathUUID(r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	return id, err == nil
}
