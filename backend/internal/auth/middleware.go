package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"openscum/internal/apperr"
	"openscum/internal/authz"
)

// RequireSession authenticates the request with the session cookie and puts
// the principal, loaded from the database, in the context (RF-42 to RF-45).
func RequireSession(sessions *Sessions, loader *PrincipalLoader, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, err := sessions.Parse(r)
			if err != nil {
				apperr.Write(w, logger, apperr.ErrUnauthenticated)
				return
			}
			p, err := loader.Load(r.Context(), id)
			if errors.Is(err, ErrUserNotFound) {
				apperr.Write(w, logger, apperr.ErrUnauthenticated)
				return
			}
			if err != nil {
				apperr.Write(w, logger, err)
				return
			}
			if p.Status != "active" {
				apperr.Write(w, logger, apperr.ErrAccountInactive)
				return
			}
			next.ServeHTTP(w, r.WithContext(authz.WithPrincipal(r.Context(), p)))
		})
	}
}

// RequirePermission rejects requests whose principal lacks perm (RF-61).
func RequirePermission(perm authz.Permission, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := authz.FromContext(r.Context())
			if p == nil || !p.Has(perm) {
				apperr.Write(w, logger, apperr.ErrForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

var errCSRF = apperr.New(http.StatusForbidden, "csrf_rejected", "Origen de la petición no permitido")

// CSRF rejects state-changing requests whose Origin (or Referer) is not the
// configured frontend.
func CSRF(publicURL string, logger *slog.Logger) func(http.Handler) http.Handler {
	allowed, _ := url.Parse(publicURL)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}
			if !sameOrigin(allowed, r) {
				apperr.Write(w, logger, errCSRF)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func sameOrigin(allowed *url.URL, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = r.Header.Get("Referer")
	}
	u, err := url.Parse(origin)
	if origin == "" || err != nil {
		return false
	}
	return u.Scheme == allowed.Scheme && u.Host == allowed.Host
}
