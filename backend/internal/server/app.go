package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"openscum/internal/audit"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/internal/config"
	"openscum/internal/db"
	"openscum/internal/health"
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

	authH := auth.NewHandler(cfg.Auth, store, sessions, logger)
	auditH := audit.NewHandler(store, logger)

	router.Route("/api", func(api chi.Router) {
		api.Use(auth.CSRF(cfg.Auth.PublicURL, logger))
		authH.PublicRoutes(api)

		api.Group(func(p chi.Router) {
			p.Use(auth.RequireSession(sessions, loader, logger))
			authH.Routes(p)
			p.With(auth.RequirePermission(authz.AuditRead, logger)).Group(auditH.Routes)
		})
	})

	return &App{Handler: router, Sessions: sessions, Loader: loader}
}
