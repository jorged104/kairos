package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/authz"
	"openscum/internal/config"
	"openscum/internal/db/sqlc"
)

const (
	discoveryTimeout = 5 * time.Second  // RF-26
	s3CheckTimeout   = 10 * time.Second // RF-35
)

var (
	errSSONotConfigured = apperr.New(http.StatusConflict, "sso_not_configured", "Falta configurar el SSO")
	errSSONotTested     = apperr.New(http.StatusConflict, "sso_not_tested", "Haz un login de prueba correcto antes de usar solo SSO")
	errNoEmergency      = apperr.New(http.StatusConflict, "no_emergency_access", "No hay acceso de emergencia")
	errSSOInUse         = apperr.New(http.StatusConflict, "sso_in_use", "Pasa el modo a «ambos» antes de cambiar el SSO")
	errStorageLocked    = apperr.New(http.StatusConflict, "storage_locked", "Ya hay archivos guardados")
)

func errDiscovery(reason string) *apperr.Error {
	return apperr.New(http.StatusBadRequest, "sso_discovery_failed", "El proveedor de SSO no responde").WithDetail("reason", reason)
}

func errS3(step string) *apperr.Error {
	return apperr.New(http.StatusBadRequest, "s3_check_failed", "No se pudo usar el S3").WithDetail("step", step)
}

// Service reads and changes the settings. The checks against the IdP and the
// S3 are fields so the tests can replace them.
type Service struct {
	h              *Holder
	CheckDiscovery func(ctx context.Context, o config.OIDCConfig) error
	CheckS3        func(ctx context.Context, f config.FilesConfig) (step string, err error)
}

func NewService(h *Holder) *Service {
	return &Service{h: h, CheckDiscovery: checkDiscovery, CheckS3: checkS3}
}

type AccessView struct {
	Mode                string `json:"mode"`
	RegistrationEnabled bool   `json:"registration_enabled"`
	DefaultRole         string `json:"default_role"`
}

type SSOView struct {
	Issuer          string   `json:"issuer"`
	DiscoveryURL    string   `json:"discovery_url"`
	ClientID        string   `json:"client_id"`
	ClientSecretSet bool     `json:"client_secret_set"`
	RedirectURL     string   `json:"redirect_url"`
	Configured      bool     `json:"configured"`
	Test            *SSOTest `json:"test"`
	Tested          bool     `json:"tested"`
}

type StorageView struct {
	Kind         string `json:"kind"`
	Endpoint     string `json:"endpoint"`
	Bucket       string `json:"bucket"`
	Region       string `json:"region"`
	AccessKey    string `json:"access_key"`
	SecretKeySet bool   `json:"secret_key_set"`
	UseSSL       bool   `json:"use_ssl"`
	FilesCount   int64  `json:"files_count"`
	Locked       bool   `json:"locked"`
}

// MCPView is the switch of the AI connections (spec 016, RF-2).
type MCPView struct {
	Enabled bool `json:"enabled"`
}

// View is the whole configuration without the secrets (RF-6).
type View struct {
	Version            int64       `json:"version"`
	Access             AccessView  `json:"access"`
	SSO                SSOView     `json:"sso"`
	Storage            StorageView `json:"storage"`
	Appearance         Theme       `json:"appearance"`
	MCP                MCPView     `json:"mcp"`
	EmergencyAvailable bool        `json:"emergency_available"`
	EmergencyEmail     string      `json:"emergency_email"`
}

