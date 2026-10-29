package config

import (
	"strings"
	"testing"
)

func setBase(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("AUTH_MODE", "local")
	t.Setenv("SESSION_SECRET", strings.Repeat("s", 32))
	t.Setenv("PUBLIC_URL", "http://localhost:5173/")
}

func setOIDC(t *testing.T) {
	t.Helper()
	t.Setenv("OIDC_ISSUER", "http://idp/realms/kairos")
	t.Setenv("OIDC_CLIENT_ID", "kairos")
	t.Setenv("OIDC_CLIENT_SECRET", "secret")
	t.Setenv("OIDC_REDIRECT_URL", "http://localhost:5173/api/auth/oidc/callback")
}

func TestLoadReadsAuthMode(t *testing.T) {
	for _, mode := range []string{"local", "oidc", "both"} {
		t.Run(mode, func(t *testing.T) {
			setBase(t)
			setOIDC(t)
			t.Setenv("AUTH_MODE", mode)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.Auth.Mode != mode {
				t.Fatalf("mode = %q, want %q", cfg.Auth.Mode, mode)
			}
		})
	}
}

func TestLoadRejectsMissingOrInvalidAuthMode(t *testing.T) {
	for _, mode := range []string{"", "ldap"} {
		setBase(t)
		t.Setenv("AUTH_MODE", mode)
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), "AUTH_MODE") {
			t.Fatalf("AUTH_MODE=%q: err = %v, want an AUTH_MODE error", mode, err)
		}
	}
}

func TestLoadNamesMissingOIDCSetting(t *testing.T) {
	for _, name := range []string{"OIDC_ISSUER", "OIDC_CLIENT_ID", "OIDC_CLIENT_SECRET", "OIDC_REDIRECT_URL"} {
		t.Run(name, func(t *testing.T) {
			setBase(t)
			setOIDC(t)
			t.Setenv("AUTH_MODE", "both")
			t.Setenv(name, "")
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("err = %v, want it to name %s", err, name)
			}
		})
	}
}

func TestLoadIgnoresOIDCSettingsInLocalMode(t *testing.T) {
	setBase(t)
	if _, err := Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
}

func TestLoadDefaults(t *testing.T) {
	setBase(t)
	setOIDC(t)
	t.Setenv("AUTH_MODE", "both")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Auth.PublicURL != "http://localhost:5173" {
		t.Errorf("PublicURL = %q, want trailing slash trimmed", cfg.Auth.PublicURL)
	}
	if !cfg.Auth.CookieSecure || cfg.Auth.RegistrationEnabled || cfg.Auth.DefaultRole != "member" {
		t.Errorf("unexpected auth defaults: %+v", cfg.Auth)
	}
	if cfg.OIDC.RoleSource != RoleSourceIdP || cfg.OIDC.DiscoveryURL != cfg.OIDC.Issuer || cfg.OIDC.LinkByEmail {
		t.Errorf("unexpected OIDC defaults: %+v", cfg.OIDC)
	}
}

func TestLoadRejectsShortSessionSecret(t *testing.T) {
	setBase(t)
	t.Setenv("SESSION_SECRET", "short")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "SESSION_SECRET") {
		t.Fatalf("err = %v, want a SESSION_SECRET error", err)
	}
}

func TestLoadAllowsEmptyDefaultRole(t *testing.T) {
	setBase(t)
	t.Setenv("DEFAULT_ROLE", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Auth.DefaultRole != "" {
		t.Fatalf("DefaultRole = %q, want empty (reject users without role)", cfg.Auth.DefaultRole)
	}
}

func TestLoadRejectsUnknownDefaultRole(t *testing.T) {
	setBase(t)
	t.Setenv("DEFAULT_ROLE", "owner")
	if _, err := Load(); err == nil {
		t.Fatal("Load accepted DEFAULT_ROLE=owner")
	}
}

func setS3(t *testing.T) {
	t.Helper()
	t.Setenv("FILES_S3_ENDPOINT", "localhost:9000")
	t.Setenv("FILES_S3_BUCKET", "kairos")
	t.Setenv("FILES_S3_ACCESS_KEY", "key")
	t.Setenv("FILES_S3_SECRET_KEY", "secret")
}

// Without S3 the page files go to the database (RF-22).
func TestLoadFilesInTheDatabaseByDefault(t *testing.T) {
	setBase(t)
	cfg, err := Load()
	if err != nil || cfg.Files.S3() {
		t.Fatalf("Load = %+v, %v; want files in the database", cfg.Files, err)
	}
}

func TestLoadFilesInS3(t *testing.T) {
	setBase(t)
	setS3(t)
	t.Setenv("FILES_S3_USE_SSL", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if f := cfg.Files; !f.S3() || f.Bucket != "kairos" || f.UseSSL || f.AccessKey != "key" {
		t.Fatalf("files = %+v", f)
	}
}

// Half a configuration never falls back to the database silently.
func TestLoadRejectsHalfAnS3Configuration(t *testing.T) {
	cases := map[string]func(t *testing.T){
		"bucket without endpoint": func(t *testing.T) { t.Setenv("FILES_S3_BUCKET", "kairos") },
		"endpoint without bucket": func(t *testing.T) { setS3(t); t.Setenv("FILES_S3_BUCKET", "") },
		"endpoint without key":    func(t *testing.T) { setS3(t); t.Setenv("FILES_S3_SECRET_KEY", "") },
		"endpoint with scheme":    func(t *testing.T) { setS3(t); t.Setenv("FILES_S3_ENDPOINT", "http://localhost:9000") },
		"ssl not a boolean":       func(t *testing.T) { setS3(t); t.Setenv("FILES_S3_USE_SSL", "quizá") },
	}
	for name, set := range cases {
		t.Run(name, func(t *testing.T) {
			setBase(t)
			set(t)
			if _, err := Load(); err == nil {
				t.Fatal("Load accepted it")
			}
		})
	}
}
