package apitest

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"openscum/internal/auth"
	"openscum/internal/config"
)

func TestHealthStaysPublic(t *testing.T) {
	env := New(t)
	c := env.Client()
	c.Get("/health/live").Expect(t, http.StatusOK)
	c.Get("/health/ready").Expect(t, http.StatusOK)
}

func TestProtectedEndpointWithoutSession(t *testing.T) {
	env := New(t)
	env.Client().Get("/api/me").Expect(t, http.StatusUnauthorized, "unauthenticated")
}

func TestTamperedAndExpiredSessionsAreRejected(t *testing.T) {
	env := New(t)
	id := env.CreateUser(UserSpec{Email: "a@x.com", Roles: []string{"member"}})

	forged, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject: id.String(), ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}).SignedString([]byte("another-secret-another-secret-123"))
	expired, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject: id.String(), ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
	}).SignedString(env.Cfg.Auth.SessionSecret)

	for name, token := range map[string]string{"forged": forged, "expired": expired, "garbage": "abc"} {
		t.Run(name, func(t *testing.T) {
			c := env.Client()
			c.SetCookie(auth.SessionCookie, token)
			c.Get("/api/me").Expect(t, http.StatusUnauthorized, "unauthenticated")
		})
	}
}

func TestInactiveUserSessionIsRejected(t *testing.T) {
	env := New(t)
	for _, status := range []string{"inactive", "offboarded"} {
		id := env.CreateUser(UserSpec{Email: status + "@x.com", Status: status, Roles: []string{"member"}})
		env.As(id).Get("/api/me").Expect(t, http.StatusForbidden, "account_inactive")
	}
}

func TestCSRFRejectsForeignOrigin(t *testing.T) {
	env := New(t)
	id := env.CreateUser(UserSpec{Email: "a@x.com", Roles: []string{"member"}, Password: "una-contrasena-larga"})
	c := env.As(id)
	c.Origin = "http://evil.test"
	c.Post("/api/anything", nil).Expect(t, http.StatusForbidden, "csrf_rejected")
	c.Origin = ""
	c.Post("/api/anything", nil).Expect(t, http.StatusForbidden, "csrf_rejected")
}

func TestMeReturnsProfileRolesTeamsAndPermissions(t *testing.T) {
	env := New(t)
	team := env.CreateTeam("BI", nil)
	id := env.CreateUser(UserSpec{Name: "Ana", Email: "ana@x.com", Roles: []string{"member", "viewer"}, Teams: []uuid.UUID{team}})

	var me auth.MeResponse
	env.As(id).Get("/api/me").Expect(t, http.StatusOK).JSON(t, &me)
	if me.ID != id || me.Name != "Ana" || me.Email != "ana@x.com" || me.Status != "active" {
		t.Fatalf("profile = %+v", me)
	}
	if len(me.Roles) != 2 || me.Roles[0].Source != "manual" {
		t.Fatalf("roles = %+v", me.Roles)
	}
	if len(me.Teams) != 1 || me.Teams[0].ID != team || me.Teams[0].Name != "BI" {
		t.Fatalf("teams = %+v", me.Teams)
	}
	if !slices.Contains(me.Permissions, "directory:read") || slices.Contains(me.Permissions, "audit:read") {
		t.Fatalf("permissions = %v", me.Permissions)
	}
}

func TestAuthConfigShowsEnabledMethods(t *testing.T) {
	cases := map[string]auth.ConfigResponse{
		"local": {LocalEnabled: true, RegistrationEnabled: true},
		"oidc":  {OIDCEnabled: true},
		"both":  {LocalEnabled: true, OIDCEnabled: true, RegistrationEnabled: true},
	}
	for mode, want := range cases {
		t.Run(mode, func(t *testing.T) {
			env := New(t, func(c *config.Config) { c.Auth.Mode = mode })
			var got auth.ConfigResponse
			env.Client().Get("/api/auth/config").Expect(t, http.StatusOK).JSON(t, &got)
			if got != want {
				t.Fatalf("config = %+v, want %+v", got, want)
			}
		})
	}
}
