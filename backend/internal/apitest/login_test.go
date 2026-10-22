package apitest

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"openscum/internal/auth"
	"openscum/internal/config"
)

const pw = "una-contrasena-larga"

func login(c *Client, email, password string) *Response {
	return c.Post("/api/auth/login", map[string]string{"email": email, "password": password})
}

func TestLocalLoginOpensSession(t *testing.T) {
	env := New(t)
	id := env.CreateUser(UserSpec{Email: "ana@x.com", Roles: []string{"member"}, Password: pw})
	c := env.Client()
	var me auth.MeResponse
	login(c, "  ANA@x.com ", pw).Expect(t, http.StatusOK).JSON(t, &me) // RF-10
	if me.ID != id || me.AuthMethod != auth.MethodLocal {
		t.Fatalf("me = %+v", me)
	}
	c.Get("/api/me").Expect(t, http.StatusOK)
}

func TestSessionCookieAttributes(t *testing.T) {
	env := New(t)
	id := env.CreateUser(UserSpec{Email: "a@x.com", Roles: []string{"member"}, Password: pw})
	res := login(env.Client(), "a@x.com", pw).Expect(t, http.StatusOK)

	var session *http.Cookie
	for _, c := range res.Cookies {
		if c.Name == auth.SessionCookie {
			session = c
		}
	}
	if session == nil {
		t.Fatal("no session cookie")
	}
	if !session.HttpOnly || session.SameSite != http.SameSiteLaxMode || session.Path != "/" {
		t.Fatalf("cookie attributes = %+v", session)
	}
	if d := time.Until(session.Expires); d < auth.SessionTTL-time.Minute || d > auth.SessionTTL {
		t.Fatalf("expires in %v, want 8h", d)
	}

	// The JWT only identifies the user and its expiry (RF-40).
	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(session.Value, claims); err != nil {
		t.Fatal(err)
	}
	for k := range claims {
		if !slices.Contains([]string{"sub", "exp", "iat"}, k) {
			t.Errorf("unexpected claim %q", k)
		}
	}
	if claims["sub"] != id.String() {
		t.Errorf("sub = %v, want %s", claims["sub"], id)
	}
}

func TestSecureCookieWhenConfigured(t *testing.T) {
	env := New(t, func(c *config.Config) { c.Auth.CookieSecure = true })
	env.CreateUser(UserSpec{Email: "a@x.com", Roles: []string{"member"}, Password: pw})
	res := login(env.Client(), "a@x.com", pw).Expect(t, http.StatusOK)
	for _, c := range res.Cookies {
		if c.Name == auth.SessionCookie && !c.Secure {
			t.Fatal("session cookie without Secure")
		}
	}
}

func TestLoginFailuresLookTheSame(t *testing.T) {
	env := New(t)
	env.CreateUser(UserSpec{Email: "ana@x.com", Roles: []string{"member"}, Password: pw})
	env.CreateUser(UserSpec{Email: "sso@x.com", Roles: []string{"member"}}) // no local password
	c := env.Client()
	login(c, "nadie@x.com", pw).Expect(t, http.StatusUnauthorized, "invalid_credentials")
	login(c, "ana@x.com", "otra-contrasena-mala").Expect(t, http.StatusUnauthorized, "invalid_credentials")
	login(c, "sso@x.com", pw).Expect(t, http.StatusUnauthorized, "invalid_credentials")
}

func TestLoginRejectsInactiveUsers(t *testing.T) {
	env := New(t)
	for _, status := range []string{"inactive", "offboarded"} {
		env.CreateUser(UserSpec{Email: status + "@x.com", Status: status, Roles: []string{"member"}, Password: pw})
		login(env.Client(), status+"@x.com", pw).Expect(t, http.StatusForbidden, "account_inactive")
	}
}

func TestLoginRateLimitByEmail(t *testing.T) {
	env := New(t)
	env.CreateUser(UserSpec{Email: "ana@x.com", Roles: []string{"member"}, Password: pw})
	for i := range auth.MaxLoginFailures {
		// Different clients share the IP, so only use one email to test the email key.
		login(env.Client(), "ana@x.com", fmt.Sprint("mala-", i, "-xxxxxxxx")).Expect(t, http.StatusUnauthorized)
	}
	login(env.Client(), "ana@x.com", pw).Expect(t, http.StatusTooManyRequests, "too_many_attempts")
}

func TestLoginRateLimitByIP(t *testing.T) {
	env := New(t)
	env.CreateUser(UserSpec{Email: "ana@x.com", Roles: []string{"member"}, Password: pw})
	for i := range auth.MaxLoginFailures {
		login(env.Client(), fmt.Sprintf("nadie%d@x.com", i), pw).Expect(t, http.StatusUnauthorized)
	}
	login(env.Client(), "ana@x.com", pw).Expect(t, http.StatusTooManyRequests, "too_many_attempts")
}

func TestLocalLoginDisabledInOIDCMode(t *testing.T) {
	env := New(t, func(c *config.Config) { c.Auth.Mode = config.AuthModeOIDC })
	env.CreateUser(UserSpec{Email: "ana@x.com", Roles: []string{"member"}, Password: pw})
	login(env.Client(), "ana@x.com", pw).Expect(t, http.StatusForbidden, "local_auth_disabled")
	env.Client().Post("/api/auth/register", map[string]string{"name": "N", "email": "n@x.com", "password": pw}).
		Expect(t, http.StatusForbidden, "local_auth_disabled")
}

func register(c *Client, name, email, password string) *Response {
	return c.Post("/api/auth/register", map[string]string{"name": name, "email": email, "password": password})
}

