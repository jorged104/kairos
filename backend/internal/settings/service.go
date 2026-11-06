package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	"openscum/internal/llm"
	"openscum/internal/mail"
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
	// Mailer sends the test mail (spec 019, RF-6).
	Mailer mail.Sender
	// AIClient checks the AI service (spec 020, RF-11); nil, the default one.
	AIClient *http.Client
}

func NewService(h *Holder) *Service {
	return &Service{h: h, CheckDiscovery: checkDiscovery, CheckS3: checkS3, Mailer: mail.SMTP{}}
}

// MailView is the outgoing mail without its password (spec 019, RF-2, RF-4).
type MailView struct {
	Enabled     bool   `json:"enabled"`
	Host        string `json:"host"`
	Port        int32  `json:"port"`
	Security    string `json:"security"`
	Username    string `json:"username"`
	PasswordSet bool   `json:"password_set"`
	FromEmail   string `json:"from_email"`
	FromName    string `json:"from_name"`
	Ready       bool   `json:"ready"`
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

// AIView is the AI service of the reports without its key (spec 020, RF-1
// to RF-8). Ready also needs the agent service (ADR 0004).
type AIView struct {
	Enabled        bool   `json:"enabled"`
	BaseURL        string `json:"base_url"`
	Model          string `json:"model"`
	APIKeySet      bool   `json:"api_key_set"`
	AgentAvailable bool   `json:"agent_available"`
	Ready          bool   `json:"ready"`
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
	Mail               MailView    `json:"mail"`
	AI                 AIView      `json:"ai"`
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
		Appearance: Theme{Palette: r.ThemePalette, Primary: r.ThemePrimary, Radius: r.ThemeRadius},
		MCP:        MCPView{Enabled: r.McpEnabled},
		Mail: MailView{
			Enabled: r.MailEnabled, Host: r.SmtpHost, Port: r.SmtpPort, Security: r.SmtpSecurity, Username: r.SmtpUsername,
			PasswordSet: snap.mailPassword != "", FromEmail: r.MailFromEmail, FromName: r.MailFromName, Ready: s.h.Mail().Ready(),
		},
		AI: AIView{
			Enabled: r.AiEnabled, BaseURL: r.AiBaseUrl, Model: r.AiModel, APIKeySet: snap.aiKey != "",
			AgentAvailable: s.h.AgentURL() != "", Ready: s.h.AI().Ready() && s.h.AgentURL() != "",
		},
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
	mailPassword string
	aiKey        string
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
		before.mailPassword, _ = s.h.box.open(row.SmtpPassword)
		before.aiKey, _ = s.h.box.open(row.AiApiKey)
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
		"mcp_enabled":  r.McpEnabled,
		"mail_enabled": r.MailEnabled, "smtp_host": r.SmtpHost, "smtp_port": r.SmtpPort, "smtp_security": r.SmtpSecurity,
		"smtp_username": r.SmtpUsername, "smtp_password": secret(d.mailPassword),
		"mail_from_email": r.MailFromEmail, "mail_from_name": r.MailFromName,
		"ai_enabled": r.AiEnabled, "ai_base_url": r.AiBaseUrl, "ai_model": r.AiModel, "ai_api_key": secret(d.aiKey),
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

type MailRequest struct {
	Enabled   bool   `json:"enabled"`
	Host      string `json:"host" validate:"max=255"`
	Port      int32  `json:"port" validate:"min=1,max=65535"`
	Security  string `json:"security" validate:"oneof=none starttls tls"`
	Username  string `json:"username" validate:"max=255"`
	Password  string `json:"password" validate:"max=500"` // empty keeps the one stored (RF-7 of spec 015)
	FromEmail string `json:"from_email" validate:"max=320"`
	FromName  string `json:"from_name" validate:"max=100"`
}

func (r *MailRequest) Normalize() {
	r.Host, r.Username = strings.TrimSpace(r.Host), strings.TrimSpace(r.Username)
	r.FromEmail, r.FromName = strings.TrimSpace(r.FromEmail), strings.TrimSpace(r.FromName)
}

// UpdateMail saves the outgoing mail; switched on, it needs a server and a
// valid sender (spec 019, RF-2 to RF-5, RF-8).
func (s *Service) UpdateMail(ctx context.Context, p *authz.Principal, req MailRequest) (View, error) {
	if req.FromEmail != "" && !mail.ValidAddress(req.FromEmail) {
		return View{}, apperr.ErrValidation.WithDetail("field", "from_email")
	}
	if req.Enabled && req.Host == "" {
		return View{}, apperr.ErrValidation.WithDetail("field", "host")
	}
	if req.Enabled && req.FromEmail == "" {
		return View{}, apperr.ErrValidation.WithDetail("field", "from_email")
	}
	return s.update(ctx, p, "mail", func(q *sqlc.Queries, d *draft) error {
		if req.Password != "" {
			d.mailPassword = req.Password
		}
		sealed, err := s.h.box.seal(d.mailPassword)
		if err != nil {
			return err
		}
		r := &d.row
		r.MailEnabled, r.SmtpHost, r.SmtpPort, r.SmtpSecurity = req.Enabled, req.Host, req.Port, req.Security
		r.SmtpUsername, r.MailFromEmail, r.MailFromName = req.Username, req.FromEmail, req.FromName
		if err := q.SetMailSettings(ctx, sqlc.SetMailSettingsParams{
			MailEnabled: r.MailEnabled, SmtpHost: r.SmtpHost, SmtpPort: r.SmtpPort, SmtpSecurity: r.SmtpSecurity,
			SmtpUsername: r.SmtpUsername, SmtpPassword: sealed, MailFromEmail: r.MailFromEmail, MailFromName: r.MailFromName,
		}); err != nil {
			return fmt.Errorf("guardar correo: %w", err)
		}
		return nil
	})
}

func errMail(step string) *apperr.Error {
	return apperr.New(http.StatusBadRequest, "mail_test_failed", "No se pudo enviar el correo de prueba").WithDetail("step", step)
}

// TestMail sends a mail to the administrator with the saved settings, and
// says the step where it failed (spec 019, RF-6, RF-7).
func (s *Service) TestMail(ctx context.Context, p *authz.Principal) error {
	if !p.Has(authz.SystemManage) {
		return apperr.ErrForbidden
	}
	cfg := s.h.Mail()
	if cfg.Host == "" || cfg.FromEmail == "" {
		return apperr.New(http.StatusConflict, "mail_not_configured", "El correo no está configurado")
	}
	u, err := s.h.store.GetUser(ctx, p.UserID)
	if err != nil {
		return fmt.Errorf("leer usuario: %w", err)
	}
	err = s.Mailer.Send(ctx, cfg, mail.Message{
		To: u.Email, Subject: "Kairos: correo de prueba",
		Text: "Este es un correo de prueba de Kairos. Si lo lees, el correo saliente funciona.\n",
		HTML: "<p>Este es un correo de prueba de Kairos. Si lo lees, el correo saliente funciona.</p>",
	})
	var me *mail.Error
	if errors.As(err, &me) {
		return errMail(me.Step)
	}
	if err != nil {
		return errMail(mail.StepConnect)
	}
	return nil
}

type AIRequest struct {
	Enabled bool   `json:"enabled"`
	BaseURL string `json:"base_url" validate:"omitempty,url,max=500"`
	APIKey  string `json:"api_key" validate:"max=1000"` // empty keeps the one stored (RF-9)
	Model   string `json:"model" validate:"max=200"`
}

func (r *AIRequest) Normalize() {
	r.BaseURL, r.Model, r.APIKey = strings.TrimRight(strings.TrimSpace(r.BaseURL), "/"), strings.TrimSpace(r.Model), strings.TrimSpace(r.APIKey)
}

// UpdateAI saves the AI service of the reports; switched on, it needs an
// address and a model (spec 020, RF-3 to RF-10).
func (s *Service) UpdateAI(ctx context.Context, p *authz.Principal, req AIRequest) (View, error) {
	if req.BaseURL != "" && !strings.HasPrefix(req.BaseURL, "http://") && !strings.HasPrefix(req.BaseURL, "https://") {
		return View{}, apperr.ErrValidation.WithDetail("field", "base_url")
	}
	if req.Enabled && req.BaseURL == "" {
		return View{}, apperr.ErrValidation.WithDetail("field", "base_url")
	}
	if req.Enabled && req.Model == "" {
		return View{}, apperr.ErrValidation.WithDetail("field", "model")
	}
	return s.update(ctx, p, "ai", func(q *sqlc.Queries, d *draft) error {
		if req.APIKey != "" {
			d.aiKey = req.APIKey
		}
		sealed, err := s.h.box.seal(d.aiKey)
		if err != nil {
			return err
		}
		r := &d.row
		r.AiEnabled, r.AiBaseUrl, r.AiModel = req.Enabled, req.BaseURL, req.Model
		if err := q.SetAISettings(ctx, sqlc.SetAISettingsParams{
			AiEnabled: r.AiEnabled, AiBaseUrl: r.AiBaseUrl, AiApiKey: sealed, AiModel: r.AiModel,
		}); err != nil {
			return fmt.Errorf("guardar servicio de IA: %w", err)
		}
		return nil
	})
}

// TestAI checks the saved AI service and says where it failed (RF-11 to RF-13).
func (s *Service) TestAI(ctx context.Context, p *authz.Principal) error {
	if !p.Has(authz.SystemManage) {
		return apperr.ErrForbidden
	}
	cfg := s.h.AI()
	if cfg.BaseURL == "" || cfg.Model == "" {
		return apperr.New(http.StatusConflict, "ai_not_configured", "El servicio de IA no está configurado")
	}
	err := llm.Check(ctx, s.AIClient, cfg)
	var le *llm.Error
	if errors.As(err, &le) {
		s.h.logger.WarnContext(ctx, "prueba del servicio de IA fallida", "step", le.Step, "error", le.Err)
		return apperr.New(http.StatusBadRequest, "ai_test_failed", "No se pudo usar el servicio de IA").WithDetail("step", le.Step)
	}
	return err
}
