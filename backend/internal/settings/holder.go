// Package settings keeps the configuration of the system that the
// administrator edits from the interface: access mode, SSO, file storage and
// appearance (spec 015). It is stored in one row, copied from the environment
// on the first start, and applied without restarting.
package settings

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"

	"openscum/internal/config"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
)

// ReloadInterval is how often another instance's change is picked up (RF-3).
const ReloadInterval = 5 * time.Second

// CallbackPath is where the IdP sends the browser back (RF-25).
const CallbackPath = "/api/auth/oidc/callback"

// Theme is the appearance of the whole system (RF-39 to RF-47).
type Theme struct {
	Palette string `json:"palette"`
	Primary string `json:"primary"`
	Radius  string `json:"radius"`
}

// DefaultTheme is today's look (RF-46).
var DefaultTheme = Theme{Palette: "neutral", Primary: "default", Radius: "soft"}

var (
	Palettes  = []string{"neutral", "zinc", "slate", "stone", "gray"}
	Primaries = []string{"default", "blue", "green", "orange", "red", "rose", "violet", "yellow"}
	Radii     = []string{"none", "soft", "round"}
	// DefaultRoles leaves admin out (RF-15, RF-16).
	DefaultRoles = []string{"", "viewer", "member", "lead"}
)

// SSOTest is the result of the administrator's last test login (RF-29).
type SSOTest struct {
	OK          bool      `json:"ok"`
	Email       string    `json:"email,omitempty"`
	Name        string    `json:"name,omitempty"`
	Error       string    `json:"error,omitempty"`
	At          time.Time `json:"at"`
	Fingerprint string    `json:"fingerprint"`
}

// snapshot is one consistent reading of the row, with the secrets in clear.
type snapshot struct {
	row          sqlc.SystemSetting
	clientSecret string
	s3Secret     string
	test         *SSOTest
}

// Holder serves the current settings merged with the environment.
type Holder struct {
	env    config.Config
	store  *db.Store
	box    *box
	logger *slog.Logger
	cur    atomic.Pointer[snapshot]
}

// Open creates the row from the environment if there is none yet (RF-8) and
// loads it.
func Open(ctx context.Context, store *db.Store, env config.Config, logger *slog.Logger) (*Holder, error) {
	h := &Holder{env: env, store: store, box: newBox(env.Auth.SessionSecret), logger: logger}
	if err := h.seed(ctx); err != nil {
		return nil, err
	}
	if err := h.Reload(ctx); err != nil {
		return nil, err
	}
	return h, nil
}

func (h *Holder) seed(ctx context.Context) error {
	role := h.env.Auth.DefaultRole
	if !contains(DefaultRoles, role) {
		h.logger.Warn("DEFAULT_ROLE no se puede usar como rol por defecto; se usa member", slog.String("role", role))
		role = "member"
	}
	mode := h.env.Auth.Mode
	if mode == "" {
		mode = config.AuthModeLocal
	}
	o, f := h.env.OIDC, h.env.Files
	// An incomplete SSO cannot be the only way in (casos límite).
	if mode == config.AuthModeOIDC && (o.Issuer == "" || o.ClientID == "" || o.ClientSecret == "") {
		mode = config.AuthModeLocal
	}
	discovery := o.DiscoveryURL
	if discovery == o.Issuer {
		discovery = ""
	}
	kind := "db"
	if f.S3() {
		kind = "s3"
	}
	clientSecret, err := h.box.seal(o.ClientSecret)
	if err != nil {
		return err
	}
	s3Secret, err := h.box.seal(f.SecretKey)
	if err != nil {
		return err
	}
	if err := h.store.InsertSystemSettings(ctx, sqlc.InsertSystemSettingsParams{
		AuthMode: mode, RegistrationEnabled: h.env.Auth.RegistrationEnabled, DefaultRole: role,
		OidcIssuer: o.Issuer, OidcDiscoveryUrl: discovery, OidcClientID: o.ClientID, OidcClientSecret: clientSecret,
		FilesKind: kind, S3Endpoint: f.Endpoint, S3Bucket: f.Bucket, S3Region: f.Region, S3AccessKey: f.AccessKey,
		S3SecretKey: s3Secret, S3UseSsl: f.UseSSL || !f.S3(),
	}); err != nil {
		return fmt.Errorf("crear la configuración del sistema: %w", err)
	}
	return nil
}

// Reload reads the row again.
func (h *Holder) Reload(ctx context.Context) error {
	row, err := h.store.GetSystemSettings(ctx)
	if err != nil {
		return fmt.Errorf("leer la configuración del sistema: %w", err)
	}
	h.set(row)
	return nil
}

