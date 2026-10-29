package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const defaultPort = "8080"

// Authentication modes (RF-1).
const (
	AuthModeLocal = "local"
	AuthModeOIDC  = "oidc"
	AuthModeBoth  = "both"
)

// Sources for the roles of OIDC users (RF-33).
const (
	RoleSourceIdP   = "idp"
	RoleSourceLocal = "local"
)

const minSessionSecretBytes = 32

var validRoles = map[string]bool{"admin": true, "lead": true, "member": true, "viewer": true}

type Config struct {
	DatabaseURL string
	Port        string
	Auth        AuthConfig
	OIDC        OIDCConfig
	// Files is the S3 storage of page files; Endpoint empty keeps them in the database (RF-21, RF-22).
	Files FilesConfig
}

type FilesConfig struct {
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

// S3 reports whether page files go to an S3 storage.
func (f FilesConfig) S3() bool { return f.Endpoint != "" }

type AuthConfig struct {
	Mode                string
	SessionSecret       []byte
	PublicURL           string
	CookieSecure        bool
	RegistrationEnabled bool
	// DefaultRole is empty when users without a role must be rejected (RF-36).
	DefaultRole         string
	BootstrapAdminEmail string
	PasswordHashMemory  uint32
	PasswordHashTime    uint32
	TrustProxy          bool
}

type OIDCConfig struct {
	Issuer       string
	DiscoveryURL string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	RoleSource   string
	LinkByEmail  bool
}

// LocalEnabled reports whether password login is allowed (RF-4).
func (a AuthConfig) LocalEnabled() bool { return a.Mode != AuthModeOIDC }

// OIDCEnabled reports whether SSO login is allowed (RF-5).
func (a AuthConfig) OIDCEnabled() bool { return a.Mode != AuthModeLocal }

// Load reads the API configuration from environment variables.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Port:        os.Getenv("API_PORT"),
	}
	if cfg.DatabaseURL == "" {
		return cfg, errors.New("falta la variable de entorno DATABASE_URL")
	}
	if cfg.Port == "" {
		cfg.Port = defaultPort
	}

	auth, err := loadAuth()
	if err != nil {
		return cfg, err
	}
	cfg.Auth = auth

	if auth.OIDCEnabled() {
		oidc, err := loadOIDC()
		if err != nil {
			return cfg, err
		}
		cfg.OIDC = oidc
	}

	files, err := loadFiles()
	if err != nil {
		return cfg, err
	}
	cfg.Files = files
	return cfg, nil
}

// loadFiles reads the optional S3 storage. Half a configuration is an error,
// so a typo never sends the files to the database unnoticed.
func loadFiles() (FilesConfig, error) {
	f := FilesConfig{
		Endpoint:  os.Getenv("FILES_S3_ENDPOINT"),
		Bucket:    os.Getenv("FILES_S3_BUCKET"),
		Region:    os.Getenv("FILES_S3_REGION"),
		AccessKey: os.Getenv("FILES_S3_ACCESS_KEY"),
		SecretKey: os.Getenv("FILES_S3_SECRET_KEY"),
	}
	var errs []error
	f.UseSSL = boolEnv("FILES_S3_USE_SSL", true, &errs)
	if f.Endpoint == "" {
		if f.Bucket != "" || f.AccessKey != "" || f.SecretKey != "" {
			errs = append(errs, errors.New("falta FILES_S3_ENDPOINT para usar el almacenamiento S3"))
		}
		return FilesConfig{}, errors.Join(errs...)
	}
	if strings.Contains(f.Endpoint, "://") {
		errs = append(errs, fmt.Errorf("FILES_S3_ENDPOINT=%q debe ser host[:puerto], sin esquema", f.Endpoint))
	}
	for name, v := range map[string]string{
		"FILES_S3_BUCKET": f.Bucket, "FILES_S3_ACCESS_KEY": f.AccessKey, "FILES_S3_SECRET_KEY": f.SecretKey,
	} {
		if v == "" {
			errs = append(errs, fmt.Errorf("falta la variable de entorno %s", name))
		}
	}
	return f, errors.Join(errs...)
}