func TestRegisterCreatesActiveMemberWithoutTeam(t *testing.T) {
	env := New(t)
	c := env.Client()
	var me auth.MeResponse
	register(c, "Nuevo", "Nuevo@X.com", pw).Expect(t, http.StatusOK).JSON(t, &me)
	if me.Status != "active" || len(me.Teams) != 0 || len(me.Roles) != 1 || me.Roles[0].Role != "member" {
		t.Fatalf("me = %+v", me)
	}
	c.Get("/api/me").Expect(t, http.StatusOK)
	login(env.Client(), "nuevo@x.com", pw).Expect(t, http.StatusOK)
}

func TestRegisterErrors(t *testing.T) {
	env := New(t)
	register(env.Client(), "A", "a@x.com", pw).Expect(t, http.StatusOK)
	register(env.Client(), "B", " A@X.COM ", pw).Expect(t, http.StatusConflict, "email_conflict")
	res := register(env.Client(), "C", "c@x.com", "corta").Expect(t, http.StatusBadRequest, "password_too_short")
	if !strings.Contains(string(res.Body), `"field":"password"`) {
		t.Fatalf("missing field detail: %s", res.Body)
	}
	register(env.Client(), "", "d@x.com", pw).Expect(t, http.StatusBadRequest, "validation_failed")
	register(env.Client(), "E", "not-an-email", pw).Expect(t, http.StatusBadRequest, "validation_failed")
}

func TestRegisterDisabled(t *testing.T) {
	env := New(t, func(c *config.Config) { c.Auth.RegistrationEnabled = false })
	register(env.Client(), "A", "a@x.com", pw).Expect(t, http.StatusForbidden, "registration_disabled")
	var cfg auth.ConfigResponse
	env.Client().Get("/api/auth/config").JSON(t, &cfg)
	if cfg.RegistrationEnabled {
		t.Fatal("config advertises registration")
	}
}

func TestBootstrapAdminOnRegister(t *testing.T) {
	env := New(t, func(c *config.Config) { c.Auth.BootstrapAdminEmail = "boss@x.com" })
	var me auth.MeResponse
	register(env.Client(), "Boss", "Boss@x.com", pw).Expect(t, http.StatusOK).JSON(t, &me)
	if !hasRole(me, "admin", "manual") {
		t.Fatalf("bootstrap user roles = %+v", me.Roles)
	}
	if got := env.AuditActions(me.ID.String()); !slices.Contains(got, "user.role_added") {
		t.Fatalf("audit = %v", got)
	}
}

func TestBootstrapAdminOnLoginWhenNoActiveAdmin(t *testing.T) {
	env := New(t, func(c *config.Config) { c.Auth.BootstrapAdminEmail = "boss@x.com" })
	env.CreateUser(UserSpec{Email: "old-admin@x.com", Status: "inactive", Roles: []string{"admin"}})
	env.CreateUser(UserSpec{Email: "boss@x.com", Roles: []string{"member"}, Password: pw})
	var me auth.MeResponse
	login(env.Client(), "boss@x.com", pw).Expect(t, http.StatusOK).JSON(t, &me)
	if !hasRole(me, "admin", "manual") {
		t.Fatalf("roles = %+v, want admin (RF-55)", me.Roles)
	}
}

func TestBootstrapIgnoredWhenAnActiveAdminExists(t *testing.T) {
	env := New(t, func(c *config.Config) { c.Auth.BootstrapAdminEmail = "boss@x.com" })
	env.CreateUser(UserSpec{Email: "admin@x.com", Roles: []string{"admin"}})
	var me auth.MeResponse
	register(env.Client(), "Boss", "boss@x.com", pw).Expect(t, http.StatusOK).JSON(t, &me)
	if hasRole(me, "admin", "manual") {
		t.Fatal("bootstrap applied with an active admin (RF-56)")
	}
}

func TestLogoutClearsSession(t *testing.T) {
	env := New(t)
	env.CreateUser(UserSpec{Email: "a@x.com", Roles: []string{"member"}, Password: pw})
	c := env.Client()
	login(c, "a@x.com", pw).Expect(t, http.StatusOK)
	var out struct {
		RedirectURL string `json:"redirect_url"`
	}
	c.Post("/api/auth/logout", nil).Expect(t, http.StatusOK).JSON(t, &out)
	if out.RedirectURL != "/login" {
		t.Fatalf("redirect = %q", out.RedirectURL)
	}
	c.Get("/api/me").Expect(t, http.StatusUnauthorized)
}

func TestChangeOwnPassword(t *testing.T) {
	env := New(t)
	id := env.CreateUser(UserSpec{Email: "a@x.com", Roles: []string{"viewer"}, Password: pw})
	c := env.As(id)
	c.Do(http.MethodPut, "/api/me/password", map[string]string{"current_password": "incorrecta-xxxxx", "new_password": "nueva-contrasena-1"}).
		Expect(t, http.StatusBadRequest, "wrong_password")
	c.Do(http.MethodPut, "/api/me/password", map[string]string{"current_password": pw, "new_password": "corta"}).
		Expect(t, http.StatusBadRequest, "password_too_short")
	c.Do(http.MethodPut, "/api/me/password", map[string]string{"current_password": pw, "new_password": "nueva-contrasena-1"}).
		Expect(t, http.StatusNoContent)
	login(env.Client(), "a@x.com", pw).Expect(t, http.StatusUnauthorized)
	login(env.Client(), "a@x.com", "nueva-contrasena-1").Expect(t, http.StatusOK)
}

func hasRole(me auth.MeResponse, role, source string) bool {
	return slices.Contains(me.Roles, auth.RoleAssignment{Role: role, Source: source})
}
