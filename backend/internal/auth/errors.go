package auth

import (
	"net/http"

	"openscum/internal/apperr"
)

var (
	errInvalidCredentials   = apperr.New(http.StatusUnauthorized, "invalid_credentials", "Email o contraseña incorrectos")
	errTooManyAttempts      = apperr.New(http.StatusTooManyRequests, "too_many_attempts", "Demasiados intentos; prueba más tarde")
	errLocalAuthDisabled    = apperr.New(http.StatusForbidden, "local_auth_disabled", "El login con contraseña no está habilitado")
	errOIDCDisabled         = apperr.New(http.StatusForbidden, "oidc_disabled", "El login con SSO no está habilitado")
	errRegistrationDisabled = apperr.New(http.StatusForbidden, "registration_disabled", "El registro no está habilitado")
	errPasswordTooShort     = apperr.New(http.StatusBadRequest, "password_too_short", "La contraseña debe tener al menos 6 caracteres").WithDetail("field", "password")
	errPasswordTooLong      = apperr.New(http.StatusBadRequest, "password_too_long", "La contraseña no puede tener más de 128 caracteres").WithDetail("field", "password")
	errWrongPassword        = apperr.New(http.StatusBadRequest, "wrong_password", "La contraseña actual no es correcta")
	errEmailConflict        = apperr.New(http.StatusConflict, "email_conflict", "El email ya pertenece a otro usuario")
	errNoRole               = apperr.New(http.StatusForbidden, "no_role", "El usuario no tiene ningún rol asignado")
	errInvalidState         = apperr.New(http.StatusBadRequest, "invalid_state", "La petición de login no es válida o ha caducado")
	errIdPUnavailable       = apperr.New(http.StatusBadGateway, "idp_unavailable", "El proveedor de identidad no responde")
	errIdPTokenInvalid      = apperr.New(http.StatusUnauthorized, "idp_token_invalid", "La respuesta del proveedor de identidad no es válida")
	errAccountConflict      = apperr.New(http.StatusConflict, "account_conflict", "Ya existe una cuenta con ese email")
	errEmailRequired        = apperr.New(http.StatusBadRequest, "email_required", "El proveedor de identidad no envió un email")
)

// PasswordError exposes the length errors to other packages (admin set password, user creation).
func PasswordError(password string) error { return CheckPasswordLength(password) }