func (s *Service) View(ctx context.Context) (View, error) {
	q := s.h.store.Queries
	snap := s.h.cur.Load()
	r := snap.row
	files, err := q.CountLiveFiles(ctx)
	if err != nil {
		return View{}, fmt.Errorf("contar archivos: %w", err)
	}
	emergency, err := s.emergency(ctx, q)
	if err != nil {
		return View{}, err
	}
	test := snap.test
	if test != nil {
		t := *test
		t.Fingerprint = ""
		test = &t
	}
	return View{
		Version: r.Version,
		Access:  AccessView{Mode: r.AuthMode, RegistrationEnabled: r.RegistrationEnabled, DefaultRole: r.DefaultRole},
		SSO: SSOView{
			Issuer: r.OidcIssuer, DiscoveryURL: r.OidcDiscoveryUrl, ClientID: r.OidcClientID, ClientSecretSet: snap.clientSecret != "",
			RedirectURL: s.h.env.Auth.PublicURL + CallbackPath, Configured: s.h.SSOConfigured(), Test: test, Tested: s.h.SSOTested(),
		},
		Storage: StorageView{
			Kind: r.FilesKind, Endpoint: r.S3Endpoint, Bucket: r.S3Bucket, Region: r.S3Region, AccessKey: r.S3AccessKey,
			SecretKeySet: snap.s3Secret != "", UseSSL: r.S3UseSsl, FilesCount: files, Locked: files > 0,
		},
		Appearance:         Theme{Palette: r.ThemePalette, Primary: r.ThemePrimary, Radius: r.ThemeRadius},
		MCP:                MCPView{Enabled: r.McpEnabled},
		EmergencyAvailable: emergency,
		EmergencyEmail:     s.h.env.Auth.BootstrapAdminEmail,
	}, nil
}

// emergency: there is an initial administrator with a local password (RF-23).
func (s *Service) emergency(ctx context.Context, q *sqlc.Queries) (bool, error) {
	email := s.h.env.Auth.BootstrapAdminEmail
	if email == "" {
		return false, nil
	}
	ok, err := q.HasLocalCredential(ctx, email)
	if err != nil {
		return false, fmt.Errorf("leer acceso de emergencia: %w", err)
	}
	return ok, nil
}

// draft is the row being changed, with the secrets in clear.
type draft struct {
	row          sqlc.SystemSetting
	clientSecret string
	s3Secret     string
}

// update changes the row under lock, audits the fields that changed (the
// secrets only as "changed") and applies it at once (RF-3, RF-4).
func (s *Service) update(ctx context.Context, p *authz.Principal, section string, fn func(q *sqlc.Queries, d *draft) error) (View, error) {
	if !p.Has(authz.SystemManage) {
		return View{}, apperr.ErrForbidden
	}
	var saved sqlc.SystemSetting
	err := s.h.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		row, err := q.LockSystemSettings(ctx)
		if err != nil {
			return fmt.Errorf("bloquear configuración: %w", err)
		}
		before := draft{row: row}
		before.clientSecret, _ = s.h.box.open(row.OidcClientSecret)
		before.s3Secret, _ = s.h.box.open(row.S3SecretKey)
		d := before
		if err := fn(q, &d); err != nil {
			return err
		}
		clientSecret, err := s.h.box.seal(d.clientSecret)
		if err != nil {
			return err
		}
		s3Secret, err := s.h.box.seal(d.s3Secret)
		if err != nil {
			return err
		}
		r := d.row
		saved, err = q.UpdateSystemSettings(ctx, sqlc.UpdateSystemSettingsParams{
			AuthMode: r.AuthMode, RegistrationEnabled: r.RegistrationEnabled, DefaultRole: r.DefaultRole,
			OidcIssuer: r.OidcIssuer, OidcDiscoveryUrl: r.OidcDiscoveryUrl, OidcClientID: r.OidcClientID, OidcClientSecret: clientSecret, OidcTest: r.OidcTest,
			FilesKind: r.FilesKind, S3Endpoint: r.S3Endpoint, S3Bucket: r.S3Bucket, S3Region: r.S3Region, S3AccessKey: r.S3AccessKey,
			S3SecretKey: s3Secret, S3UseSsl: r.S3UseSsl, ThemePalette: r.ThemePalette, ThemePrimary: r.ThemePrimary, ThemeRadius: r.ThemeRadius,
		})
		if err != nil {
			return fmt.Errorf("guardar configuración: %w", err)
		}
		old, now := auditable(before), auditable(d)
		for k, v := range now {
			if old[k] == v {
				delete(old, k)
				delete(now, k)
			}
		}
		if len(now) == 0 {
			return nil
		}
		return audit.Record(ctx, q, audit.Entry{
			Actor: audit.ByUser(p), Action: "system.settings_changed", TargetType: "system", TargetID: section, Before: old, After: now,
		})
	})
	if err != nil {
		return View{}, err
	}
	s.h.set(saved)
	return s.View(ctx)
}

