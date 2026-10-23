package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/authz"
	"openscum/internal/config"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
)

// NormalizeEmail compares emails ignoring case and outer spaces (RF-10).
func NormalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// IsUniqueViolation reports a duplicate key error.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

type Service struct {
	cfg     config.AuthConfig
	store   *db.Store
	loader  *PrincipalLoader
	hasher  *Hasher
	limiter *RateLimiter
	logger  *slog.Logger
}

func NewService(cfg config.AuthConfig, store *db.Store, loader *PrincipalLoader, hasher *Hasher, limiter *RateLimiter, logger *slog.Logger) *Service {
	return &Service{cfg: cfg, store: store, loader: loader, hasher: hasher, limiter: limiter, logger: logger}
}

// Login checks a local email and password (RF-7 to RF-10, RF-46).
func (s *Service) Login(ctx context.Context, email, password, ip string) (uuid.UUID, error) {
	if !s.cfg.LocalEnabled() {
		return uuid.Nil, errLocalAuthDisabled
	}
	email = NormalizeEmail(email)
	keys := []string{emailKey(email), ipKey(ip)}
	if !s.limiter.Allowed(keys...) {
		return uuid.Nil, errTooManyAttempts
	}

	u, hash, err := s.credentials(ctx, email)
	if err != nil {
		return uuid.Nil, err
	}
	// Verify runs even for unknown emails so both failures look the same (RF-8).
	if !s.hasher.Verify(password, hash) {
		s.limiter.Fail(keys...)
		return uuid.Nil, errInvalidCredentials
	}
	s.limiter.Reset(emailKey(email))

	if u.Status != "active" {
		return uuid.Nil, apperr.ErrAccountInactive
	}
	if err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		return s.ApplyBootstrap(ctx, q, u.ID, u.EmailNormalized)
	}); err != nil {
		return uuid.Nil, err
	}
	return u.ID, nil
}

// credentials returns the user and its password hash; hash is "" when either is missing.
func (s *Service) credentials(ctx context.Context, email string) (sqlc.User, string, error) {
	u, err := s.store.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, "", nil
	}
	if err != nil {
		return u, "", fmt.Errorf("buscar usuario: %w", err)
	}
	cred, err := s.store.GetCredential(ctx, u.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, "", nil
	}
	if err != nil {
		return u, "", fmt.Errorf("buscar credencial: %w", err)
	}
	return u, cred.PasswordHash, nil
}

// Register creates an active user with the default role and no team (RF-11 to RF-14).
func (s *Service) Register(ctx context.Context, req registerRequest) (uuid.UUID, error) {
	if !s.cfg.LocalEnabled() {
		return uuid.Nil, errLocalAuthDisabled
	}
	if !s.cfg.RegistrationEnabled {
		return uuid.Nil, errRegistrationDisabled
	}
	if err := CheckPasswordLength(req.Password); err != nil {
		return uuid.Nil, err
	}
	hash, err := s.hasher.Hash(req.Password)
	if err != nil {
		return uuid.Nil, err
	}

	var id uuid.UUID
	err = s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		u, err := q.CreateUser(ctx, sqlc.CreateUserParams{
			Name: strings.TrimSpace(req.Name), Email: strings.TrimSpace(req.Email),
			EmailNormalized: NormalizeEmail(req.Email), Status: "active",
		})
		if IsUniqueViolation(err) {
			return errEmailConflict
		}
		if err != nil {
			return fmt.Errorf("crear usuario: %w", err)
		}
		id = u.ID
		if err := q.UpsertCredential(ctx, sqlc.UpsertCredentialParams{UserID: u.ID, PasswordHash: hash}); err != nil {
			return fmt.Errorf("guardar credencial: %w", err)
		}
		if err := audit.Record(ctx, q, audit.Entry{
			Actor: audit.Actor{ID: &u.ID, Kind: audit.ActorUser}, Action: "user.registered",
			TargetType: "user", TargetID: u.ID.String(), After: map[string]string{"name": u.Name, "email": u.Email, "status": u.Status},
		}); err != nil {
			return err
		}
		if s.cfg.DefaultRole != "" {
			if err := AddRole(ctx, q, audit.System, u.ID, s.cfg.DefaultRole, "manual"); err != nil {
				return err
			}
		}
		return s.ApplyBootstrap(ctx, q, u.ID, u.EmailNormalized)
	})
	return id, err
}

// ChangeOwnPassword replaces the user's local password after checking the current one (RF-80, RF-82).
func (s *Service) ChangeOwnPassword(ctx context.Context, p *authz.Principal, current, next string) error {
	if !s.cfg.LocalEnabled() {
		return errLocalAuthDisabled
	}
	if !p.Has(authz.SelfPassword) {
		return apperr.ErrForbidden
	}
	cred, err := s.store.GetCredential(ctx, p.UserID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("buscar credencial: %w", err)
	}
	if !s.hasher.Verify(current, cred.PasswordHash) {
		return errWrongPassword
	}
	if err := CheckPasswordLength(next); err != nil {
		return err
	}
	hash, err := s.hasher.Hash(next)
	if err != nil {
		return err
	}
	return s.store.UpsertCredential(ctx, sqlc.UpsertCredentialParams{UserID: p.UserID, PasswordHash: hash})
}

// ApplyBootstrap makes the bootstrap email an admin while no active admin
// exists (RF-54 to RF-56).
func (s *Service) ApplyBootstrap(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, email string) error {
	if s.cfg.BootstrapAdminEmail == "" || email != s.cfg.BootstrapAdminEmail {
		return nil
	}
	if err := q.LockAdmins(ctx); err != nil {
		return fmt.Errorf("bloquear administradores: %w", err)
	}
	n, err := q.CountActiveAdmins(ctx)
	if err != nil {
		return fmt.Errorf("contar administradores: %w", err)
	}
	if n > 0 {
		return nil
	}
	s.logger.Info("bootstrap: se asigna el rol admin", slog.String("user_id", userID.String()))
	if err := AddRole(ctx, q, audit.System, userID, string(authz.RoleAdmin), "manual"); err != nil {
		return err
	}
	s.loader.Invalidate(userID)
	return nil
}

// AddRole assigns a role with its source and audits it. A manual assignment
// upgrades an IdP one; an IdP assignment never touches a manual one (RF-32).
func AddRole(ctx context.Context, q *sqlc.Queries, actor audit.Actor, userID uuid.UUID, role, source string) error {
	before, err := q.GetUserRole(ctx, sqlc.GetUserRoleParams{UserID: userID, Role: role})
	existed := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("leer rol: %w", err)
	}
	if existed && (before.Source == source || before.Source == "manual") {
		return nil
	}
	if source == "manual" {
		err = q.AddManualRole(ctx, sqlc.AddManualRoleParams{UserID: userID, Role: role})
	} else {
		_, err = q.AddIdpRole(ctx, sqlc.AddIdpRoleParams{UserID: userID, Role: role})
	}
	if err != nil {
		return fmt.Errorf("asignar rol: %w", err)
	}
	var prev any
	if existed {
		prev = map[string]string{"role": role, "source": before.Source}
	}
	return audit.Record(ctx, q, audit.Entry{
		Actor: actor, Action: "user.role_added", TargetType: "user", TargetID: userID.String(),
		Before: prev, After: map[string]string{"role": role, "source": source},
	})
}
