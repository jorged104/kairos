package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"openscum/internal/accounts"
	"openscum/internal/attributes"
	"openscum/internal/audit"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/internal/catalog"
	"openscum/internal/comments"
	"openscum/internal/config"
	"openscum/internal/customfields"
	"openscum/internal/dashboard"
	"openscum/internal/db"
	"openscum/internal/events"
	"openscum/internal/files"
	"openscum/internal/gamification"
	"openscum/internal/health"
	"openscum/internal/idpmappings"
	"openscum/internal/mcp"
	"openscum/internal/meetings"
	"openscum/internal/mentions"
	"openscum/internal/milestones"
	"openscum/internal/notifications"
	"openscum/internal/objectives"
	"openscum/internal/pages"
	"openscum/internal/portfolio"
	"openscum/internal/projectboard"
	"openscum/internal/projects"
	"openscum/internal/realtime"
	"openscum/internal/schedule"
	"openscum/internal/settings"
	"openscum/internal/teams"
	"openscum/internal/tracking"
	"openscum/internal/users"
	"openscum/internal/views"
	"openscum/internal/workitems"
)

// App wires every module behind one router. main and the integration tests
// build it the same way.
type App struct {
	Handler  http.Handler
	Sessions *auth.Sessions
	Loader   *auth.PrincipalLoader
	// Hub must be started with Hub.Run.
	Hub *realtime.Hub
	// Events must be started with Events.Run.
	Events *events.Dispatcher
	// Settings changes the settings of the system; tests replace its checks.
	Settings *settings.Service
	// MCP serves the AI connections (spec 016); tests lower its rate limit.
	MCP *mcp.Server
	// Accounts sends the invitations and links (spec 019); tests wait for them.
	Accounts *accounts.Service
}