// auditable lists the fields for the audit, with the secrets reduced to a
// hash that only tells whether they changed (RF-4, RF-6).
func auditable(d draft) map[string]any {
	r := d.row
	secret := func(v string) string {
		if v == "" {
			return ""
		}
		return "changed:" + Fingerprint(config.OIDCConfig{ClientSecret: v})
	}
	return map[string]any{
		"auth_mode": r.AuthMode, "registration_enabled": r.RegistrationEnabled, "default_role": r.DefaultRole,
		"oidc_issuer": r.OidcIssuer, "oidc_discovery_url": r.OidcDiscoveryUrl, "oidc_client_id": r.OidcClientID,
		"oidc_client_secret": secret(d.clientSecret),
		"files_kind":         r.FilesKind, "s3_endpoint": r.S3Endpoint, "s3_bucket": r.S3Bucket, "s3_region": r.S3Region,
		"s3_access_key": r.S3AccessKey, "s3_secret_key": secret(d.s3Secret), "s3_use_ssl": r.S3UseSsl,
		"theme_palette": r.ThemePalette, "theme_primary": r.ThemePrimary, "theme_radius": r.ThemeRadius,
		"mcp_enabled": r.McpEnabled,
	}
}

type AccessRequest struct {
	Mode                string `json:"mode" validate:"oneof=local oidc both"`
	RegistrationEnabled bool   `json:"registration_enabled"`
	DefaultRole         string `json:"default_role" validate:"omitempty,oneof=viewer member lead"`
	AcceptNoEmergency   bool   `json:"accept_no_emergency"`
}

// UpdateAccess changes the mode, the registration and the default role
// (RF-11 to RF-16, RF-23).
func (s *Service) UpdateAccess(ctx context.Context, p *authz.Principal, req AccessRequest) (View, error) {
	return s.update(ctx, p, "access", func(q *sqlc.Queries, d *draft) error {
		if req.Mode != config.AuthModeLocal && !s.h.SSOConfigured() {
			return errSSONotConfigured
		}
		if req.Mode == config.AuthModeOIDC && d.row.AuthMode != config.AuthModeOIDC {
			if !s.h.SSOTested() {
				return errSSONotTested
			}
			ok, err := s.emergency(ctx, q)
			if err != nil {
				return err
			}
			if !ok && !req.AcceptNoEmergency {
				return errNoEmergency
			}
		}
		d.row.AuthMode, d.row.RegistrationEnabled, d.row.DefaultRole = req.Mode, req.RegistrationEnabled, req.DefaultRole
		return nil
	})
}

type SSORequest struct {
	Issuer       string `json:"issuer" validate:"required,url,max=500"`
	DiscoveryURL string `json:"discovery_url" validate:"omitempty,url,max=500"`
	ClientID     string `json:"client_id" validate:"required,max=255"`
	ClientSecret string `json:"client_secret" validate:"max=1000"`
}

func (r *SSORequest) Normalize() {
	r.Issuer, r.DiscoveryURL, r.ClientID = strings.TrimSpace(r.Issuer), strings.TrimSpace(r.DiscoveryURL), strings.TrimSpace(r.ClientID)
}

// UpdateSSO checks the discovery before saving (RF-24 to RF-27, RF-31, RF-32).
func (s *Service) UpdateSSO(ctx context.Context, p *authz.Principal, req SSORequest) (View, error) {
	return s.update(ctx, p, "sso", func(_ *sqlc.Queries, d *draft) error {
		if d.row.AuthMode == config.AuthModeOIDC {
			return errSSOInUse
		}
		secret := req.ClientSecret
		if secret == "" {
			secret = d.clientSecret // RF-7
		}
		if secret == "" {
			return apperr.ErrValidation.WithDetail("field", "client_secret")
		}
		o := s.h.env.OIDC
		o.Issuer, o.DiscoveryURL, o.ClientID, o.ClientSecret = req.Issuer, req.DiscoveryURL, req.ClientID, secret
		if o.DiscoveryURL == "" {
			o.DiscoveryURL = o.Issuer
		}
		if err := s.CheckDiscovery(ctx, o); err != nil {
			return errDiscovery(err.Error())
		}
		before := Fingerprint(s.h.OIDC())
		d.row.OidcIssuer, d.row.OidcDiscoveryUrl, d.row.OidcClientID, d.clientSecret = req.Issuer, req.DiscoveryURL, req.ClientID, secret
		if Fingerprint(o) != before {
			d.row.OidcTest = nil // RF-31
		}
		return nil
	})
}

