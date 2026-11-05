package apitest

import (
	"bufio"
	"encoding/base64"
	"io"
	"mime/quotedprintable"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// SMTP is a fake mail server for the tests of spec 019: it speaks just
// enough SMTP, keeps every message and may ask for a password.
type SMTP struct {
	t        *testing.T
	ln       net.Listener
	Username string
	Password string

	mu       sync.Mutex
	messages []Mail
}

// Mail is a message the fake server received, its body decoded.
type Mail struct {
	To   string
	Data string
}

func NewSMTP(t *testing.T) *SMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &SMTP{t: t, ln: ln}
	t.Cleanup(func() { _ = ln.Close() })
	go s.serve()
	return s
}

// Port is where it listens, on 127.0.0.1.
func (s *SMTP) Port() int { return s.ln.Addr().(*net.TCPAddr).Port }

func (s *SMTP) serve() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.session(c)
	}
}

func (s *SMTP) session(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	say := func(line string) { _, _ = io.WriteString(c, line+"\r\n") }
	say("220 fake ESMTP")
	var to string
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		cmd := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			say("250-fake")
			say("250 AUTH PLAIN")
		case strings.HasPrefix(cmd, "AUTH PLAIN"):
			raw, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(line[len("AUTH PLAIN"):]))
			parts := strings.Split(string(raw), "\x00")
			if len(parts) == 3 && parts[1] == s.Username && parts[2] == s.Password {
				say("235 ok")
			} else {
				say("535 bad credentials")
			}
		case strings.HasPrefix(cmd, "MAIL FROM"):
			say("250 ok")
		case strings.HasPrefix(cmd, "RCPT TO"):
			to = strings.Trim(strings.TrimSpace(line[len("RCPT TO:"):]), "<>")
			say("250 ok")
		case cmd == "DATA":
			say("354 go on")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				b.WriteString(l)
			}
			body, _ := io.ReadAll(quotedprintable.NewReader(strings.NewReader(b.String())))
			s.mu.Lock()
			s.messages = append(s.messages, Mail{To: to, Data: string(body)})
			s.mu.Unlock()
			say("250 queued")
		case cmd == "QUIT":
			say("221 bye")
			return
		default:
			say("250 ok")
		}
	}
}

// Messages are the ones received so far.
func (s *SMTP) Messages() []Mail {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Mail(nil), s.messages...)
}

// Wait waits up to 5 s for n messages and returns them.
func (s *SMTP) Wait(n int) []Mail {
	s.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if m := s.Messages(); len(m) >= n {
			return m
		}
		time.Sleep(20 * time.Millisecond)
	}
	s.t.Fatalf("got %d mails, want %d", len(s.Messages()), n)
	return nil
}

// Link is the link of a password page inside a mail.
func (m Mail) Link() string {
	i := strings.Index(m.Data, "/password#t=")
	if i < 0 {
		return ""
	}
	end := strings.IndexAny(m.Data[i:], "\r\n <\"")
	if end < 0 {
		return m.Data[i:]
	}
	return m.Data[i : i+end]
}

// secretOf is the secret of a link, after #t=.
func secretOf(link string) string {
	_, s, _ := strings.Cut(link, "#t=")
	return s
}
