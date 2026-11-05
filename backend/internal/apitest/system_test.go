package apitest

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"openscum/internal/config"
	"openscum/internal/settings"
)

type systemDTO struct {
	Version int64 `json:"version"`
	Access  struct {
		Mode                string `json:"mode"`
		RegistrationEnabled bool   `json:"registration_enabled"`
		DefaultRole         string `json:"default_role"`
	} `json:"access"`
	SSO struct {
		Issuer          string `json:"issuer"`
		ClientID        string `json:"client_id"`
		ClientSecretSet bool   `json:"client_secret_set"`
		RedirectURL     string `json:"redirect_url"`
		Configured      bool   `json:"configured"`
		Tested          bool   `json:"tested"`
		Test            *struct {
			OK    bool   `json:"ok"`
			Email string `json:"email"`
			Error string `json:"error"`
		} `json:"test"`
	} `json:"sso"`
	Storage struct {
		Kind         string `json:"kind"`
		Endpoint     string `json:"endpoint"`
		Bucket       string `json:"bucket"`
		Region       string `json:"region"`
		SecretKeySet bool   `json:"secret_key_set"`
		FilesCount   int64  `json:"files_count"`
		Locked       bool   `json:"locked"`
	} `json:"storage"`
	Appearance settings.Theme `json:"appearance"`
	MCP        struct {
		Enabled bool `json:"enabled"`
	} `json:"mcp"`
	EmergencyAvailable bool `json:"emergency_available"`
}

func system(t *testing.T, c *Client) systemDTO {
	t.Helper()
	var out systemDTO
	c.Get("/api/system/settings").Expect(t, http.StatusOK).JSON(t, &out)
	return out
}

func putSystem(c *Client, section string, body map[string]any) *Response {
	return c.Do(http.MethodPut, "/api/system/settings/"+section, body)
}

// testSSO runs the administrator's test login against the IdP (RF-28).
func testSSO(t *testing.T, idp *IdP, c *Client, claims map[string]any) *Response {
	t.Helper()
	start := c.Get("/api/auth/oidc/start?test=1")
	if start.Status != http.StatusFound {
		t.Fatalf("test start = %d %s", start.Status, start.Body)
	}
	return c.Get(idp.Authorize(start.Header.Get("Location"), claims))
}

func TestSystemSettingsCopiedFromTheEnvironment(t *testing.T) {
	idp := NewIdP(t)
	env := New(t, WithIdP(idp), func(c *config.Config) {
		c.Auth.DefaultRole = "admin" // never a default role (RF-16)
		c.Files = config.FilesConfig{Endpoint: "s3.local:9000", Bucket: "kairos", Region: "eu", AccessKey: "AK", SecretKey: "SK-very-secret", UseSSL: false}
	})
	admin, member := env.Admin(), env.As(env.CreateUser(UserSpec{Email: "m@x.com", Roles: []string{"member"}}))

	res := admin.Get("/api/system/settings").Expect(t, http.StatusOK)
	if strings.Contains(string(res.Body), idpClientSecret+`"`) || strings.Contains(string(res.Body), "SK-very-secret") {
		t.Fatalf("the settings return a secret (RF-6): %s", res.Body)
	}
	var s systemDTO
	res.JSON(t, &s)
	if s.Access.Mode != "both" || s.Access.DefaultRole != "member" || !s.Access.RegistrationEnabled {
		t.Fatalf("access = %+v (RF-8, RF-16)", s.Access)
	}
	if s.SSO.Issuer != idp.Srv.URL || !s.SSO.ClientSecretSet || !s.SSO.Configured || s.SSO.Tested || s.SSO.RedirectURL != Origin+"/api/auth/oidc/callback" {
		t.Fatalf("sso = %+v (RF-8, RF-25)", s.SSO)
	}
	if s.Storage.Kind != "s3" || s.Storage.Bucket != "kairos" || !s.Storage.SecretKeySet || s.Appearance != settings.DefaultTheme {
		t.Fatalf("storage = %+v, appearance = %+v (RF-8, RF-46)", s.Storage, s.Appearance)
	}
	// The secret is stored encrypted (RF-5).
	var raw []byte
	if err := env.Store.Pool.QueryRow(context.Background(), "SELECT s3_secret_key FROM system_settings").Scan(&raw); err != nil || strings.Contains(string(raw), "SK-very-secret") {
		t.Fatalf("secret in clear: %v", err)
	}
	member.Get("/api/system/settings").Expect(t, http.StatusForbidden)
	putSystem(member, "appearance", map[string]any{"palette": "zinc", "primary": "blue", "radius": "round"}).Expect(t, http.StatusForbidden)
}

