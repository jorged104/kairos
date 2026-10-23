package apitest

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"openscum/internal/auth"
	"openscum/internal/config"
)

// loginError returns the error code of a callback that redirected to /login.
func loginError(t *testing.T, r *Response) string {
	t.Helper()
	if r.Status != http.StatusFound {
		t.Fatalf("status = %d, want a redirect; body %s", r.Status, r.Body)
	}
	u, _ := url.Parse(r.Header.Get("Location"))
	if u.Path != "/login" {
		return ""
	}
	return u.Query().Get("error")
}

func expectLoggedIn(t *testing.T, r *Response) {
	t.Helper()
	if r.Status != http.StatusFound || r.Header.Get("Location") != Origin+"/users" {
		t.Fatalf("callback = %d → %q, want redirect to return_to", r.Status, r.Header.Get("Location"))
	}
}

func me(t *testing.T, c *Client) auth.MeResponse {
	t.Helper()
	var m auth.MeResponse
	c.Get("/api/me").Expect(t, http.StatusOK).JSON(t, &m)
	return m
}

func teamSource(m auth.MeResponse, id uuid.UUID) string {
	for _, t := range m.Teams {
		if t.ID == id {
			return t.Source
		}
	}
	return ""
}

func roleSource(m auth.MeResponse, role string) string {
	for _, r := range m.Roles {
		if r.Role == role {
			return r.Source
		}
	}
	return ""
}

type oidcFixture struct {
	idp   *IdP
	env   *Env
	admin *Client
	bi    uuid.UUID
	sub   uuid.UUID
}

func newOIDCFixture(t *testing.T, opts ...Option) oidcFixture {
	idp := NewIdP(t)
	env := New(t, append([]Option{WithIdP(idp)}, opts...)...)
	f := oidcFixture{idp: idp, env: env}
	f.admin = env.As(env.CreateUser(UserSpec{Email: "admin@x.com", Roles: []string{"admin"}}))
	f.bi = env.CreateTeam("BI", nil)
	f.sub = env.CreateTeam("BI Reportes", &f.bi)
	f.mapping("roles", "kairos-lead", "role", "lead")
	f.mapping("groups", "/Areas/BI/*", "team", f.bi.String())
	return f
}

func (f oidcFixture) mapping(claim, value, targetType, target string) {
	f.admin.Post("/api/idp-mappings", map[string]any{
		"claim": claim, "external_value": value, "target_type": targetType, "target": target,
	}).Expect(f.env.t, http.StatusCreated)
}

func TestOIDCJustInTimeWithMappings(t *testing.T) {
	f := newOIDCFixture(t)
	c := f.env.Client()
	expectLoggedIn(t, f.idp.Login(c, Claims("u1", "Ana@x.com", map[string]any{
		"roles": []string{"kairos-lead", "unmapped"}, "groups": []string{"/Areas/BI/Reportes"},
	})))
	m := me(t, c)
	if m.Email != "Ana@x.com" || m.Name != "Usuario u1" || m.AuthMethod != auth.MethodOIDC {
		t.Fatalf("me = %+v", m)
	}
	if roleSource(m, "lead") != "idp" || teamSource(m, f.bi) != "idp" { // RF-28, RF-86
		t.Fatalf("roles %+v teams %+v", m.Roles, m.Teams)
	}
	if got := f.env.AuditActions(m.ID.String()); got[0] != "user.created" {
		t.Fatalf("audit = %v", got)
	}
}

func TestOIDCWildcardDoesNotMatchParent(t *testing.T) {
	f := newOIDCFixture(t)
	c := f.env.Client()
	expectLoggedIn(t, f.idp.Login(c, Claims("u1", "a@x.com", map[string]any{"groups": []string{"/Areas/BI"}})))
	if m := me(t, c); len(m.Teams) != 0 {
		t.Fatalf("parent group matched /Areas/BI/*: %+v", m.Teams)
	}
	if !auth.MatchesClaim("/Areas/BI/*", "/Areas/BI/a/b/c") || auth.MatchesClaim("/Areas/BI/*", "/Areas/BIX/a") {
		t.Fatal("wildcard depth/prefix handling")
	}
}

