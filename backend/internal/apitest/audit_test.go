package apitest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/audit"
	"openscum/internal/db/sqlc"
)

func TestAuditIsPartOfTheTransaction(t *testing.T) {
	env := New(t)
	boom := errors.New("boom")
	err := env.Store.InTx(context.Background(), func(_ pgx.Tx, q *sqlc.Queries) error {
		if err := audit.Record(context.Background(), q, audit.Entry{Actor: audit.System, Action: "x", TargetType: "user", TargetID: "t1"}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if got := env.AuditActions("t1"); len(got) != 0 {
		t.Fatalf("audit kept %v after rollback", got)
	}
}

func TestAuditListFiltersAndPaginates(t *testing.T) {
	env := New(t)
	admin := env.CreateUser(UserSpec{Name: "Admin", Email: "admin@x.com", Roles: []string{"admin"}})
	target := uuid.New()
	ctx := context.Background()
	actor := audit.Actor{ID: &admin, Kind: audit.ActorUser}
	for i := range 3 {
		if err := audit.Record(ctx, env.Store.Queries, audit.Entry{Actor: actor, Action: fmt.Sprint("a", i), TargetType: "user", TargetID: target.String(), After: map[string]int{"i": i}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := audit.Record(ctx, env.Store.Queries, audit.Entry{Actor: audit.IdP, Action: "other", TargetType: "user", TargetID: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}

	var res struct {
		Items []struct {
			Action    string  `json:"action"`
			ActorKind string  `json:"actor_kind"`
			ActorName *string `json:"actor_name"`
		} `json:"items"`
		Total int `json:"total"`
	}
	c := env.As(admin)
	c.Get("/api/audit?user_id="+target.String()+"&page_size=2").Expect(t, http.StatusOK).JSON(t, &res)
	if res.Total != 3 || len(res.Items) != 2 || res.Items[0].Action != "a2" || *res.Items[0].ActorName != "Admin" {
		t.Fatalf("page 1 = %+v", res)
	}
	c.Get("/api/audit?actor_id="+admin.String()+"&from=2000-01-01&to=2999-12-31").Expect(t, http.StatusOK).JSON(t, &res)
	if res.Total != 3 {
		t.Fatalf("by actor total = %d, want 3", res.Total)
	}
	c.Get("/api/audit?to=2000-01-01").Expect(t, http.StatusOK).JSON(t, &res)
	if res.Total != 0 {
		t.Fatalf("old range total = %d, want 0", res.Total)
	}
	c.Get("/api/audit?user_id=nope").Expect(t, http.StatusBadRequest, "invalid_filter")
}

func TestAuditIsAdminOnly(t *testing.T) {
	env := New(t)
	for _, role := range []string{"lead", "member", "viewer"} {
		id := env.CreateUser(UserSpec{Email: role + "@x.com", Roles: []string{role}})
		env.As(id).Get("/api/audit").Expect(t, http.StatusForbidden, "forbidden")
	}
}