func TestAccessModeAndSSOTest(t *testing.T) {
	idp := NewIdP(t)
	env := New(t, WithIdP(idp), func(c *config.Config) { c.Auth.BootstrapAdminEmail = "root@x.com" })
	rootID := env.CreateUser(UserSpec{Name: "Root", Email: "root@x.com", Roles: []string{"admin"}, Password: pw})
	admin := env.As(rootID)

	// Only SSO needs a test login with the SSO in force (RF-13).
	putSystem(admin, "access", map[string]any{"mode": "oidc"}).Expect(t, http.StatusConflict, "sso_not_tested")
	putSystem(admin, "access", map[string]any{"mode": "both", "default_role": "admin"}).Expect(t, http.StatusBadRequest, "validation_failed")

	// A member cannot test; the administrator's test opens no session and creates nobody (RF-28 to RF-30).
	env.As(env.CreateUser(UserSpec{Email: "m@x.com", Roles: []string{"member"}})).Get("/api/auth/oidc/start?test=1").Expect(t, http.StatusFound)
	before := countUsers(t, env)
	done := testSSO(t, idp, admin, Claims("nuevo", "nuevo@x.com", nil))
	if done.Status != http.StatusFound || done.Header.Get("Location") != Origin+"/admin/system?tab=sso" {
		t.Fatalf("test callback = %d %s", done.Status, done.Header.Get("Location"))
	}
	if countUsers(t, env) != before {
		t.Fatal("the test login created a user (RF-30)")
	}
	if got := me(t, admin); got.ID != rootID {
		t.Fatalf("the session changed to %v (RF-30)", got.ID)
	}
	s := system(t, admin)
	if !s.SSO.Tested || s.SSO.Test == nil || !s.SSO.Test.OK || s.SSO.Test.Email != "nuevo@x.com" {
		t.Fatalf("test = %+v (RF-29)", s.SSO)
	}

	// Now only SSO is accepted; registration has no effect without passwords (RF-14, RF-17).
	putSystem(admin, "access", map[string]any{"mode": "oidc", "registration_enabled": true, "default_role": "viewer"}).Expect(t, http.StatusOK)
	var cfg struct {
		LocalEnabled        bool `json:"local_enabled"`
		OIDCEnabled         bool `json:"oidc_enabled"`
		RegistrationEnabled bool `json:"registration_enabled"`
		EmergencyEnabled    bool `json:"emergency_enabled"`
	}
	env.Client().Get("/api/auth/config").Expect(t, http.StatusOK).JSON(t, &cfg)
	if cfg.LocalEnabled || !cfg.OIDCEnabled || cfg.RegistrationEnabled || !cfg.EmergencyEnabled {
		t.Fatalf("config = %+v (RF-17, RF-20)", cfg)
	}
	// The open session goes on (RF-18), new SSO users get the new default role.
	me(t, admin)
	expectLoggedIn(t, idp.Login(env.Client(), Claims("ana", "ana@x.com", nil)))

	// Emergency access: only the initial administrator, audited (RF-19, RF-22).
	env.CreateUser(UserSpec{Email: "otra@x.com", Roles: []string{"member"}, Password: pw})
	login(env.Client(), "otra@x.com", pw).Expect(t, http.StatusForbidden, "local_auth_disabled")
	login(env.Client(), "root@x.com", pw).Expect(t, http.StatusOK)
	if got := env.AuditActions(rootID.String()); got[len(got)-1] != "auth.emergency_login" {
		t.Fatalf("audit = %v (RF-22)", got)
	}

	// The SSO cannot change while it is the only way in (RF-32)...
	putSystem(admin, "sso", map[string]any{"issuer": idp.Srv.URL, "client_id": "otro"}).Expect(t, http.StatusConflict, "sso_in_use")
	putSystem(admin, "access", map[string]any{"mode": "both"}).Expect(t, http.StatusOK)
	// ...nor be saved if the discovery fails (RF-26, RF-27)...
	putSystem(admin, "sso", map[string]any{"issuer": "http://127.0.0.1:1", "client_id": "kairos"}).Expect(t, http.StatusBadRequest, "sso_discovery_failed")
	// ...and a change undoes the test (RF-31); an empty secret keeps the old one (RF-7).
	putSystem(admin, "sso", map[string]any{"issuer": idp.Srv.URL, "client_id": "kairos", "client_secret": ""}).Expect(t, http.StatusOK)
	if s := system(t, admin); !s.SSO.Tested || !s.SSO.ClientSecretSet {
		t.Fatalf("unchanged SSO lost its test or secret: %+v", s.SSO)
	}
	putSystem(admin, "sso", map[string]any{"issuer": idp.Srv.URL, "client_id": "kairos", "client_secret": "otro-secreto"}).Expect(t, http.StatusOK)
	if s := system(t, admin); s.SSO.Tested {
		t.Fatal("the test survived a change of the SSO (RF-31)")
	}
	putSystem(admin, "access", map[string]any{"mode": "oidc"}).Expect(t, http.StatusConflict, "sso_not_tested")

	// The audit keeps the change without the secret (RF-4, RF-6).
	var after string
	if err := env.Store.Pool.QueryRow(context.Background(),
		"SELECT after::text FROM audit_log WHERE action = 'system.settings_changed' AND target_id = 'sso' ORDER BY id DESC LIMIT 1").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(after, "otro-secreto") || !strings.Contains(after, "oidc_client_secret") {
		t.Fatalf("audit = %s (RF-4, RF-6)", after)
	}
}

