package db_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/pressly/goose/v3"

	"openscum/internal/db"
	"openscum/internal/db/dbtest"
	"openscum/internal/db/migrations"
	"openscum/internal/db/sqlc"
)

func TestAuditLogIsAppendOnly(t *testing.T) {
	ctx := context.Background()
	store := db.NewStore(dbtest.New(t))
	if err := store.InsertAudit(ctx, sqlc.InsertAuditParams{
		ActorKind: "system", Action: "test", TargetType: "user", TargetID: "x",
	}); err != nil {
		t.Fatalf("insert audit: %v", err)
	}
	if _, err := store.Pool.Exec(ctx, "UPDATE audit_log SET action = 'changed'"); err == nil {
		t.Fatal("UPDATE on audit_log succeeded, want it rejected (RF-119)")
	}
	if _, err := store.Pool.Exec(ctx, "DELETE FROM audit_log"); err == nil {
		t.Fatal("DELETE on audit_log succeeded, want it rejected (RF-119)")
	}
}

func TestIdentityIsUniquePerIssuerAndSubject(t *testing.T) {
	ctx := context.Background()
	store := db.NewStore(dbtest.New(t))
	a := createUser(t, store, "a@example.com")
	b := createUser(t, store, "b@example.com")

	if _, err := store.CreateIdentity(ctx, sqlc.CreateIdentityParams{UserID: a.ID, Issuer: "iss", Subject: "sub"}); err != nil {
		t.Fatalf("first identity: %v", err)
	}
	// ON CONFLICT DO NOTHING returns no row: the identity stays with the first user (RF-26).
	if _, err := store.CreateIdentity(ctx, sqlc.CreateIdentityParams{UserID: b.ID, Issuer: "iss", Subject: "sub"}); err == nil {
		t.Fatal("second identity with the same issuer+subject was created")
	}
	got, err := store.GetIdentity(ctx, sqlc.GetIdentityParams{Issuer: "iss", Subject: "sub"})
	if err != nil || got.UserID != a.ID {
		t.Fatalf("identity = %+v, %v; want it linked to the first user", got, err)
	}
}

func TestEmailIsUniqueAcrossStatuses(t *testing.T) {
	ctx := context.Background()
	store := db.NewStore(dbtest.New(t))
	u := createUser(t, store, "dup@example.com")
	if _, err := store.SetUserStatus(ctx, sqlc.SetUserStatusParams{ID: u.ID, Status: "offboarded"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateUser(ctx, sqlc.CreateUserParams{
		Name: "Dup", Email: "dup@example.com", EmailNormalized: "dup@example.com", Status: "active",
	}); err == nil {
		t.Fatal("created a second user with the email of an offboarded one (RF-71)")
	}
}

func TestMigrationsGoDownAndUpAgain(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.New(t)
	sqlDB, err := sql.Open("pgx", pool.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 0); err != nil {
		t.Fatalf("down: %v", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up again: %v", err)
	}
}

func createUser(t *testing.T, store *db.Store, email string) sqlc.User {
	t.Helper()
	u, err := store.CreateUser(context.Background(), sqlc.CreateUserParams{
		Name: email, Email: email, EmailNormalized: email, Status: "active",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}