func TestOIDCSyncReplacesOnlyIdPAssignments(t *testing.T) {
	f := newOIDCFixture(t)
	other := f.env.CreateTeam("Otro", nil)
	c := f.env.Client()
	claims := Claims("u1", "a@x.com", map[string]any{"roles": []string{"kairos-lead"}, "groups": []string{"/Areas/BI/x"}})
	expectLoggedIn(t, f.idp.Login(c, claims))
	id := me(t, c).ID

	// Manual team and a manual copy of the IdP role.
	f.admin.Post("/api/users/"+id.String()+"/teams", map[string]any{"team_id": other}).Expect(t, http.StatusOK)
	f.admin.Post("/api/users/"+id.String()+"/roles", map[string]any{"role": "lead"}).Expect(t, http.StatusOK)

	// The IdP no longer sends the group nor the role.
	expectLoggedIn(t, f.idp.Login(c, Claims("u1", "a@x.com", nil)))
	m := me(t, c)
	if teamSource(m, f.bi) != "" || teamSource(m, other) != "manual" { // RF-30, RF-31
		t.Fatalf("teams = %+v", m.Teams)
	}
	if roleSource(m, "lead") != "manual" { // RF-32
		t.Fatalf("roles = %+v", m.Roles)
	}
}

func TestOIDCUpdatesProfileFromIdP(t *testing.T) {
	f := newOIDCFixture(t)
	f.env.CreateUser(UserSpec{Email: "taken@x.com"})
	c := f.env.Client()
	expectLoggedIn(t, f.idp.Login(c, Claims("u1", "a@x.com", nil)))
	id := me(t, c).ID
	f.admin.Do(http.MethodPatch, "/api/users/"+id.String(), map[string]any{"name": "Cambiado a mano"}).Expect(t, http.StatusOK)

	expectLoggedIn(t, f.idp.Login(c, Claims("u1", "nuevo@x.com", map[string]any{"name": "Nombre IdP"})))
	if m := me(t, c); m.Name != "Nombre IdP" || m.Email != "nuevo@x.com" { // RF-22
		t.Fatalf("me = %+v", m)
	}
	expectLoggedIn(t, f.idp.Login(c, Claims("u1", "taken@x.com", map[string]any{"name": "Nombre IdP"})))
	if m := me(t, c); m.Email != "nuevo@x.com" { // RF-22a
		t.Fatalf("email = %q, want the current one kept", m.Email)
	}
}

func TestOIDCLinksVerifiedEmail(t *testing.T) {
	f := newOIDCFixture(t)
	local := f.env.CreateUser(UserSpec{Email: "ana@x.com", Roles: []string{"viewer"}, Password: pw})
	c := f.env.Client()
	expectLoggedIn(t, f.idp.Login(c, Claims("u1", "ANA@x.com", nil)))
	m := me(t, c)
	if m.ID != local || roleSource(m, "viewer") != "manual" || !m.HasPassword { // RF-24
		t.Fatalf("me = %+v", m)
	}
	login(f.env.Client(), "ana@x.com", pw).Expect(t, http.StatusOK)
}

func TestOIDCRejectsUnverifiedOrDisabledLinking(t *testing.T) {
	f := newOIDCFixture(t)
	f.env.CreateUser(UserSpec{Email: "ana@x.com", Roles: []string{"viewer"}})
	if code := loginError(t, f.idp.Login(f.env.Client(), Claims("u1", "ana@x.com", map[string]any{"email_verified": false}))); code != "account_conflict" {
		t.Fatalf("unverified: %q", code)
	}

	g := newOIDCFixture(t, func(c *config.Config) { c.OIDC.LinkByEmail = false })
	g.env.CreateUser(UserSpec{Email: "ana@x.com", Roles: []string{"viewer"}})
	if code := loginError(t, g.idp.Login(g.env.Client(), Claims("u1", "ana@x.com", nil))); code != "account_conflict" {
		t.Fatalf("linking disabled: %q", code)
	}
	if n := countUsers(t, g.env); n != 2 { // admin + ana: nothing created (RF-25)
		t.Fatalf("users = %d", n)
	}
}