func TestOnlySSOWithoutEmergencyAccess(t *testing.T) {
	idp := NewIdP(t)
	env := New(t, WithIdP(idp))
	admin := env.Admin()
	// Without SSO there is no SSO mode (RF-12).
	local := New(t)
	putSystem(local.Admin(), "access", map[string]any{"mode": "both"}).Expect(t, http.StatusConflict, "sso_not_configured")

	testSSO(t, idp, admin, Claims("x", "x@x.com", nil))
	if s := system(t, admin); s.EmergencyAvailable {
		t.Fatal("emergency access without an initial administrator")
	}
	putSystem(admin, "access", map[string]any{"mode": "oidc"}).Expect(t, http.StatusConflict, "no_emergency_access")
	putSystem(admin, "access", map[string]any{"mode": "oidc", "accept_no_emergency": true}).Expect(t, http.StatusOK)
}

func TestStorageSettings(t *testing.T) {
	env := New(t)
	admin := env.Admin()
	var checked []config.FilesConfig
	fail := ""
	env.App.Settings.CheckS3 = func(_ context.Context, f config.FilesConfig) (string, error) {
		checked = append(checked, f)
		if fail != "" {
			return fail, errors.New("no")
		}
		return "", nil
	}
	s3 := map[string]any{"kind": "s3", "endpoint": "s3.local:9000", "bucket": "kairos", "region": "eu", "access_key": "AK", "secret_key": "SK", "use_ssl": true}

	// The check must pass (RF-35, RF-36).
	fail = "write"
	putSystem(admin, "storage", s3).Expect(t, http.StatusBadRequest, "s3_check_failed")
	fail = ""
	putSystem(admin, "storage", map[string]any{"kind": "s3", "endpoint": "http://s3.local", "bucket": "k", "access_key": "a", "secret_key": "s"}).
		Expect(t, http.StatusBadRequest, "validation_failed")
	putSystem(admin, "storage", s3).Expect(t, http.StatusOK)
	if s := system(t, admin); s.Storage.Kind != "s3" || !s.Storage.SecretKeySet || s.Storage.Locked {
		t.Fatalf("storage = %+v (RF-33, RF-34)", s.Storage)
	}

	// With files, the place is fixed but the keys can change (RF-37, RF-38).
	project := env.SeedProject("FIL", env.CreateTeam("T", nil), env.CreateUser(UserSpec{Email: "o@x.com", Roles: []string{"member"}}))
	if _, err := env.Store.Pool.Exec(context.Background(), `INSERT INTO page_files (project_id, owner_kind, owner_id, kind, name, content_type, size_bytes, storage, created_by)
		SELECT $1, 'project', $1, 'file', 'a.txt', 'text/plain', 1, 'db', owner_id FROM projects WHERE id = $1`, project); err != nil {
		t.Fatal(err)
	}
	putSystem(admin, "storage", map[string]any{"kind": "db"}).Expect(t, http.StatusConflict, "storage_locked")
	moved := map[string]any{}
	for k, v := range s3 {
		moved[k] = v
	}
	moved["bucket"] = "otro"
	putSystem(admin, "storage", moved).Expect(t, http.StatusConflict, "storage_locked")
	s3["region"], s3["secret_key"] = "us", ""
	putSystem(admin, "storage", s3).Expect(t, http.StatusOK)
	if last := checked[len(checked)-1]; last.Region != "us" || last.SecretKey != "SK" {
		t.Fatalf("checked = %+v, want the new region with the kept secret (RF-7, RF-38)", last)
	}
	if s := system(t, admin); s.Storage.FilesCount != 1 || !s.Storage.Locked || s.Storage.Region != "us" {
		t.Fatalf("storage = %+v (RF-37)", s.Storage)
	}
}

