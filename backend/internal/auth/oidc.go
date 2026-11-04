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
	"openscum/internal/authz"
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
	cfg     func() config.OIDCConfig
	authCfg func() config.AuthConfig
	// tested receives the result of an administrator's test login (spec 015, RF-29).
	tested   func(ctx context.Context, email, name string, err error) error
	store    *db.Store
	svc      *Service
	sessions *Sessions
	logger   *slog.Logger
	client   *http.Client
	now      func() time.Time

	mu         sync.Mutex
	provider   *oidc.Provider
	providerOf string // the issuer and discovery URL it was found with
	endSession string
}

func NewOIDC(cfg func() config.OIDCConfig, authCfg func() config.AuthConfig, store *db.Store, svc *Service, sessions *Sessions, logger *slog.Logger) *OIDC {
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

// SetTested receives the result of the test logins (spec 015, RF-29).
func (o *OIDC) SetTested(f func(ctx context.Context, email, name string, err error) error) {
	o.tested = f
}

// discover fetches the IdP metadata once per issuer and discovery URL, so a
// change of the settings finds the new IdP (spec 015); failures are retried on
// the next login.
func (o *OIDC) discover() (*oidc.Provider, error) {
	cfg := o.cfg()
	of := cfg.Issuer + " " + cfg.DiscoveryURL
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.provider != nil && o.providerOf == of {
		return o.provider, nil
	}
	// The key set keeps this context for later fetches, so it must not be a
	// request context; the client timeout bounds every call instead.
	ctx := oidc.ClientContext(context.Background(), o.client)
	if cfg.DiscoveryURL != cfg.Issuer {
		ctx = oidc.InsecureIssuerURLContext(ctx, cfg.Issuer)
	}
	p, err := oidc.NewProvider(ctx, cfg.DiscoveryURL)
	if err != nil {
		return nil, fmt.Errorf("descubrir el IdP: %w", err)
	}
	var meta struct {
		EndSession string `json:"end_session_endpoint"`
	}
	_ = p.Claims(&meta)
	o.provider, o.providerOf, o.endSession = p, of, meta.EndSession
	return p, nil
}

func (o *OIDC) oauth(p *oidc.Provider) *oauth2.Config {
	cfg := o.cfg()
	return &oauth2.Config{
		ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, RedirectURL: cfg.RedirectURL,
		Endpoint: p.Endpoint(), Scopes: []string{oidc.ScopeOpenID, "profile", "email"},
	}
}

// Start redirects the browser to the IdP (RF-15). With ?test=1 it is the
// administrator's test of the saved SSO, allowed in any mode (spec 015, RF-28).
func (o *OIDC) Start(w http.ResponseWriter, r *http.Request) {
	purpose := "login"
	if r.URL.Query().Get("test") == "1" {
		if !o.canTest(r) {
			o.fail(w, r, apperr.ErrForbidden)
			return
		}
		purpose = "test"
	} else if !o.authCfg().OIDCEnabled() {
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
		State: state, Nonce: nonce, CodeVerifier: verifier, ReturnTo: safeReturnTo(r.URL.Query().Get("return_to")), Purpose: purpose,
	}); err != nil {
		o.fail(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: stateCookie, Value: state, Path: statePath, MaxAge: int(StateTTL.Seconds()),
		HttpOnly: true, Secure: o.authCfg().CookieSecure, SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, o.oauth(p).AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)), http.StatusFound)
}

// Callback completes the login when the IdP sends the browser back (RF-16 to RF-39).
func (o *OIDC) Callback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	http.SetCookie(w, &http.Cookie{Name: stateCookie, Path: statePath, MaxAge: -1, HttpOnly: true, Secure: o.authCfg().CookieSecure, SameSite: http.SameSiteLaxMode})

	// The state is checked before talking to the IdP (RF-17).
	saved, err := o.consumeState(ctx, r, q.Get("state"))
	if err != nil {
		o.fail(w, r, err)
		return
	}
	if saved.Purpose == "test" {
		o.finishTest(w, r, saved)
		return
	}
	if !o.authCfg().OIDCEnabled() {
		o.fail(w, r, errOIDCDisabled)
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

	userID, err := o.svc.LoginOIDC(ctx, o.cfg(), claims)
	if err != nil {
		o.fail(w, r, err)
		return
	}
	if err := o.sessions.Issue(w, userID, MethodOIDC); err != nil {
		o.fail(w, r, err)
		return
	}
	http.Redirect(w, r, o.authCfg().PublicURL+saved.ReturnTo, http.StatusFound)
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
	idToken, err := p.Verifier(&oidc.Config{ClientID: o.cfg().ClientID, Now: o.now}).Verify(ctx, raw)
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
	q.Set("client_id", o.cfg().ClientID)
	q.Set("post_logout_redirect_uri", o.authCfg().PublicURL+"/login")
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
	http.Redirect(w, r, o.authCfg().PublicURL+"/login?error="+url.QueryEscape(code), http.StatusFound)
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

// TestReturnPath is where a test login ends, on the SSO tab (spec 015).
const TestReturnPath = "/admin/system?tab=sso"

// canTest: only a signed-in administrator of the system tests the SSO.
func (o *OIDC) canTest(r *http.Request) bool {
	id, err := o.sessions.Parse(r)
	if err != nil {
		return false
	}
	p, err := o.svc.loader.Load(r.Context(), id)
	return err == nil && p.Has(authz.SystemManage)
}

// finishTest validates the IdP's answer like a login but opens no session and
// creates or links nobody; it only records the result (spec 015, RF-29, RF-30).
func (o *OIDC) finishTest(w http.ResponseWriter, r *http.Request, saved sqlc.OidcLoginState) {
	ctx := r.Context()
	claims, err := o.exchange(ctx, r, saved)
	if o.tested != nil {
		if rerr := o.tested(ctx, claims.Email, displayName(claims), err); rerr != nil {
			o.logger.Warn("guardar el login de prueba", slog.String("error", rerr.Error()))
		}
	}
	http.Redirect(w, r, o.authCfg().PublicURL+TestReturnPath, http.StatusFound)
}

// exchange turns the authorization code into validated claims.
func (o *OIDC) exchange(ctx context.Context, r *http.Request, saved sqlc.OidcLoginState) (Claims, error) {
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		return Claims{}, errors.New("el IdP devolvió un error: " + e)
	}
	p, err := o.discover()
	if err != nil {
		return Claims{}, err
	}
	exCtx, cancel := context.WithTimeout(oidc.ClientContext(ctx, o.client), IdPTimeout)
	defer cancel()
	token, err := o.oauth(p).Exchange(exCtx, q.Get("code"), oauth2.VerifierOption(saved.CodeVerifier))
	if err != nil {
		return Claims{}, fmt.Errorf("canjear el código: %w", err)
	}
	return o.verify(exCtx, p, token, saved.Nonce)
}