func TestOIDCRequiresEmailForJIT(t *testing.T) {
	f := newOIDCFixture(t)
	if code := loginError(t, f.idp.Login(f.env.Client(), map[string]any{"sub": "u1"})); code != "email_required" {
		t.Fatalf("code = %q", code)
	}
}

func TestOIDCInactiveUserIsLinkedButRejected(t *testing.T) {
	f := newOIDCFixture(t)
	f.env.CreateUser(UserSpec{Email: "ana@x.com", Status: "inactive", Roles: []string{"viewer"}})
	if code := loginError(t, f.idp.Login(f.env.Client(), Claims("u1", "ana@x.com", nil))); code != "account_inactive" {
		t.Fatalf("code = %q", code)
	}
	var n int
	if err := f.env.Store.Pool.QueryRow(context.Background(), "SELECT count(*) FROM user_identities").Scan(&n); err != nil || n != 1 {
		t.Fatalf("identities = %d, %v", n, err)
	}
}

func TestOIDCStateIsCheckedBeforeTheIdP(t *testing.T) {
	f := newOIDCFixture(t)
	c := f.env.Client()
	start := c.Get("/api/auth/oidc/start?return_to=/users")
	callback := f.idp.Authorize(start.Header.Get("Location"), Claims("u1", "a@x.com", nil))

	// Another browser (without the state cookie) can't use it.
	if code := loginError(t, f.env.Client().Get(callback)); code != "invalid_state" {
		t.Fatalf("foreign browser: %q", code)
	}
	if f.idp.TokenCalls != 0 {
		t.Fatal("the IdP was contacted with an invalid state")
	}
	// Used once, then it's gone.
	expectLoggedIn(t, c.Get(callback))
	c.SetCookie("kairos_oidc_state", stateOf(t, callback))
	if code := loginError(t, c.Get(callback)); code != "invalid_state" {
		t.Fatalf("reused state: %q", code)
	}

	// Expired after 10 minutes.
	c2 := f.env.Client()
	start = c2.Get("/api/auth/oidc/start")
	callback = f.idp.Authorize(start.Header.Get("Location"), Claims("u1", "a@x.com", nil))
	if _, err := f.env.Store.Pool.Exec(context.Background(), "UPDATE oidc_login_states SET created_at = now() - interval '11 minutes'"); err != nil {
		t.Fatal(err)
	}
	if code := loginError(t, c2.Get(callback)); code != "invalid_state" {
		t.Fatalf("expired state: %q", code)
	}
}

func stateOf(t *testing.T, callback string) string {
	u, _ := url.Parse(callback)
	return u.Query().Get("state")
}

func TestOIDCRejectsInvalidTokens(t *testing.T) {
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	cases := map[string]func(*IdP){
		"signature": func(i *IdP) { i.SignWith = other },
		"audience":  func(i *IdP) { i.Audience = "someone-else" },
		"nonce":     func(i *IdP) { i.OverrideNonce = "replayed" },
	}
	for name, tweak := range cases {
		t.Run(name, func(t *testing.T) {
			f := newOIDCFixture(t)
			tweak(f.idp)
			if code := loginError(t, f.idp.Login(f.env.Client(), Claims("u1", "a@x.com", nil))); code != "idp_token_invalid" {
				t.Fatalf("code = %q", code)
			}
		})
	}
}

func TestOIDCIdPUnavailable(t *testing.T) {
	f := newOIDCFixture(t)
	f.idp.TokenDelay = 10 * time.Second
	start := time.Now()
	if code := loginError(t, f.idp.Login(f.env.Client(), Claims("u1", "a@x.com", nil))); code != "idp_unavailable" {
		t.Fatalf("slow IdP: %q", code)
	}
	if d := time.Since(start); d > auth.IdPTimeout+time.Second {
		t.Fatalf("gave up after %v, want ≤ 5 s (RF-19)", d)
	}

	g := newOIDCFixture(t)
	g.idp.Srv.Close()
	if code := loginError(t, g.env.Client().Get("/api/auth/oidc/start")); code != "idp_unavailable" {
		t.Fatalf("IdP down: %q", code)
	}
	// Local login keeps working in "both" mode.
	g.env.CreateUser(UserSpec{Email: "l@x.com", Roles: []string{"member"}, Password: pw})
	login(g.env.Client(), "l@x.com", pw).Expect(t, http.StatusOK)
}

