package apitest

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"openscum/internal/config"
)

type deliveryDTO struct {
	Sent      bool    `json:"sent"`
	URL       string  `json:"url"`
	ExpiresAt *string `json:"expires_at"`
	Kind      string  `json:"kind"`
}

type createdUserDTO struct {
	ID          uuid.UUID    `json:"id"`
	HasPassword *bool        `json:"has_password"`
	Delivery    *deliveryDTO `json:"delivery"`
	Invitation  *struct {
		ExpiresAt string `json:"expires_at"`
		Expired   bool   `json:"expired"`
	} `json:"invitation"`
	Can *struct {
		Invite       bool `json:"invite"`
		PasswordLink bool `json:"password_link"`
	} `json:"can"`
}

// useMail points the system's mail to a fake server (spec 019).
func useMail(t *testing.T, env *Env, admin *Client) *SMTP {
	t.Helper()
	smtp := NewSMTP(t)
	admin.Do(http.MethodPut, "/api/system/settings/mail", map[string]any{
		"enabled": true, "host": "127.0.0.1", "port": smtp.Port(), "security": "none", "from_email": "kairos@x.com", "from_name": "Kairos",
	}).Expect(t, http.StatusOK)
	env.App.Accounts.Wait = 5 * time.Second // the fake server answers at once
	return smtp
}

func setPassword(c *Client, link, password string) *Response {
	return c.Post("/api/auth/password/reset", map[string]any{"token": secretOf(link), "password": password})
}

func TestMailSettings(t *testing.T) {
	env := New(t)
	admin := env.Admin()
	member := env.As(env.CreateUser(UserSpec{Email: "m@x.com", Roles: []string{"member"}}))
	smtp := NewSMTP(t)
	smtp.Username, smtp.Password = "kairos", "secreto-smtp"
	body := map[string]any{"enabled": true, "host": "127.0.0.1", "port": smtp.Port(), "security": "none",
		"username": "kairos", "password": "secreto-smtp", "from_email": "kairos@x.com", "from_name": "Kairos"}

	// Only the system administrator (RF-1); a valid sender and a server (RF-5).
	member.Do(http.MethodPut, "/api/system/settings/mail", body).Expect(t, http.StatusForbidden)
	bad := map[string]any{}
	for k, v := range body {
		bad[k] = v
	}
	bad["from_email"] = "no es un correo"
	admin.Do(http.MethodPut, "/api/system/settings/mail", bad).Expect(t, http.StatusBadRequest)
	res := admin.Do(http.MethodPut, "/api/system/settings/mail", body).Expect(t, http.StatusOK)
	if strings.Contains(string(res.Body), "secreto-smtp") || !strings.Contains(string(res.Body), `"password_set":true`) { // RF-4
		t.Fatalf("view = %s", res.Body)
	}

	// The test mail goes to the administrator (RF-6).
	admin.Post("/api/system/settings/mail/test", nil).Expect(t, http.StatusNoContent)
	if got := smtp.Wait(1); !strings.HasPrefix(got[0].To, "admin-") || !strings.Contains(got[0].Data, "correo de prueba") {
		t.Fatalf("test mail = %+v", got)
	}
	// Saved without the password, it keeps it; a wrong one fails at auth (RF-7).
	body["password"] = ""
	admin.Do(http.MethodPut, "/api/system/settings/mail", body).Expect(t, http.StatusOK)
	admin.Post("/api/system/settings/mail/test", nil).Expect(t, http.StatusNoContent)
	body["password"] = "otra"
	admin.Do(http.MethodPut, "/api/system/settings/mail", body).Expect(t, http.StatusOK)
	if r := admin.Post("/api/system/settings/mail/test", nil).Expect(t, http.StatusBadRequest, "mail_test_failed"); !strings.Contains(string(r.Body), `"step":"auth"`) {
		t.Fatalf("auth = %s", r.Body)
	}
	body["port"] = 1
	admin.Do(http.MethodPut, "/api/system/settings/mail", body).Expect(t, http.StatusOK)
	if r := admin.Post("/api/system/settings/mail/test", nil).Expect(t, http.StatusBadRequest, "mail_test_failed"); !strings.Contains(string(r.Body), `"step":"connect"`) {
		t.Fatalf("connect = %s", r.Body)
	}
	var raw string
	if err := env.Store.Pool.QueryRow(context.Background(),
		"SELECT string_agg(after::text, ' ') FROM audit_log WHERE action = 'system.settings_changed' AND target_id = 'mail'").Scan(&raw); err != nil ||
		strings.Contains(raw, "secreto-smtp") || !strings.Contains(raw, "changed:") { // RF-8
		t.Fatalf("audit = %s %v", raw, err)
	}
}