type StorageRequest struct {
	Kind      string `json:"kind" validate:"oneof=db s3"`
	Endpoint  string `json:"endpoint" validate:"max=255"`
	Bucket    string `json:"bucket" validate:"max=255"`
	Region    string `json:"region" validate:"max=100"`
	AccessKey string `json:"access_key" validate:"max=255"`
	SecretKey string `json:"secret_key" validate:"max=1000"`
	UseSSL    bool   `json:"use_ssl"`
}

func (r *StorageRequest) Normalize() {
	r.Endpoint, r.Bucket, r.Region, r.AccessKey = strings.TrimSpace(r.Endpoint), strings.TrimSpace(r.Bucket), strings.TrimSpace(r.Region), strings.TrimSpace(r.AccessKey)
}

// UpdateStorage checks the S3 before saving and keeps the place of the files
// while there are any (RF-33 to RF-38).
func (s *Service) UpdateStorage(ctx context.Context, p *authz.Principal, req StorageRequest) (View, error) {
	return s.update(ctx, p, "storage", func(q *sqlc.Queries, d *draft) error {
		files, err := q.CountLiveFiles(ctx)
		if err != nil {
			return fmt.Errorf("contar archivos: %w", err)
		}
		r := &d.row
		if files > 0 && (req.Kind != r.FilesKind || (req.Kind == "s3" && (req.Endpoint != r.S3Endpoint || req.Bucket != r.S3Bucket))) {
			return errStorageLocked.WithDetail("files", fmt.Sprint(files))
		}
		if req.Kind == "db" {
			r.FilesKind = "db"
			return nil
		}
		secret := req.SecretKey
		if secret == "" {
			secret = d.s3Secret // RF-7
		}
		switch {
		case req.Endpoint == "" || strings.Contains(req.Endpoint, "://"):
			return apperr.ErrValidation.WithDetail("field", "endpoint")
		case req.Bucket == "":
			return apperr.ErrValidation.WithDetail("field", "bucket")
		case req.AccessKey == "":
			return apperr.ErrValidation.WithDetail("field", "access_key")
		case secret == "":
			return apperr.ErrValidation.WithDetail("field", "secret_key")
		}
		f := config.FilesConfig{Endpoint: req.Endpoint, Bucket: req.Bucket, Region: req.Region, AccessKey: req.AccessKey, SecretKey: secret, UseSSL: req.UseSSL}
		if step, err := s.CheckS3(ctx, f); err != nil {
			return errS3(step)
		}
		r.FilesKind, r.S3Endpoint, r.S3Bucket, r.S3Region, r.S3AccessKey, r.S3UseSsl = "s3", req.Endpoint, req.Bucket, req.Region, req.AccessKey, req.UseSSL
		d.s3Secret = secret
		return nil
	})
}

type AppearanceRequest struct {
	Palette string `json:"palette"`
	Primary string `json:"primary"`
	Radius  string `json:"radius"`
}

// UpdateAppearance validates against the lists (RF-39, RF-40, RF-43).
func (s *Service) UpdateAppearance(ctx context.Context, p *authz.Principal, req AppearanceRequest) (View, error) {
	for field, ok := range map[string]bool{"palette": contains(Palettes, req.Palette), "primary": contains(Primaries, req.Primary), "radius": contains(Radii, req.Radius)} {
		if !ok {
			return View{}, apperr.ErrValidation.WithDetail("field", field)
		}
	}
	return s.update(ctx, p, "appearance", func(_ *sqlc.Queries, d *draft) error {
		d.row.ThemePalette, d.row.ThemePrimary, d.row.ThemeRadius = req.Palette, req.Primary, req.Radius
		return nil
	})
}