func TestAppearanceAndPreferences(t *testing.T) {
	env := New(t)
	admin := env.Admin()
	putSystem(admin, "appearance", map[string]any{"palette": "teal", "primary": "blue", "radius": "soft"}).Expect(t, http.StatusBadRequest, "validation_failed")
	putSystem(admin, "appearance", map[string]any{"palette": "zinc", "primary": "violet", "radius": "round"}).Expect(t, http.StatusOK)

	// The login sees it at once (RF-3, RF-45).
	var cfg struct {
		Theme settings.Theme `json:"theme"`
	}
	env.Client().Get("/api/auth/config").Expect(t, http.StatusOK).JSON(t, &cfg)
	if cfg.Theme != (settings.Theme{Palette: "zinc", Primary: "violet", Radius: "round"}) {
		t.Fatalf("theme = %+v (RF-45)", cfg.Theme)
	}
	if got := env.AuditActions("appearance"); len(got) != 1 || got[0] != "system.settings_changed" {
		t.Fatalf("audit = %v (RF-4)", got)
	}

	// Light or dark per person, kept on the server (RF-48 to RF-50).
	c := env.As(env.CreateUser(UserSpec{Email: "u@x.com", Roles: []string{"viewer"}}))
	if got := me(t, c); got.ThemeMode != "system" {
		t.Fatalf("mode = %q (RF-49)", got.ThemeMode)
	}
	c.Do(http.MethodPut, "/api/me/preferences", map[string]any{"theme_mode": "sepia"}).Expect(t, http.StatusBadRequest, "validation_failed")
	c.Do(http.MethodPut, "/api/me/preferences", map[string]any{"theme_mode": "dark"}).Expect(t, http.StatusNoContent)
	if got := me(t, c); got.ThemeMode != "dark" {
		t.Fatalf("mode = %q (RF-50)", got.ThemeMode)
	}
}
