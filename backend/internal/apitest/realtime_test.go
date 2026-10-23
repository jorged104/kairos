package apitest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"openscum/internal/auth"
	"openscum/internal/realtime"
)

func dialWS(t *testing.T, env *Env, user *uuid.UUID, origin string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	h := http.Header{"Origin": {origin}}
	if user != nil {
		rec := httptest.NewRecorder()
		if err := env.App.Sessions.Issue(rec, *user, auth.MethodLocal); err != nil {
			t.Fatal(err)
		}
		for _, c := range rec.Result().Cookies() {
			h.Add("Cookie", c.Name+"="+c.Value)
		}
	}
	return websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(env.Srv.URL, "http")+"/ws", h)
}

func TestRealtimeRequiresSession(t *testing.T) {
	env := New(t)
	if _, res, err := dialWS(t, env, nil, Origin); err == nil || res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("dial without session: %v", err)
	}
	inactive := env.CreateUser(UserSpec{Email: "i@x.com", Status: "inactive"})
	if _, res, err := dialWS(t, env, &inactive, Origin); err == nil || res.StatusCode != http.StatusForbidden {
		t.Fatalf("dial as inactive user: %v", err)
	}
	active := env.CreateUser(UserSpec{Email: "a@x.com", Roles: []string{"member"}})
	if _, _, err := dialWS(t, env, &active, "http://evil.test"); err == nil {
		t.Fatal("dial from a foreign origin succeeded")
	}
}

func TestRealtimeClosesWhenUserIsDeactivated(t *testing.T) {
	env := New(t)
	env.App.Hub.Interval = 50 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go env.App.Hub.Run(ctx)

	admin := env.CreateUser(UserSpec{Email: "admin@x.com", Roles: []string{"admin"}})
	member := env.CreateUser(UserSpec{Email: "m@x.com", Roles: []string{"member"}})
	conn, _, err := dialWS(t, env, &member, Origin)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	env.As(admin).Post("/api/users/"+member.String()+"/deactivate", nil).Expect(t, http.StatusOK)

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, _, err = conn.ReadMessage()
	var ce *websocket.CloseError
	if !errors.As(err, &ce) || ce.Code != websocket.ClosePolicyViolation {
		t.Fatalf("read after deactivation: %v, want a policy-violation close", err)
	}
}

func TestRealtimeWindowWithinSpec(t *testing.T) {
	if realtime.CheckInterval+auth.PrincipalTTL > 120*time.Second {
		t.Fatal("RF-49 allows at most 120 s")
	}
}
