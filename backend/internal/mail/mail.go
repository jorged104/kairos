// Package mail sends Kairos' messages through the organization's SMTP server
// (spec 019): plain text and HTML, over no security, STARTTLS or TLS, with
// the standard library only. A failure says at which step it happened (RF-7).
package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Security of the connection.
const (
	SecurityNone     = "none"
	SecurityStartTLS = "starttls"
	SecurityTLS      = "tls"
)

// Steps where a send fails (RF-7).
const (
	StepConnect = "connect"
	StepTLS     = "tls"
	StepAuth    = "auth"
	StepSend    = "send"
)

// Timeout bounds a whole send.
const Timeout = 15 * time.Second

// Config is the outgoing mail of the system settings.
type Config struct {
	Enabled   bool
	Host      string
	Port      int
	Security  string
	Username  string
	Password  string
	FromEmail string
	FromName  string
}

// Ready: switched on and with a server and a sender (spec 019, «correo configurado»).
func (c Config) Ready() bool {
	return c.Enabled && c.Host != "" && c.FromEmail != ""
}

// Message is one mail to one address.
type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

// Error is a failed send and the step where it failed.
type Error struct {
	Step string
	Err  error
}

func (e *Error) Error() string { return "correo (" + e.Step + "): " + e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// ErrNotConfigured: the mail is off or incomplete.
var ErrNotConfigured = errors.New("el correo no está configurado")

// Sender sends messages; tests replace it.
type Sender interface {
	Send(ctx context.Context, cfg Config, m Message) error
}

// SMTP sends with the standard library.
type SMTP struct{}

func fail(step string, err error) error { return &Error{Step: step, Err: err} }

func (SMTP) Send(ctx context.Context, cfg Config, m Message) error {
	if cfg.Host == "" || cfg.FromEmail == "" {
		return ErrNotConfigured
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fail(StepConnect, err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	tlsConfig := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}
	if cfg.Security == SecurityTLS {
		tc := tls.Client(conn, tlsConfig)
		if err := tc.HandshakeContext(ctx); err != nil {
			_ = conn.Close()
			return fail(StepTLS, err)
		}
		conn = tc
	}
	c, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		_ = conn.Close()
		return fail(StepConnect, err)
	}
	defer c.Close()
	if cfg.Security == SecurityStartTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return fail(StepTLS, errors.New("el servidor no ofrece STARTTLS"))
		}
		if err := c.StartTLS(tlsConfig); err != nil {
			return fail(StepTLS, err)
		}
	}
	if cfg.Username != "" {
		if err := c.Auth(auth{username: cfg.Username, password: cfg.Password, host: cfg.Host}); err != nil {
			return fail(StepAuth, err)
		}
	}
	if err := c.Mail(cfg.FromEmail); err != nil {
		return fail(StepSend, err)
	}
	if err := c.Rcpt(m.To); err != nil {
		return fail(StepSend, err)
	}
	w, err := c.Data()
	if err != nil {
		return fail(StepSend, err)
	}
	if _, err := w.Write(Build(cfg, m, time.Now())); err != nil {
		return fail(StepSend, err)
	}
	if err := w.Close(); err != nil {
		return fail(StepSend, err)
	}
	if err := c.Quit(); err != nil {
		return fail(StepSend, err)
	}
	return nil
}

// auth is PLAIN, also over a connection with no TLS: the administrator
// chose that security; smtp.PlainAuth would refuse it.
type auth struct{ username, password, host string }

func (a auth) Start(_ *smtp.ServerInfo) (string, []byte, error) {
	return "PLAIN", []byte("\x00" + a.username + "\x00" + a.password), nil
}

func (a auth) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		return nil, errors.New("respuesta inesperada del servidor")
	}
	return nil, nil
}

// Build writes the message: headers and a multipart/alternative body (RF-40).
func Build(cfg Config, m Message, now time.Time) []byte {
	var b bytes.Buffer
	from := (&mail.Address{Name: cfg.FromName, Address: cfg.FromEmail}).String()
	boundary := randomHex(12)
	header := func(k, v string) { fmt.Fprintf(&b, "%s: %s\r\n", k, v) }
	header("From", from)
	header("To", (&mail.Address{Address: m.To}).String())
	header("Subject", mime.QEncoding.Encode("utf-8", m.Subject))
	header("Date", now.Format(time.RFC1123Z))
	header("Message-ID", "<"+randomHex(16)+"@"+domain(cfg.FromEmail)+">")
	header("MIME-Version", "1.0")
	header("Content-Type", `multipart/alternative; boundary="`+boundary+`"`)
	b.WriteString("\r\n")
	part := func(contentType, body string) {
		fmt.Fprintf(&b, "--%s\r\nContent-Type: %s; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n", boundary, contentType)
		qp := quotedprintable.NewWriter(&b)
		_, _ = qp.Write([]byte(strings.ReplaceAll(body, "\n", "\r\n")))
		_ = qp.Close()
		b.WriteString("\r\n")
	}
	part("text/plain", m.Text)
	part("text/html", m.HTML)
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return b.Bytes()
}

func randomHex(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func domain(address string) string {
	if i := strings.LastIndexByte(address, '@'); i >= 0 {
		return address[i+1:]
	}
	return "kairos"
}

// ValidAddress: an address alone, as the sender must be (RF-5).
func ValidAddress(s string) bool {
	a, err := mail.ParseAddress(s)
	return err == nil && a.Address == s
}

// Secret is a random token for a link, 32 bytes in base64url (RF-37).
func Secret() string {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	return base64.RawURLEncoding.EncodeToString(buf)
}
