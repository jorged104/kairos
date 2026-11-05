// Package accounts sends the links to create or reset a password (spec 019):
// the invitation of a new person and the recovery from the login or from an
// administrator. A link is single use and only its hash is kept; when the mail
// is not configured or fails, the administrator gets it to pass it on.
package accounts

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/auth"
	"openscum/internal/config"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
	"openscum/internal/mail"
)

const (
	// InviteTTL and ResetTTL are how long a link lasts (RF-11, RF-26).
	InviteTTL = 7 * 24 * time.Hour
	ResetTTL  = time.Hour
	// MaxResets a person may ask for in an hour (RF-27).
	MaxResets = 3

	PurposeInvite = "invite"
	PurposeReset  = "reset"

	// PasswordPath is the page of a link; the secret goes in the fragment,
	// which no server ever receives, so it reaches no log (RF-39).
	PasswordPath = "/password#t="
)

var errInvalidLink = apperr.New(http.StatusBadRequest, "invalid_link", "El enlace no es válido o ha caducado")

// Delivery is what the administrator gets back from an invitation or a link
// (RF-13, RF-21, RF-33): whether the mail went out, and if not, the link.
type Delivery struct {
	Sent      bool       `json:"sent"`
	URL       string     `json:"url,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// Kind is "password" for a link to create or reset one, "login" for the
	// notice of a mode with SSO only (RF-15).
	Kind string `json:"kind"`
}

// Outgoing is a mail prepared inside a transaction and sent after it.
type Outgoing struct {
	message  mail.Message
	delivery Delivery
}

type Service struct {
	store   *db.Store
	hasher  *auth.Hasher
	authCfg func() config.AuthConfig
	mailCfg func() mail.Config
	logger  *slog.Logger
	now     func() time.Time
	// Mailer sends; Wait is how long an administrator waits for the answer
	// before getting the link anyway.
	Mailer mail.Sender
	Wait   time.Duration
}

func NewService(store *db.Store, hasher *auth.Hasher, authCfg func() config.AuthConfig, mailCfg func() mail.Config, logger *slog.Logger) *Service {
	return &Service{store: store, hasher: hasher, authCfg: authCfg, mailCfg: mailCfg, logger: logger, now: time.Now,
		Mailer: mail.SMTP{}, Wait: 1500 * time.Millisecond}
}

func hash(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

func (s *Service) base() string { return strings.TrimRight(s.authCfg().PublicURL, "/") }

// newLink makes a link of a purpose for a person; it voids the earlier ones of
// that purpose (RF-20, RF-28).
func (s *Service) newLink(ctx context.Context, q *sqlc.Queries, user uuid.UUID, purpose string, by *uuid.UUID, ttl time.Duration) (string, time.Time, error) {
	if err := q.RevokeUserTokens(ctx, sqlc.RevokeUserTokensParams{UserID: user, Purpose: &purpose}); err != nil {
		return "", time.Time{}, fmt.Errorf("anular enlaces: %w", err)
	}
	secret := mail.Secret()
	expires := s.now().Add(ttl)
	if _, err := q.CreateUserToken(ctx, sqlc.CreateUserTokenParams{UserID: user, Purpose: purpose, TokenHash: hash(secret), CreatedBy: by, ExpiresAt: expires}); err != nil {
		return "", time.Time{}, fmt.Errorf("crear enlace: %w", err)
	}
	return s.base() + PasswordPath + secret, expires, nil
}

// Invite prepares the invitation of a person, in the caller's transaction
// (RF-11, RF-14, RF-15, RF-35). by is the administrator.
func (s *Service) Invite(ctx context.Context, q *sqlc.Queries, by uuid.UUID, u sqlc.User) (Outgoing, error) {
	if !s.authCfg().LocalEnabled() {
		// SSO only: a notice with the way in, no password (RF-15).
		login := s.base() + "/login"
		return Outgoing{
			message: message(u.Email, "Ya tienes acceso a Kairos",
				"Hola, "+u.Name+":", "Ya tienes acceso a Kairos. Entra con la cuenta de tu organización desde este enlace:", login, ""),
			delivery: Delivery{URL: login, Kind: "login"},
		}, audit.Record(ctx, q, audit.Entry{Actor: actor(by), Action: "user.invitation_sent", TargetType: "user", TargetID: u.ID.String(),
			After: map[string]any{"kind": "login"}})
	}
	link, expires, err := s.newLink(ctx, q, u.ID, PurposeInvite, &by, InviteTTL)
	if err != nil {
		return Outgoing{}, err
	}
	out := Outgoing{
		message: message(u.Email, "Te han dado de alta en Kairos",
			"Hola, "+u.Name+":", "Te han dado de alta en Kairos. Crea tu contraseña desde este enlace, que vale durante 7 días y una sola vez:", link,
			"Si no esperabas este correo, puedes ignorarlo."),
		delivery: Delivery{URL: link, ExpiresAt: &expires, Kind: "password"},
	}
	return out, audit.Record(ctx, q, audit.Entry{Actor: actor(by), Action: "user.invitation_sent", TargetType: "user", TargetID: u.ID.String(),
		After: map[string]any{"kind": "password", "expires_at": expires}})
}

// ResetLink prepares a link of recovery that an administrator sends (RF-32, RF-35).
func (s *Service) ResetLink(ctx context.Context, q *sqlc.Queries, by uuid.UUID, u sqlc.User) (Outgoing, error) {
	link, expires, err := s.newLink(ctx, q, u.ID, PurposeReset, &by, ResetTTL)
	if err != nil {
		return Outgoing{}, err
	}
	out := Outgoing{
		message: message(u.Email, "Pon una contraseña nueva en Kairos",
			"Hola, "+u.Name+":", "Un administrador te envía un enlace para poner una contraseña nueva en Kairos. Vale durante 1 hora y una sola vez:", link,
			"Si no lo esperabas, avisa a tu administrador."),
		delivery: Delivery{URL: link, ExpiresAt: &expires, Kind: "password"},
	}
	return out, audit.Record(ctx, q, audit.Entry{Actor: actor(by), Action: "user.password_link_sent", TargetType: "user", TargetID: u.ID.String(),
		After: map[string]any{"expires_at": expires}})
}

// Deliver sends a prepared mail, waiting at most Wait: the administrator
// gets the link when the mail is off, fails or takes longer (RF-13).
func (s *Service) Deliver(ctx context.Context, out Outgoing) Delivery {
	d := out.delivery
	cfg := s.mailCfg()
	if !cfg.Ready() {
		return d
	}
	done := make(chan error, 1)
	go func() { done <- s.Mailer.Send(context.WithoutCancel(ctx), cfg, out.message) }()
	select {
	case err := <-done:
		if err != nil {
			s.logger.Warn("no se pudo enviar el correo", slog.String("error", err.Error()))
			return d
		}
		return Delivery{Sent: true, ExpiresAt: d.ExpiresAt, Kind: d.Kind}
	case <-time.After(s.Wait):
		return d // still sending; the link is there in case it never arrives
	}
}

// send is for mails nobody waits for (RF-26, RF-31).
func (s *Service) send(m mail.Message) {
	cfg := s.mailCfg()
	if !cfg.Ready() {
		return
	}
	go func() {
		if err := s.Mailer.Send(context.Background(), cfg, m); err != nil {
			s.logger.Warn("no se pudo enviar el correo", slog.String("error", err.Error()))
		}
	}()
}

func actor(id uuid.UUID) audit.Actor { return audit.Actor{ID: &id, Kind: audit.ActorUser} }

// Forgot sends a recovery link if the address is of an active person with a
// password, without telling whether it is (RF-24 to RF-28).
func (s *Service) Forgot(ctx context.Context, email string) error {
	cfg := s.authCfg()
	normalized := auth.NormalizeEmail(email)
	if normalized == "" || !s.mailCfg().Ready() {
		return nil
	}
	// SSO only: the initial administrator's emergency way in alone (RF-24).
	if !cfg.LocalEnabled() && normalized != auth.NormalizeEmail(cfg.BootstrapAdminEmail) {
		return nil
	}
	var out *mail.Message
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		u, err := q.GetUserByEmail(ctx, normalized)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("leer usuario: %w", err)
		}
		if u.Status != "active" {
			return nil
		}
		if _, err := q.GetCredential(ctx, u.ID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil // a pending invitation has no password to reset
			}
			return fmt.Errorf("leer credencial: %w", err)
		}
		recent, err := q.CountRecentResets(ctx, sqlc.CountRecentResetsParams{UserID: u.ID, Since: s.now().Add(-time.Hour)})
		if err != nil {
			return fmt.Errorf("contar enlaces: %w", err)
		}
		if recent >= MaxResets {
			return nil
		}
		link, _, err := s.newLink(ctx, q, u.ID, PurposeReset, nil, ResetTTL)
		if err != nil {
			return err
		}
		m := message(u.Email, "Restablece tu contraseña de Kairos",
			"Hola, "+u.Name+":", "Has pedido restablecer tu contraseña de Kairos. Ponla nueva desde este enlace, que vale durante 1 hora y una sola vez:", link,
			"Si no lo pediste, ignora este correo: tu contraseña no cambia.")
		out = &m
		return nil
	})
	if err != nil {
		return err
	}
	if out != nil {
		s.send(*out)
	}
	return nil
}

// Link is what the page of a link shows before asking for the password.
type Link struct {
	Purpose string `json:"purpose"`
	Email   string `json:"email"`
	Name    string `json:"name"`
}

// valid: unused, not voided, not expired, of an active person, and usable in
// the access mode (RF-18, RF-24, RF-30).
func (s *Service) valid(t sqlc.UserToken, status, email string) bool {
	if t.UsedAt != nil || t.RevokedAt != nil || !s.now().Before(t.ExpiresAt) || status != "active" {
		return false
	}
	cfg := s.authCfg()
	return cfg.LocalEnabled() || (t.Purpose == PurposeReset && auth.NormalizeEmail(email) == auth.NormalizeEmail(cfg.BootstrapAdminEmail))
}

// Check tells what a link is for, without using it.
func (s *Service) Check(ctx context.Context, secret string) (Link, error) {
	row, err := s.store.UserTokenByHash(ctx, hash(secret))
	if errors.Is(err, pgx.ErrNoRows) {
		return Link{}, errInvalidLink
	}
	if err != nil {
		return Link{}, fmt.Errorf("leer enlace: %w", err)
	}
	t := sqlc.UserToken{ID: row.ID, UserID: row.UserID, Purpose: row.Purpose, ExpiresAt: row.ExpiresAt, UsedAt: row.UsedAt, RevokedAt: row.RevokedAt}
	if !s.valid(t, row.Status, row.Email) {
		return Link{}, errInvalidLink
	}
	return Link{Purpose: row.Purpose, Email: row.Email, Name: row.Name}, nil
}

// Reset sets the password of a valid link and uses it up (RF-16, RF-17, RF-29, RF-36).
func (s *Service) Reset(ctx context.Context, secret, password string) error {
	if err := auth.CheckPasswordLength(password); err != nil {
		return err
	}
	pwHash, err := s.hasher.Hash(password)
	if err != nil {
		return err
	}
	var notice *mail.Message
	err = s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		t, err := q.UserTokenByHashForUpdate(ctx, hash(secret))
		if errors.Is(err, pgx.ErrNoRows) {
			return errInvalidLink
		}
		if err != nil {
			return fmt.Errorf("leer enlace: %w", err)
		}
		u, err := q.GetUser(ctx, t.UserID)
		if err != nil {
			return fmt.Errorf("leer usuario: %w", err)
		}
		if !s.valid(t, u.Status, u.Email) {
			return errInvalidLink
		}
		if err := q.UpsertCredential(ctx, sqlc.UpsertCredentialParams{UserID: u.ID, PasswordHash: pwHash}); err != nil {
			return fmt.Errorf("guardar credencial: %w", err)
		}
		if err := q.UseUserToken(ctx, t.ID); err != nil {
			return fmt.Errorf("usar enlace: %w", err)
		}
		purpose := t.Purpose
		if err := q.RevokeUserTokens(ctx, sqlc.RevokeUserTokensParams{UserID: u.ID, Purpose: &purpose}); err != nil {
			return fmt.Errorf("anular enlaces: %w", err)
		}
		if t.Purpose == PurposeReset {
			m := message(u.Email, "Tu contraseña de Kairos ha cambiado",
				"Hola, "+u.Name+":", "Tu contraseña de Kairos se acaba de cambiar. Puedes entrar aquí:", s.base()+"/login",
				"Si no fuiste tú, avisa enseguida a tu administrador.")
			notice = &m
		}
		return audit.Record(ctx, q, audit.Entry{Actor: actor(u.ID), Action: "user.password_set_by_link", TargetType: "user", TargetID: u.ID.String(),
			After: map[string]any{"purpose": t.Purpose}})
	})
	if err != nil {
		return err
	}
	if notice != nil {
		s.send(*notice) // RF-31
	}
	return nil
}

// Revoke voids every link of a person, e.g. when deactivated (RF-42).
func Revoke(ctx context.Context, q *sqlc.Queries, user uuid.UUID) error {
	if err := q.RevokeUserTokens(ctx, sqlc.RevokeUserTokensParams{UserID: user}); err != nil {
		return fmt.Errorf("anular enlaces: %w", err)
	}
	return nil
}

// message writes a mail in Spanish, as text and as HTML, never with a password (RF-40, RF-41).
func message(to, subject, greeting, body, link, footer string) mail.Message {
	text := greeting + "\n\n" + body + "\n\n" + link + "\n"
	if footer != "" {
		text += "\n" + footer + "\n"
	}
	text += "\n— Kairos\n"
	e := html.EscapeString
	h := "<p>" + e(greeting) + "</p><p>" + e(body) + `</p><p><a href="` + e(link) + `">` + e(link) + "</a></p>"
	if footer != "" {
		h += "<p>" + e(footer) + "</p>"
	}
	h += "<p>— Kairos</p>"
	return mail.Message{To: to, Subject: subject, Text: text, HTML: h}
}