func loadAuth() (AuthConfig, error) {
	a := AuthConfig{Mode: os.Getenv("AUTH_MODE")}
	switch a.Mode {
	case AuthModeLocal, AuthModeOIDC, AuthModeBoth:
	case "":
		return a, errors.New("falta la variable de entorno AUTH_MODE (local, oidc o both)")
	default:
		return a, fmt.Errorf("AUTH_MODE=%q no es válido (local, oidc o both)", a.Mode)
	}

	secret := os.Getenv("SESSION_SECRET")
	if len(secret) < minSessionSecretBytes {
		return a, fmt.Errorf("SESSION_SECRET debe tener al menos %d bytes", minSessionSecretBytes)
	}
	a.SessionSecret = []byte(secret)

	publicURL, err := requiredURL("PUBLIC_URL")
	if err != nil {
		return a, err
	}
	a.PublicURL = strings.TrimRight(publicURL, "/")

	var errs []error
	a.CookieSecure = boolEnv("COOKIE_SECURE", true, &errs)
	a.RegistrationEnabled = boolEnv("REGISTRATION_ENABLED", false, &errs)
	a.TrustProxy = boolEnv("TRUST_PROXY", false, &errs)
	a.PasswordHashMemory = uintEnv("PASSWORD_HASH_MEMORY_KIB", 64*1024, &errs)
	a.PasswordHashTime = uintEnv("PASSWORD_HASH_TIME", 3, &errs)
	if err := errors.Join(errs...); err != nil {
		return a, err
	}

	a.DefaultRole = "member"
	if v, ok := os.LookupEnv("DEFAULT_ROLE"); ok {
		a.DefaultRole = strings.TrimSpace(v)
	}
	if a.DefaultRole != "" && !validRoles[a.DefaultRole] {
		return a, fmt.Errorf("DEFAULT_ROLE=%q no es un rol válido", a.DefaultRole)
	}
	a.BootstrapAdminEmail = strings.ToLower(strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_EMAIL")))
	return a, nil
}

func loadOIDC() (OIDCConfig, error) {
	o := OIDCConfig{
		Issuer:       os.Getenv("OIDC_ISSUER"),
		DiscoveryURL: os.Getenv("OIDC_DISCOVERY_URL"),
		ClientID:     os.Getenv("OIDC_CLIENT_ID"),
		ClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("OIDC_REDIRECT_URL"),
		RoleSource:   os.Getenv("OIDC_ROLE_SOURCE"),
	}
	for _, req := range []struct{ name, value string }{
		{"OIDC_ISSUER", o.Issuer},
		{"OIDC_CLIENT_ID", o.ClientID},
		{"OIDC_CLIENT_SECRET", o.ClientSecret},
		{"OIDC_REDIRECT_URL", o.RedirectURL},
	} {
		if req.value == "" {
			return o, fmt.Errorf("falta la variable de entorno %s (obligatoria con AUTH_MODE oidc o both)", req.name)
		}
	}
	if o.DiscoveryURL == "" {
		o.DiscoveryURL = o.Issuer
	}
	switch o.RoleSource {
	case "":
		o.RoleSource = RoleSourceIdP
	case RoleSourceIdP, RoleSourceLocal:
	default:
		return o, fmt.Errorf("OIDC_ROLE_SOURCE=%q no es válido (idp o local)", o.RoleSource)
	}
	var errs []error
	o.LinkByEmail = boolEnv("OIDC_LINK_BY_EMAIL", false, &errs)
	return o, errors.Join(errs...)
}

func requiredURL(name string) (string, error) {
	v := os.Getenv(name)
	if v == "" {
		return "", fmt.Errorf("falta la variable de entorno %s", name)
	}
	u, err := url.Parse(v)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("%s=%q no es una URL absoluta", name, v)
	}
	return v, nil
}

func boolEnv(name string, def bool, errs *[]error) bool {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s=%q no es un booleano", name, v))
		return def
	}
	return b
}

func uintEnv(name string, def uint32, errs *[]error) uint32 {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.ParseUint(v, 10, 32)
	if err != nil || n == 0 {
		*errs = append(*errs, fmt.Errorf("%s=%q no es un entero positivo", name, v))
		return def
	}
	return uint32(n)
}
