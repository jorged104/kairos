// Package apitest runs the whole API against a throwaway database. The tests
// in this package cover the HTTP behaviour of spec 003.
package apitest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"openscum/internal/auth"
	"openscum/internal/config"
	"openscum/internal/db"
	"openscum/internal/db/dbtest"
	"openscum/internal/db/sqlc"
	"openscum/internal/server"
)

const Origin = "http://kairos.test"

type Env struct {
	t     *testing.T
	Cfg   config.Config
	Store *db.Store
	App   *server.App
	Srv   *httptest.Server
}

// Option tweaks the configuration before the app is built.
type Option func(*config.Config)

func New(t *testing.T, opts ...Option) *Env {
	t.Helper()
	cfg := config.Config{
		Auth: config.AuthConfig{
			Mode:                config.AuthModeLocal,
			SessionSecret:       []byte("test-session-secret-0123456789abcdef"),
			PublicURL:           Origin,
			RegistrationEnabled: true,
			DefaultRole:         "member",
			// Cheap hashing keeps the tests fast; production uses the defaults.
			PasswordHashMemory: 1024,
			PasswordHashTime:   1,
		},
	}
	for _, o := range opts {
		o(&cfg)
	}
	store := db.NewStore(dbtest.New(t))
	app := server.NewApp(cfg, store, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	srv := httptest.NewServer(app.Handler)
	t.Cleanup(srv.Close)
	return &Env{t: t, Cfg: cfg, Store: store, App: app, Srv: srv}
}

// Client is an HTTP client with its own cookie jar that sends the frontend Origin.
type Client struct {
	t      *testing.T
	env    *Env
	http   *http.Client
	Origin string
}

func (e *Env) Client() *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		t: e.t, env: e, Origin: Origin,
		http: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
	}
}

// As returns a client with a session already open for the user.
func (e *Env) As(userID uuid.UUID) *Client {
	c := e.Client()
	rec := httptest.NewRecorder()
	if err := e.App.Sessions.Issue(rec, userID, auth.MethodLocal); err != nil {
		e.t.Fatal(err)
	}
	u, _ := url.Parse(e.Srv.URL)
	c.http.Jar.SetCookies(u, rec.Result().Cookies())
	return c
}

type Response struct {
	Status  int
	Header  http.Header
	Body    []byte
	Cookies []*http.Cookie
}

func (c *Client) Do(method, path string, body any) *Response {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, c.env.Srv.URL+path, r)
	if err != nil {
		c.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Origin != "" {
		req.Header.Set("Origin", c.Origin)
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return &Response{Status: res.StatusCode, Header: res.Header, Body: data, Cookies: res.Cookies()}
}

func (c *Client) Get(path string) *Response { return c.Do(http.MethodGet, path, nil) }
func (c *Client) Post(path string, body any) *Response {
	return c.Do(http.MethodPost, path, body)
}

// SetCookie puts a cookie in the jar, e.g. a forged session.
func (c *Client) SetCookie(name, value string) {
	u, _ := url.Parse(c.env.Srv.URL)
	c.http.Jar.SetCookies(u, []*http.Cookie{{Name: name, Value: value, Path: "/"}})
}

// Cookie returns a cookie from the jar.
func (c *Client) Cookie(name string) *http.Cookie {
	u, _ := url.Parse(c.env.Srv.URL)
	for _, ck := range c.http.Jar.Cookies(u) {
		if ck.Name == name {
			return ck
		}
	}
	return nil
}

func (r *Response) JSON(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		t.Fatalf("decode %s: %v", r.Body, err)
	}
}

// Code returns error.code of an error response.
func (r *Response) Code() string {
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(r.Body, &body)
	return body.Error.Code
}

// Expect fails the test unless the status (and, if given, the error code) match.
func (r *Response) Expect(t *testing.T, status int, code ...string) *Response {
	t.Helper()
	if r.Status != status {
		t.Fatalf("status = %d, want %d; body %s", r.Status, status, r.Body)
	}
	if len(code) > 0 && r.Code() != code[0] {
		t.Fatalf("code = %q, want %q; body %s", r.Code(), code[0], r.Body)
	}
	return r
}

// UserSpec seeds a user straight into the database.
type UserSpec struct {
	Name     string
	Email    string
	Status   string
	Roles    []string
	Teams    []uuid.UUID
	Password string
}

func (e *Env) CreateUser(s UserSpec) uuid.UUID {
	e.t.Helper()
	ctx := context.Background()
	if s.Status == "" {
		s.Status = "active"
	}
	if s.Name == "" {
		s.Name = s.Email
	}
	u, err := e.Store.CreateUser(ctx, sqlc.CreateUserParams{
		Name: s.Name, Email: s.Email, EmailNormalized: strings.ToLower(strings.TrimSpace(s.Email)), Status: s.Status,
	})
	if err != nil {
		e.t.Fatalf("seed user: %v", err)
	}
	for _, r := range s.Roles {
		if err := e.Store.AddManualRole(ctx, sqlc.AddManualRoleParams{UserID: u.ID, Role: r}); err != nil {
			e.t.Fatal(err)
		}
	}
	for _, team := range s.Teams {
		if err := e.Store.AddManualTeamMember(ctx, sqlc.AddManualTeamMemberParams{TeamID: team, UserID: u.ID}); err != nil {
			e.t.Fatal(err)
		}
	}
	if s.Password != "" {
		hash, err := auth.NewHasher(e.Cfg.Auth.PasswordHashMemory, e.Cfg.Auth.PasswordHashTime).Hash(s.Password)
		if err != nil {
			e.t.Fatal(err)
		}
		if err := e.Store.UpsertCredential(ctx, sqlc.UpsertCredentialParams{UserID: u.ID, PasswordHash: hash}); err != nil {
			e.t.Fatal(err)
		}
	}
	return u.ID
}

// CreateTeam seeds a team, optionally under a parent and led by some users.
func (e *Env) CreateTeam(name string, parent *uuid.UUID, leaders ...uuid.UUID) uuid.UUID {
	e.t.Helper()
	ctx := context.Background()
	team, err := e.Store.CreateTeam(ctx, sqlc.CreateTeamParams{Name: name, NameNormalized: strings.ToLower(name), ParentID: parent})
	if err != nil {
		e.t.Fatalf("seed team: %v", err)
	}
	for _, l := range leaders {
		if err := e.Store.AddTeamLeader(ctx, sqlc.AddTeamLeaderParams{TeamID: team.ID, UserID: l}); err != nil {
			e.t.Fatal(err)
		}
	}
	return team.ID
}

// AuditActions lists the actions recorded for a target, oldest first.
func (e *Env) AuditActions(targetID string) []string {
	e.t.Helper()
	rows, err := e.Store.Pool.Query(context.Background(),
		"SELECT action FROM audit_log WHERE target_id = $1 ORDER BY id", targetID)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var a string
		_ = rows.Scan(&a)
		out = append(out, a)
	}
	return out
}