// live holds the settings of the system edited from the interface (spec 015);
// the services read it on every use, so a change needs no restart.
func NewApp(cfg config.Config, store *db.Store, live *settings.Holder, logger *slog.Logger) *App {
	storage := files.NewLiveStorage(live.Files)
	sessions := auth.NewSessions(cfg.Auth.SessionSecret, cfg.Auth.CookieSecure)
	loader := auth.NewPrincipalLoader(store, auth.PrincipalTTL)

	router := NewRouter(logger)
	health.NewHandler(health.NewService(health.NewRepository(store.Queries), health.DatabaseTimeout), logger).Routes(router)

	hasher := auth.NewHasher(cfg.Auth.PasswordHashMemory, cfg.Auth.PasswordHashTime)
	authSvc := auth.NewService(live.Auth, store, loader, hasher, auth.NewRateLimiter(), logger)
	authH := auth.NewHandler(live.Auth, authSvc, store, sessions, logger)
	oidcH := auth.NewOIDC(live.OIDC, live.Auth, store, authSvc, sessions, logger)
	authH.SetEndSessionURL(func(ctx context.Context) string {
		if !live.Auth().OIDCEnabled() {
			return ""
		}
		return oidcH.EndSessionURL(ctx)
	})
	settingsSvc := settings.NewService(live)
	settingsH := settings.NewHandler(settingsSvc, logger)
	authH.SetTheme(func() any { return live.Theme() })
	oidcH.SetTested(func(ctx context.Context, email, name string, err error) error {
		t := settings.SSOTest{OK: err == nil, Email: email, Name: name}
		if err != nil {
			t.Email, t.Name, t.Error = "", "", err.Error()
		}
		return settingsSvc.RecordSSOTest(ctx, t)
	})
	mappingsH := idpmappings.NewHandler(store, func() string { return live.OIDC().Issuer }, logger)
	auditH := audit.NewHandler(store, logger)
	eventsH := events.NewHandler(store, logger)
	catalogH := catalog.NewHandler(catalog.NewService(store), logger)
	scheduleH := schedule.NewHandler(schedule.NewService(store), logger)
	portfolioH := portfolio.NewHandler(portfolio.NewService(store.Queries), logger)
	trackingH := tracking.NewHandler(tracking.NewService(store.Queries), logger)
	dashboardH := dashboard.NewHandler(dashboard.NewService(store.Queries), logger)
	boardH := projectboard.NewHandler(projectboard.NewService(store.Queries), projectboard.NewViews(store), logger)
	fieldsH := customfields.NewHandler(customfields.NewService(store), logger)
	filesSvc := files.NewService(store, storage)
	projectsH := projects.NewHandler(projects.NewService(store, filesSvc).OnRolesChanged(loader.Invalidate), logger)
	workItemsSvc := workitems.NewService(store)
	workItemsH := workitems.NewHandler(workItemsSvc, logger)
	viewsH := views.NewHandler(views.NewService(store, workItemsSvc), logger)
	milestonesH := milestones.NewHandler(milestones.NewService(store), logger)
	meetingsH := meetings.NewHandler(meetings.NewService(store), logger)
	objectivesH := objectives.NewHandler(objectives.NewService(store), logger)
	pagesH := pages.NewHandler(pages.NewService(store, filesSvc), logger)
	mentionsH := mentions.NewHandler(store, logger)
	commentsH := comments.NewHandler(comments.NewService(store, filesSvc), logger)
	hub := realtime.NewHub(sessions, loader, cfg.Auth.PublicURL, logger)
	notificationsSvc := notifications.NewService(store, hub, logger)
	notificationsH := notifications.NewHandler(notificationsSvc, logger)
	dispatcher := events.NewDispatcher(store, logger, time.Second)
	dispatcher.Register(gamification.ConsumerName, gamification.Consumer(store))
	dispatcher.Register(notifications.ConsumerName, notifications.Consumer(store, notificationsSvc, loader, logger))
	gamificationH := gamification.NewHandler(gamification.NewService(store), logger)
	teamsH := teams.NewHandler(teams.NewService(store, loader), logger)
	attributesH := attributes.NewHandler(attributes.NewService(store), logger)
	accountsSvc := accounts.NewService(store, hasher, live.Auth, live.Mail, logger)
	accountsH := accounts.NewHandler(accountsSvc, logger)
	usersH := users.NewHandler(users.NewService(live.Config, store, authSvc, accountsSvc, loader), logger)

	mcpConns := mcp.NewConnections(store, live.MCPEnabled, func() string { return cfg.Auth.PublicURL })
	mcpH := mcp.NewHandler(mcpConns, logger)
	mcpSrv := mcp.NewServer(mcpConns, loader, live.MCPEnabled, logger)
	mcpSrv.SetAPI(router)

	router.Get("/ws", hub.ServeWS)

	router.Route("/api", func(api chi.Router) {
		api.Use(auth.CSRF(cfg.Auth.PublicURL, logger))
		authH.PublicRoutes(api)
		oidcH.Routes(api)
		accountsH.PublicRoutes(api)
		// The secret URL is the credential of an AI connection (spec 016).
		api.Handle("/mcp/{secret}", mcpSrv)

		api.Group(func(p chi.Router) {
			p.Use(auth.RequireSession(sessions, loader, logger))
			authH.Routes(p)
			teamsH.Routes(p)
			attributesH.Routes(p)
			usersH.Routes(p)
			catalogH.Routes(p)
			scheduleH.Routes(p)
			portfolioH.Routes(p)
			trackingH.Routes(p)
			boardH.Routes(p)
			dashboardH.Routes(p)
			gamificationH.Routes(p)
			settingsH.Routes(p)
			fieldsH.Routes(p)
			projectsH.Routes(p)
			workItemsH.Routes(p)
			viewsH.Routes(p)
			milestonesH.Routes(p)
			meetingsH.Routes(p)
			objectivesH.Routes(p)
			pagesH.Routes(p)
			mentionsH.Routes(p)
			commentsH.Routes(p)
			mcpH.Routes(p)
			notificationsH.Routes(p)
			p.With(auth.RequirePermission(authz.AuditRead, logger)).Group(auditH.Routes)
			p.With(auth.RequirePermission(authz.AuditRead, logger)).Group(eventsH.Routes)
			p.With(auth.RequirePermission(authz.IdpMappingsManage, logger)).Group(mappingsH.Routes)
		})
	})

	return &App{Handler: router, Sessions: sessions, Loader: loader, Hub: hub, Events: dispatcher, Settings: settingsSvc, MCP: mcpSrv, Accounts: accountsSvc}
}