func (h *Holder) set(row sqlc.SystemSetting) {
	s := &snapshot{row: row}
	var err error
	if s.clientSecret, err = h.box.open(row.OidcClientSecret); err != nil {
		h.logger.Warn("no se puede descifrar el client secret del SSO; hay que escribirlo de nuevo")
	}
	if s.s3Secret, err = h.box.open(row.S3SecretKey); err != nil {
		h.logger.Warn("no se puede descifrar la clave secreta del S3; hay que escribirla de nuevo")
	}
	if len(row.OidcTest) > 0 {
		var t SSOTest
		if json.Unmarshal(row.OidcTest, &t) == nil {
			s.test = &t
		}
	}
	h.cur.Store(s)
}

// Run reloads when another instance changed the version (RF-3).
func (h *Holder) Run(ctx context.Context) {
	t := time.NewTicker(ReloadInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		v, err := h.store.SystemSettingsVersion(ctx)
		if err != nil {
			if !errors.Is(err, pgx.ErrNoRows) && ctx.Err() == nil {
				h.logger.Warn("leer la versión de la configuración", slog.String("error", err.Error()))
			}
			continue
		}
		if v != h.cur.Load().row.Version {
			if err := h.Reload(ctx); err != nil && ctx.Err() == nil {
				h.logger.Warn("recargar la configuración", slog.String("error", err.Error()))
			}
		}
	}
}

func (h *Holder) Version() int64 { return h.cur.Load().row.Version }

// Auth is the environment with the access mode, registration and default
// role of the interface (RF-9, RF-10).
func (h *Holder) Auth() config.AuthConfig {
	a := h.env.Auth
	r := h.cur.Load().row
	a.Mode, a.RegistrationEnabled, a.DefaultRole = r.AuthMode, r.RegistrationEnabled, r.DefaultRole
	return a
}

// OIDC is the SSO of the interface with the rules that stay in the
// environment; the return URL derives from the public URL (RF-25).
func (h *Holder) OIDC() config.OIDCConfig {
	s := h.cur.Load()
	o := h.env.OIDC
	o.Issuer, o.ClientID, o.ClientSecret = s.row.OidcIssuer, s.row.OidcClientID, s.clientSecret
	o.DiscoveryURL = s.row.OidcDiscoveryUrl
	if o.DiscoveryURL == "" {
		o.DiscoveryURL = o.Issuer
	}
	o.RedirectURL = h.env.Auth.PublicURL + CallbackPath
	if o.RoleSource == "" {
		o.RoleSource = config.RoleSourceIdP
	}
	return o
}

// Config is the whole configuration with the settings of the interface.
func (h *Holder) Config() config.Config {
	c := h.env
	c.Auth, c.OIDC, c.Files = h.Auth(), h.OIDC(), h.Files()
	return c
}

// Files is the storage of the interface.
func (h *Holder) Files() config.FilesConfig {
	s := h.cur.Load()
	if s.row.FilesKind != "s3" {
		return config.FilesConfig{}
	}
	return config.FilesConfig{
		Endpoint: s.row.S3Endpoint, Bucket: s.row.S3Bucket, Region: s.row.S3Region,
		AccessKey: s.row.S3AccessKey, SecretKey: s.s3Secret, UseSSL: s.row.S3UseSsl,
	}
}

// MCPEnabled reports whether AI clients may connect (spec 016, RF-3).
func (h *Holder) MCPEnabled() bool { return h.cur.Load().row.McpEnabled }

func (h *Holder) Theme() Theme {
	r := h.cur.Load().row
	return Theme{Palette: r.ThemePalette, Primary: r.ThemePrimary, Radius: r.ThemeRadius}
}

// SSOConfigured reports whether the SSO has every required piece (RF-12).
func (h *Holder) SSOConfigured() bool {
	o := h.OIDC()
	return o.Issuer != "" && o.ClientID != "" && o.ClientSecret != ""
}

// Fingerprint identifies an SSO configuration, so a test of another one does
// not count (RF-31). It hashes the secret, never stores it.
func Fingerprint(o config.OIDCConfig) string {
	sum := sha256.Sum256([]byte(o.Issuer + "\x00" + o.DiscoveryURL + "\x00" + o.ClientID + "\x00" + o.ClientSecret))
	return hex.EncodeToString(sum[:8])
}

// SSOTested reports whether the last test login passed with the current SSO (RF-13).
func (h *Holder) SSOTested() bool {
	t := h.cur.Load().test
	return t != nil && t.OK && t.Fingerprint == Fingerprint(h.OIDC())
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
