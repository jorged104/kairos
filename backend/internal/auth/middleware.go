package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"openscum/internal/apperr"
	"openscum/internal/authz"
)

type internalKey struct{}

// Internal marks a request served inside the process for an AI connection
// (spec 016): its principal is already in the context, put there by the MCP
// server after checking the connection. Nothing outside the process can set it.
func Internal(ctx context.Context, p *authz.Principal) context.Context {
	return context.WithValue(authz.WithPrincipal(ctx, p), internalKey{}, true)
}

func isInternal(ctx context.Context) bool {
	v, _ := ctx.Value(internalKey{}).(bool)
	return v
}

// RequireSession authenticates the request with the session cookie and puts
// the principal, loaded from the database, in the context (RF-42 to RF-45).
func RequireSession(sessions *Sessions, loader *PrincipalLoader, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isInternal(r.Context()) && authz.FromContext(r.Context()) != nil {
				next.ServeHTTP(w, r)
				return
			}
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

// MCPPath is where the AI connections are served (spec 016).
const MCPPath = "/api/mcp/"

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
			// AI clients call from their own servers, with the secret URL as
			// the only credential: no cookie to protect (spec 016).
			if isInternal(r.Context()) || strings.HasPrefix(r.URL.Path, MCPPath) {
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