func TestOIDCDisabledInLocalMode(t *testing.T) {
	env := New(t)
	if code := loginError(t, env.Client().Get("/api/auth/oidc/start")); code != "oidc_disabled" {
		t.Fatalf("code = %q", code)
	}
}

func TestOIDCRoleSourceLocal(t *testing.T) {
	f := newOIDCFixture(t, func(c *config.Config) { c.OIDC.RoleSource = config.RoleSourceLocal })
	c := f.env.Client()
	expectLoggedIn(t, f.idp.Login(c, Claims("u1", "a@x.com", map[string]any{"roles": []string{"kairos-lead"}, "groups": []string{"/Areas/BI/x"}})))
	m := me(t, c)
	if roleSource(m, "lead") != "" || teamSource(m, f.bi) != "idp" { // RF-33, RF-34
		t.Fatalf("roles %+v teams %+v", m.Roles, m.Teams)
	}
	if roleSource(m, "member") != "manual" {
		t.Fatalf("default role = %+v", m.Roles)
	}
}

func TestOIDCDefaultRoleOrRejection(t *testing.T) {
	f := newOIDCFixture(t)
	c := f.env.Client()
	expectLoggedIn(t, f.idp.Login(c, Claims("u1", "a@x.com", nil)))
	if m := me(t, c); roleSource(m, "member") != "idp" { // RF-35
		t.Fatalf("roles = %+v", m.Roles)
	}

	g := newOIDCFixture(t, func(c *config.Config) { c.Auth.DefaultRole = "" })
	if code := loginError(t, g.idp.Login(g.env.Client(), Claims("u2", "b@x.com", nil))); code != "no_role" { // RF-36
		t.Fatalf("code = %q", code)
	}
}

func TestOIDCKeepsTheLastAdmin(t *testing.T) {
	f := newOIDCFixture(t)
	f.mapping("roles", "kairos-admin", "role", "admin")
	c := f.env.Client()
	expectLoggedIn(t, f.idp.Login(c, Claims("boss", "boss@x.com", map[string]any{"roles": []string{"kairos-admin"}})))
	id := me(t, c).ID
	// The seeded admin leaves: the SSO admin is the last one.
	var seeded uuid.UUID
	if err := f.env.Store.Pool.QueryRow(context.Background(), "SELECT id FROM users WHERE email = 'admin@x.com'").Scan(&seeded); err != nil {
		t.Fatal(err)
	}
	f.env.As(id).Post("/api/users/"+seeded.String()+"/deactivate", nil).Expect(t, http.StatusOK)

	expectLoggedIn(t, f.idp.Login(c, Claims("boss", "boss@x.com", nil)))
	if roleSource(me(t, c), "admin") != "idp" { // RF-38
		t.Fatal("sync removed the last admin")
	}
	if got := f.env.AuditActions(id.String()); !strings.Contains(strings.Join(got, ","), "user.role_kept_last_admin") {
		t.Fatalf("audit = %v", got)
	}
}

func TestOIDCIgnoresArchivedTeams(t *testing.T) {
	f := newOIDCFixture(t)
	f.admin.Post("/api/teams/"+f.sub.String()+"/archive", nil).Expect(t, http.StatusOK)
	f.mapping("groups", "/Areas/Old", "team", f.sub.String())
	c := f.env.Client()
	expectLoggedIn(t, f.idp.Login(c, Claims("u1", "a@x.com", map[string]any{"groups": []string{"/Areas/Old"}})))
	if m := me(t, c); len(m.Teams) != 0 {
		t.Fatalf("teams = %+v", m.Teams)
	}
}

