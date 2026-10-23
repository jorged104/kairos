package auth

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	SessionCookie = "kairos_session"
	// MethodCookie remembers how the session was opened, only to decide
	// whether logout must go through the IdP (RF-52).
	MethodCookie = "kairos_auth_method"
	// SessionTTL is fixed: the session is not renewed by activity (RF-41).
	SessionTTL = 8 * time.Hour

	MethodLocal = "local"
	MethodOIDC  = "oidc"
)

var errInvalidSession = errors.New("sesión no válida")

// Sessions issues and reads the Kairos session cookie: a JWT that only
// carries the user id and its expiry (RF-40).
type Sessions struct {
	secret []byte
	secure bool
	now    func() time.Time
}

func NewSessions(secret []byte, secure bool) *Sessions {
	return &Sessions{secret: secret, secure: secure, now: time.Now}
}

// Issue opens a session for the user.
func (s *Sessions) Issue(w http.ResponseWriter, userID uuid.UUID, method string) error {
	now := s.now()
	exp := now.Add(SessionTTL)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(exp),
	}).SignedString(s.secret)
	if err != nil {
		return fmt.Errorf("firmar sesión: %w", err)
	}
	http.SetCookie(w, s.cookie(SessionCookie, token, exp))
	http.SetCookie(w, s.cookie(MethodCookie, method, exp))
	return nil
}

// Parse returns the user of a valid, unexpired and untampered session.
func (s *Sessions) Parse(r *http.Request) (uuid.UUID, error) {
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		return uuid.Nil, errInvalidSession
	}
	var claims jwt.RegisteredClaims
	_, err = jwt.ParseWithClaims(c.Value, &claims, func(*jwt.Token) (any, error) { return s.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(s.now),
	)
	if err != nil {
		return uuid.Nil, errInvalidSession
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, errInvalidSession
	}
	return id, nil
}

// Method returns how the current session was opened.
func (s *Sessions) Method(r *http.Request) string {
	if c, err := r.Cookie(MethodCookie); err == nil {
		return c.Value
	}
	return ""
}

// Clear deletes the session cookies (RF-51).
func (s *Sessions) Clear(w http.ResponseWriter) {
	for _, name := range []string{SessionCookie, MethodCookie} {
		c := s.cookie(name, "", time.Unix(0, 0))
		c.MaxAge = -1
		http.SetCookie(w, c)
	}
}

func (s *Sessions) cookie(name, value string, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	}
}