func TestInvitations(t *testing.T) {
	env := New(t)
	admin := env.Admin()

	// Without mail the person is created anyway and the link comes back (RF-10, RF-13).
	var u createdUserDTO
	admin.Post("/api/users", map[string]any{"name": "Ana", "email": "ana@x.com", "password": "ignorada-del-todo"}).Expect(t, http.StatusCreated).JSON(t, &u)
	if *u.HasPassword || u.Delivery == nil || u.Delivery.Sent || !strings.Contains(u.Delivery.URL, Origin+"/password#t=") || u.Invitation == nil || !u.Can.Invite {
		t.Fatalf("created = %+v %+v", u, u.Delivery)
	}
	first := u.Delivery.URL

	// With mail it goes out, and the link is not handed over (RF-12).
	smtp := useMail(t, env, admin)
	var d deliveryDTO
	admin.Post("/api/users/"+u.ID.String()+"/invitation", nil).Expect(t, http.StatusOK).JSON(t, &d)
	mail := smtp.Wait(1)[0]
	if !d.Sent || d.URL != "" || mail.To != "ana@x.com" || mail.Link() == "" || strings.Contains(mail.Data, "ignorada") {
		t.Fatalf("resend = %+v, mail = %+v", d, mail)
	}
	// The new link voids the first one (RF-20, RF-18).
	setPassword(env.Client(), first, "una-contrasena-larga").Expect(t, http.StatusBadRequest, "invalid_link")
	var check struct {
		Purpose string `json:"purpose"`
		Email   string `json:"email"`
	}
	env.Client().Post("/api/auth/password/check", map[string]any{"token": secretOf(mail.Link())}).Expect(t, http.StatusOK).JSON(t, &check)
	if check.Purpose != "invite" || check.Email != "ana@x.com" {
		t.Fatalf("check = %+v", check)
	}
	// The password follows the rules (RF-17), and the link is single use (RF-16).
	setPassword(env.Client(), mail.Link(), "corta").Expect(t, http.StatusBadRequest)
	setPassword(env.Client(), mail.Link(), "una-contrasena-larga").Expect(t, http.StatusNoContent)
	login(env.Client(), "ana@x.com", "una-contrasena-larga").Expect(t, http.StatusOK)
	setPassword(env.Client(), mail.Link(), "otra-contrasena-larga").Expect(t, http.StatusBadRequest, "invalid_link")
	var after createdUserDTO
	admin.Get("/api/users/"+u.ID.String()).Expect(t, http.StatusOK).JSON(t, &after)
	if after.Invitation != nil || after.Can.Invite {
		t.Fatalf("still pending = %+v", after)
	}
	admin.Post("/api/users/"+u.ID.String()+"/invitation", nil).Expect(t, http.StatusConflict, "user_has_password")
	if a := strings.Join(env.AuditActions(u.ID.String()), " "); strings.Count(a, "user.invitation_sent") != 2 || !strings.HasSuffix(a, "user.password_set_by_link") { // RF-35, RF-36
		t.Fatalf("audit = %v", a)
	}

	// Deactivated, its links are void (RF-42).
	var b createdUserDTO
	admin.Post("/api/users", map[string]any{"name": "Bea", "email": "bea@x.com"}).Expect(t, http.StatusCreated).JSON(t, &b)
	link := smtp.Wait(2)[1].Link()
	admin.Post("/api/users/"+b.ID.String()+"/deactivate", nil).Expect(t, http.StatusOK)
	setPassword(env.Client(), link, "una-contrasena-larga").Expect(t, http.StatusBadRequest, "invalid_link")
}

func TestInvitationWithSSOOnly(t *testing.T) {
	env := New(t, WithMode(config.AuthModeOIDC))
	admin := env.Admin()
	var u createdUserDTO
	admin.Post("/api/users", map[string]any{"name": "Ana", "email": "ana@x.com"}).Expect(t, http.StatusCreated).JSON(t, &u)
	if u.Delivery == nil || u.Delivery.Kind != "login" || u.Delivery.URL != Origin+"/login" || u.Invitation != nil { // RF-15
		t.Fatalf("created = %+v %+v", u, u.Delivery)
	}
}