type MCPRequest struct {
	Enabled bool `json:"enabled"`
}

// UpdateMCP turns the AI connections on or off for everybody; the
// connections are kept either way (spec 016, RF-2 to RF-7).
func (s *Service) UpdateMCP(ctx context.Context, p *authz.Principal, req MCPRequest) (View, error) {
	return s.update(ctx, p, "mcp", func(q *sqlc.Queries, d *draft) error {
		d.row.McpEnabled = req.Enabled
		if err := q.SetMCPEnabled(ctx, req.Enabled); err != nil {
			return fmt.Errorf("guardar acceso por MCP: %w", err)
		}
		return nil
	})
}

// RecordSSOTest keeps the result of a test login (RF-29); it does not move
// the version that other instances watch for the rest of the settings, but
// it is saved like any other change.
func (s *Service) RecordSSOTest(ctx context.Context, t SSOTest) error {
	t.At = time.Now().UTC()
	t.Fingerprint = Fingerprint(s.h.OIDC())
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	var saved sqlc.SystemSetting
	err = s.h.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		row, err := q.LockSystemSettings(ctx)
		if err != nil {
			return fmt.Errorf("bloquear configuración: %w", err)
		}
		saved, err = q.UpdateSystemSettings(ctx, sqlc.UpdateSystemSettingsParams{
			AuthMode: row.AuthMode, RegistrationEnabled: row.RegistrationEnabled, DefaultRole: row.DefaultRole,
			OidcIssuer: row.OidcIssuer, OidcDiscoveryUrl: row.OidcDiscoveryUrl, OidcClientID: row.OidcClientID, OidcClientSecret: row.OidcClientSecret, OidcTest: b,
			FilesKind: row.FilesKind, S3Endpoint: row.S3Endpoint, S3Bucket: row.S3Bucket, S3Region: row.S3Region, S3AccessKey: row.S3AccessKey,
			S3SecretKey: row.S3SecretKey, S3UseSsl: row.S3UseSsl, ThemePalette: row.ThemePalette, ThemePrimary: row.ThemePrimary, ThemeRadius: row.ThemeRadius,
		})
		if err != nil {
			return fmt.Errorf("guardar prueba de SSO: %w", err)
		}
		return audit.Record(ctx, q, audit.Entry{
			Actor: audit.System, Action: "system.sso_tested", TargetType: "system", TargetID: "sso",
			After: map[string]any{"ok": t.OK, "email": t.Email, "error": t.Error},
		})
	})
	if err != nil {
		return err
	}
	s.h.set(saved)
	return nil
}

// checkDiscovery reads the IdP metadata within 5 s (RF-26).
func checkDiscovery(ctx context.Context, o config.OIDCConfig) error {
	ctx, cancel := context.WithTimeout(ctx, discoveryTimeout)
	defer cancel()
	ctx = oidc.ClientContext(ctx, &http.Client{Timeout: discoveryTimeout})
	if o.DiscoveryURL != o.Issuer {
		ctx = oidc.InsecureIssuerURLContext(ctx, o.Issuer)
	}
	if _, err := oidc.NewProvider(ctx, o.DiscoveryURL); err != nil {
		return err
	}
	return nil
}

// checkS3 writes, reads and removes an object within 10 s (RF-35, RF-36).
func checkS3(ctx context.Context, f config.FilesConfig) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, s3CheckTimeout)
	defer cancel()
	client, err := minio.New(f.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(f.AccessKey, f.SecretKey, ""), Secure: f.UseSSL, Region: f.Region})
	if err != nil {
		return "connect", err
	}
	key := "kairos-check/" + time.Now().UTC().Format("20060102T150405.000000000")
	data := []byte("kairos")
	if _, err := client.PutObject(ctx, f.Bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{}); err != nil {
		return "write", err
	}
	obj, err := client.GetObject(ctx, f.Bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return "read", err
	}
	got, err := io.ReadAll(obj)
	_ = obj.Close()
	if err != nil || !bytes.Equal(got, data) {
		return "read", fmt.Errorf("contenido distinto: %w", err)
	}
	if err := client.RemoveObject(ctx, f.Bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return "delete", err
	}
	return "", nil
}
