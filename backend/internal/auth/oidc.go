package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-chi/chi/v5"
	"golang.org/x/oauth2"

	"openscum/internal/apperr"
	"openscum/internal/config"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
)

const (
	// IdPTimeout bounds every call to the IdP during login (RF-19).
	IdPTimeout = 5 * time.Second
	// StateTTL is how long a login may stay at the IdP (RF-17).
	StateTTL = 10 * time.Minute

	stateCookie = "kairos_oidc_state"
	statePath   = "/api/auth/oidc"
)

// OIDC is the backend side of the SSO login: Kairos is a confidential
// client that runs Authorization Code + PKCE and keeps its own session (BFF).
type OIDC struct {
	cfg      config.OIDCConfig
	authCfg  config.AuthConfig
	store    *db.Store
	svc      *Service
	sessions *Sessions
	logger   *slog.Logger
	client   *http.Client
	now      func() time.Time

	mu         sync.Mutex
	provider   *oidc.Provider
	endSession string
}

func NewOIDC(cfg config.OIDCConfig, authCfg config.AuthConfig, store *db.Store, svc *Service, sessions *Sessions, logger *slog.Logger) *OIDC {
	return &OIDC{
		cfg: cfg, authCfg: authCfg, store: store, svc: svc, sessions: sessions, logger: logger,
		client: &http.Client{Timeout: IdPTimeout}, now: time.Now,
	}
}

// Routes are public: the browser arrives here without a session.
func (o *OIDC) Routes(r chi.Router) {
	r.Get("/auth/oidc/start", o.Start)
	r.Get("/auth/oidc/callback", o.Callback)
}

// discover fetches the IdP metadata once; failures are retried on the next login.
func (o *OIDC) discover() (*oidc.Provider, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.provider != nil {
		return o.provider, nil
	}
	// The key set keeps this context for later fetches, so it must not be a
	// request context; the client timeout bounds every call instead.
	ctx := oidc.ClientContext(context.Background(), o.client)
	if o.cfg.DiscoveryURL != o.cfg.Issuer {
		ctx = oidc.InsecureIssuerURLContext(ctx, o.cfg.Issuer)
	}
	p, err := oidc.NewProvider(ctx, o.cfg.DiscoveryURL)
	if err != nil {
		return nil, fmt.Errorf("descubrir el IdP: %w", err)
	}
	var meta struct {
		EndSession string `json:"end_session_endpoint"`
	}
	_ = p.Claims(&meta)
	o.provider, o.endSession = p, meta.EndSession
	return p, nil
}

func (o *OIDC) oauth(p *oidc.Provider) *oauth2.Config {
	return &oauth2.Config{
		ClientID: o.cfg.ClientID, ClientSecret: o.cfg.ClientSecret, RedirectURL: o.cfg.RedirectURL,
		Endpoint: p.Endpoint(), Scopes: []string{oidc.ScopeOpenID, "profile", "email"},
	}
}

// Start redirects the browser to the IdP (RF-15).
func (o *OIDC) Start(w http.ResponseWriter, r *http.Request) {
	if !o.authCfg.OIDCEnabled() {
		o.fail(w, r, errOIDCDisabled)
		return
	}
	p, err := o.discover()
	if err != nil {
		o.logger.Warn("IdP no disponible", slog.String("error", err.Error()))
		o.fail(w, r, errIdPUnavailable)
		return
	}
	state, nonce, verifier := randomToken(), randomToken(), oauth2.GenerateVerifier()
	ctx := r.Context()
	if err := o.store.PurgeOIDCStates(ctx, o.now().Add(-StateTTL)); err != nil {
		o.fail(w, r, err)
		return
	}
	if err := o.store.InsertOIDCState(ctx, sqlc.InsertOIDCStateParams{
		State: state, Nonce: nonce, CodeVerifier: verifier, ReturnTo: safeReturnTo(r.URL.Query().Get("return_to")),
	}); err != nil {
		o.fail(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: stateCookie, Value: state, Path: statePath, MaxAge: int(StateTTL.Seconds()),
		HttpOnly: true, Secure: o.authCfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, o.oauth(p).AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)), http.StatusFound)
}