func TestPasswordRecovery(t *testing.T) {
	env := New(t, func(c *config.Config) { c.Auth.BootstrapAdminEmail = "root@x.com" })
	admin := env.Admin()
	smtp := useMail(t, env, admin)
	ana := env.CreateUser(UserSpec{Email: "ana@x.com", Roles: []string{"member"}, Password: pw})
	forgot := func(email string) {
		env.Client().Post("/api/auth/password/forgot", map[string]any{"email": email}).Expect(t, http.StatusAccepted) // RF-25
	}

	// Unknown, and known: the same answer; only the known gets a mail (RF-26).
	forgot("nadie@x.com")
	forgot("ANA@x.com")
	first := smtp.Wait(1)[0]
	if first.To != "ana@x.com" || first.Link() == "" {
		t.Fatalf("mail = %+v", first)
	}
	// Only the last link counts (RF-28), at most 3 an hour (RF-27).
	forgot("ana@x.com")
	forgot("ana@x.com")
	forgot("ana@x.com")
	time.Sleep(300 * time.Millisecond)
	mails := smtp.Wait(3)
	if len(smtp.Messages()) != 3 {
		t.Fatalf("mails = %d", len(smtp.Messages()))
	}
	setPassword(env.Client(), first.Link(), "nueva-contrasena-1").Expect(t, http.StatusBadRequest, "invalid_link")
	last := mails[2].Link()

	// Used once, with a notice after (RF-29, RF-31).
	setPassword(env.Client(), last, "nueva-contrasena-1").Expect(t, http.StatusNoContent)
	login(env.Client(), "ana@x.com", pw).Expect(t, http.StatusUnauthorized)
	login(env.Client(), "ana@x.com", "nueva-contrasena-1").Expect(t, http.StatusOK)
	if notice := smtp.Wait(4)[3]; notice.To != "ana@x.com" || !strings.Contains(notice.Data, "se acaba de cambiar") || notice.Link() != "" {
		t.Fatalf("notice = %+v", notice)
	}
	setPassword(env.Client(), last, "otra-contrasena-1").Expect(t, http.StatusBadRequest, "invalid_link")

	// An expired link is not valid (RF-30).
	env.Store.Pool.Exec(context.Background(), "DELETE FROM user_tokens")
	forgot("ana@x.com")
	link := smtp.Wait(5)[4].Link()
	env.Store.Pool.Exec(context.Background(), "UPDATE user_tokens SET expires_at = now() - interval '1 minute'")
	env.Client().Post("/api/auth/password/check", map[string]any{"token": secretOf(link)}).Expect(t, http.StatusBadRequest, "invalid_link")

	// The administrator sends a link instead of setting it by hand (RF-32 to RF-34).
	lead := env.As(env.CreateUser(UserSpec{Email: "lead@x.com", Roles: []string{"lead"}}))
	lead.Post("/api/users/"+ana.String()+"/password-link", nil).Expect(t, http.StatusForbidden)
	var d deliveryDTO
	admin.Post("/api/users/"+ana.String()+"/password-link", nil).Expect(t, http.StatusOK).JSON(t, &d)
	if !d.Sent || smtp.Wait(6)[5].Link() == "" {
		t.Fatalf("admin link = %+v", d)
	}
	admin.Do(http.MethodPut, "/api/users/"+ana.String()+"/password", map[string]any{"password": "x-x-x-x-x-x-x"}).Expect(t, http.StatusNotFound)
	if a := env.AuditActions(ana.String()); !strings.Contains(strings.Join(a, " "), "user.password_link_sent") {
		t.Fatalf("audit = %v", a)
	}

	// A deactivated person gets nothing.
	admin.Post("/api/users/"+ana.String()+"/deactivate", nil).Expect(t, http.StatusOK)
	forgot("ana@x.com")
	time.Sleep(300 * time.Millisecond)
	if n := len(smtp.Messages()); n != 6 {
		t.Fatalf("mails = %d", n)
	}
}

func TestPasswordRecoveryWithSSOOnly(t *testing.T) {
	env := New(t, WithMode(config.AuthModeOIDC), func(c *config.Config) { c.Auth.BootstrapAdminEmail = "root@x.com" })
	admin := env.Admin()
	smtp := useMail(t, env, admin)
	env.CreateUser(UserSpec{Email: "ana@x.com", Roles: []string{"member"}, Password: pw})
	env.CreateUser(UserSpec{Email: "root@x.com", Roles: []string{"admin"}, Password: pw})

	// Only the initial administrator, for the emergency way in (RF-24).
	env.Client().Post("/api/auth/password/forgot", map[string]any{"email": "ana@x.com"}).Expect(t, http.StatusAccepted)
	env.Client().Post("/api/auth/password/forgot", map[string]any{"email": "root@x.com"}).Expect(t, http.StatusAccepted)
	time.Sleep(300 * time.Millisecond)
	if got := smtp.Wait(1); len(smtp.Messages()) != 1 || got[0].To != "root@x.com" {
		t.Fatalf("mails = %+v", smtp.Messages())
	}
	setPassword(env.Client(), smtp.Messages()[0].Link(), "nueva-contrasena-1").Expect(t, http.StatusNoContent)
}