func TestOIDCConcurrentFirstLogins(t *testing.T) {
	f := newOIDCFixture(t)
	clients := []*Client{f.env.Client(), f.env.Client(), f.env.Client()}
	callbacks := make([]string, len(clients))
	for i, c := range clients {
		callbacks[i] = f.idp.Authorize(c.Get("/api/auth/oidc/start?return_to=/users").Header.Get("Location"), Claims("same", "same@x.com", nil))
	}
	var wg sync.WaitGroup
	results := make([]*Response, len(clients))
	for i, c := range clients {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = c.Get(callbacks[i])
		}()
	}
	wg.Wait()
	for _, r := range results {
		expectLoggedIn(t, r)
	}
	if n := countUsers(t, f.env); n != 2 { // admin + one SSO user (RF-26)
		t.Fatalf("users = %d", n)
	}
}

func TestOIDCLogoutGoesThroughTheIdP(t *testing.T) {
	f := newOIDCFixture(t)
	c := f.env.Client()
	expectLoggedIn(t, f.idp.Login(c, Claims("u1", "a@x.com", nil)))
	var out struct {
		RedirectURL string `json:"redirect_url"`
	}
	c.Post("/api/auth/logout", nil).Expect(t, http.StatusOK).JSON(t, &out)
	u, _ := url.Parse(out.RedirectURL)
	if !strings.HasPrefix(out.RedirectURL, f.idp.Srv.URL+"/logout") || u.Query().Get("client_id") != idpClientID ||
		u.Query().Get("post_logout_redirect_uri") != Origin+"/login" { // RF-52
		t.Fatalf("redirect = %q", out.RedirectURL)
	}
	c.Get("/api/me").Expect(t, http.StatusUnauthorized)

	g := newOIDCFixture(t)
	g.idp.NoEndSession = true
	c = g.env.Client()
	expectLoggedIn(t, g.idp.Login(c, Claims("u1", "a@x.com", nil)))
	c.Post("/api/auth/logout", nil).JSON(t, &out)
	if out.RedirectURL != "/login" { // RF-53
		t.Fatalf("redirect = %q", out.RedirectURL)
	}
}

func TestIdPMappingsAdmin(t *testing.T) {
	f := newOIDCFixture(t)
	f.admin.Post("/api/idp-mappings", map[string]any{"claim": "roles", "external_value": "x", "target_type": "role", "target": "owner"}).
		Expect(t, http.StatusBadRequest, "validation_failed")
	f.admin.Post("/api/idp-mappings", map[string]any{"claim": "groups", "external_value": "x", "target_type": "team", "target": uuid.NewString()}).
		Expect(t, http.StatusBadRequest, "validation_failed")
	f.admin.Post("/api/idp-mappings", map[string]any{"claim": "roles", "external_value": "kairos-lead", "target_type": "role", "target": "lead"}).
		Expect(t, http.StatusConflict, "idp_mapping_conflict")

	var list struct {
		Items []struct {
			ID     uuid.UUID `json:"id"`
			Issuer string    `json:"issuer"`
		} `json:"items"`
	}
	f.admin.Get("/api/idp-mappings").Expect(t, http.StatusOK).JSON(t, &list)
	if len(list.Items) != 2 || list.Items[0].Issuer != f.idp.Srv.URL {
		t.Fatalf("mappings = %+v", list.Items)
	}
	f.admin.Do(http.MethodDelete, "/api/idp-mappings/"+list.Items[0].ID.String(), nil).Expect(t, http.StatusNoContent)
	if got := f.env.AuditActions(list.Items[0].ID.String()); len(got) != 2 {
		t.Fatalf("audit = %v", got)
	}
	lead := f.env.As(f.env.CreateUser(UserSpec{Email: "l@x.com", Roles: []string{"lead"}}))
	lead.Get("/api/idp-mappings").Expect(t, http.StatusForbidden, "forbidden")
}

func countUsers(t *testing.T, env *Env) int {
	t.Helper()
	var n int
	if err := env.Store.Pool.QueryRow(context.Background(), "SELECT count(*) FROM users").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