// Callback completes the login when the IdP sends the browser back (RF-16 to RF-39).
func (o *OIDC) Callback(w http.ResponseWriter, r *http.Request) {
	if !o.authCfg.OIDCEnabled() {
		o.fail(w, r, errOIDCDisabled)
		return
	}
	ctx := r.Context()
	q := r.URL.Query()
	http.SetCookie(w, &http.Cookie{Name: stateCookie, Path: statePath, MaxAge: -1, HttpOnly: true, Secure: o.authCfg.CookieSecure, SameSite: http.SameSiteLaxMode})

	// The state is checked before talking to the IdP (RF-17).
	saved, err := o.consumeState(ctx, r, q.Get("state"))
	if err != nil {
		o.fail(w, r, err)
		return
	}
	if e := q.Get("error"); e != "" {
		o.logger.Warn("el IdP devolvió un error", slog.String("error", e))
		o.fail(w, r, errIdPTokenInvalid)
		return
	}
	p, err := o.discover()
	if err != nil {
		o.logger.Warn("IdP no disponible", slog.String("error", err.Error()))
		o.fail(w, r, errIdPUnavailable)
		return
	}

	exCtx, cancel := context.WithTimeout(oidc.ClientContext(ctx, o.client), IdPTimeout)
	defer cancel()
	token, err := o.oauth(p).Exchange(exCtx, q.Get("code"), oauth2.VerifierOption(saved.CodeVerifier))
	if err != nil {
		var re *oauth2.RetrieveError
		if errors.As(err, &re) {
			o.logger.Warn("el IdP rechazó el código", slog.Int("status", re.Response.StatusCode), slog.String("error_code", re.ErrorCode))
			o.fail(w, r, errIdPTokenInvalid)
			return
		}
		o.logger.Warn("IdP no disponible al canjear el código", slog.String("error", err.Error()))
		o.fail(w, r, errIdPUnavailable)
		return
	}
	claims, err := o.verify(exCtx, p, token, saved.Nonce)
	if err != nil {
		// The reason is logged, never the token (RF-18).
		o.logger.Warn("id_token no válido", slog.String("reason", err.Error()))
		o.fail(w, r, errIdPTokenInvalid)
		return
	}

	userID, err := o.svc.LoginOIDC(ctx, o.cfg, claims)
	if err != nil {
		o.fail(w, r, err)
		return
	}
	if err := o.sessions.Issue(w, userID, MethodOIDC); err != nil {
		o.fail(w, r, err)
		return
	}
	http.Redirect(w, r, o.authCfg.PublicURL+saved.ReturnTo, http.StatusFound)
}

func (o *OIDC) consumeState(ctx context.Context, r *http.Request, state string) (sqlc.OidcLoginState, error) {
	c, err := r.Cookie(stateCookie)
	if state == "" || err != nil || c.Value != state {
		return sqlc.OidcLoginState{}, errInvalidState
	}
	saved, err := o.store.ConsumeOIDCState(ctx, state)
	if err != nil || o.now().Sub(saved.CreatedAt) > StateTTL {
		return sqlc.OidcLoginState{}, errInvalidState
	}
	return saved, nil
}

// Claims are the parts of the id_token Kairos uses.
type Claims struct {
	Issuer            string
	Subject           string   `json:"sub"`
	Email             string   `json:"email"`
	EmailVerified     bool     `json:"email_verified"`
	Name              string   `json:"name"`
	PreferredUsername string   `json:"preferred_username"`
	Roles             []string `json:"roles"`
	Groups            []string `json:"groups"`
}

// verify checks signature, issuer, audience, expiry and nonce (RF-16).
func (o *OIDC) verify(ctx context.Context, p *oidc.Provider, token *oauth2.Token, nonce string) (Claims, error) {
	raw, ok := token.Extra("id_token").(string)
	if !ok || raw == "" {
		return Claims{}, errors.New("la respuesta no trae id_token")
	}
	idToken, err := p.Verifier(&oidc.Config{ClientID: o.cfg.ClientID, Now: o.now}).Verify(ctx, raw)
	if err != nil {
		return Claims{}, err
	}
	if idToken.Nonce != nonce {
		return Claims{}, errors.New("nonce no coincide")
	}
	var c Claims
	if err := idToken.Claims(&c); err != nil {
		return Claims{}, fmt.Errorf("leer claims: %w", err)
	}
	c.Issuer = idToken.Issuer
	return c, nil
}

// EndSessionURL is where logout sends an OIDC user, or "" if the IdP has no
// end_session_endpoint or cannot be reached (RF-52, RF-53).
func (o *OIDC) EndSessionURL(context.Context) string {
	if _, err := o.discover(); err != nil || o.endSession == "" {
		return ""
	}
	u, err := url.Parse(o.endSession)
	if err != nil {
		return ""
	}
	q := u.Query()
	q.Set("client_id", o.cfg.ClientID)
	q.Set("post_logout_redirect_uri", o.authCfg.PublicURL+"/login")
	u.RawQuery = q.Encode()
	return u.String()
}

// fail sends the browser to the login screen with an error code to translate.
func (o *OIDC) fail(w http.ResponseWriter, r *http.Request, err error) {
	code := "internal_error"
	var e *apperr.Error
	if errors.As(err, &e) {
		code = e.Code
	} else {
		o.logger.Error("error en el login OIDC", slog.String("error", err.Error()))
	}
	http.Redirect(w, r, o.authCfg.PublicURL+"/login?error="+url.QueryEscape(code), http.StatusFound)
}

// safeReturnTo only accepts paths inside Kairos.
func safeReturnTo(v string) string {
	if !strings.HasPrefix(v, "/") || strings.HasPrefix(v, "//") || strings.HasPrefix(v, "/\\") {
		return "/"
	}
	return v
}

func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
