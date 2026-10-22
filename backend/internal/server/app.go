package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"openscum/internal/attributes"
	"openscum/internal/audit"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/internal/config"
	"openscum/internal/db"
	"openscum/internal/health"
	"openscum/internal/idpmappings"
	"openscum/internal/teams"
	"openscum/internal/users"
)

// App wires every module behind one router. main and the integration tests
// build it the same way.
type App struct {
	Handler  http.Handler
	Sessions *auth.Sessions
	Loader   *auth.PrincipalLoader
}

func NewApp(cfg config.Config, store *db.Store, logger *slog.Logger) *App {
	sessions := auth.NewSessions(cfg.Auth.SessionSecret, cfg.Auth.CookieSecure)
	loader := auth.NewPrincipalLoader(store, auth.PrincipalTTL)

	router := NewRouter(logger)
	health.NewHandler(health.NewService(health.NewRepository(store.Queries), health.DatabaseTimeout), logger).Routes(router)

	hasher := auth.NewHasher(cfg.Auth.PasswordHashMemory, cfg.Auth.PasswordHashTime)
	authSvc := auth.NewService(cfg.Auth, store, loader, hasher, auth.NewRateLimiter(), logger)
	authH := auth.NewHandler(cfg.Auth, authSvc, store, sessions, logger)
	oidcH := auth.NewOIDC(cfg.OIDC, cfg.Auth, store, authSvc, sessions, logger)
	if cfg.Auth.OIDCEnabled() {
		authH.SetEndSessionURL(oidcH.EndSessionURL)
	}
	mappingsH := idpmappings.NewHandler(store, cfg.OIDC.Issuer, logger)
	auditH := audit.NewHandler(store, logger)
	teamsH := teams.NewHandler(teams.NewService(store, loader), logger)
	attributesH := attributes.NewHandler(attributes.NewService(store), logger)
	usersH := users.NewHandler(users.NewService(cfg, store, authSvc, hasher, loader), logger)

	router.Route("/api", func(api chi.Router) {
		api.Use(auth.CSRF(cfg.Auth.PublicURL, logger))
		authH.PublicRoutes(api)
		oidcH.Routes(api)

		api.Group(func(p chi.Router) {
			p.Use(auth.RequireSession(sessions, loader, logger))
			authH.Routes(p)
			teamsH.Routes(p)
			attributesH.Routes(p)
			usersH.Routes(p)
			p.With(auth.RequirePermission(authz.AuditRead, logger)).Group(auditH.Routes)
			p.With(auth.RequirePermission(authz.IdpMappingsManage, logger)).Group(mappingsH.Routes)
		})
	})

	return &App{Handler: router, Sessions: sessions, Loader: loader}
}
