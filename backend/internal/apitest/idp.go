package apitest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"openscum/internal/config"
)

const (
	idpClientID     = "kairos"
	idpClientSecret = "secret"
)

// IdP is a minimal OIDC provider: discovery, JWKS, token endpoint and an
// optional end_session_endpoint. Tests "log in" with Authorize.
type IdP struct {
	t   *testing.T
	Srv *httptest.Server
	key *rsa.PrivateKey

	mu    sync.Mutex
	codes map[string]pendingCode

	// Knobs for the failure cases.
	TokenDelay    time.Duration
	SignWith      *rsa.PrivateKey // a key not published in the JWKS
	Audience      string
	OverrideNonce string
	NoEndSession  bool
	TokenCalls    int
}

type pendingCode struct {
	claims    map[string]any
	nonce     string
	challenge string
}

func NewIdP(t *testing.T) *IdP {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	idp := &IdP{t: t, key: key, codes: map[string]pendingCode{}, Audience: idpClientID}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", idp.discovery)
	mux.HandleFunc("/jwks", idp.jwks)
	mux.HandleFunc("/token", idp.token)
	idp.Srv = httptest.NewServer(mux)
	t.Cleanup(idp.Srv.Close)
	return idp
}

// WithIdP configures the app for this IdP in "both" mode.
func WithIdP(idp *IdP) Option {
	return func(c *config.Config) {
		c.Auth.Mode = config.AuthModeBoth
		c.OIDC = config.OIDCConfig{
			Issuer: idp.Srv.URL, DiscoveryURL: idp.Srv.URL, ClientID: idpClientID, ClientSecret: idpClientSecret,
			RedirectURL: Origin + "/api/auth/oidc/callback", RoleSource: config.RoleSourceIdP, LinkByEmail: true,
		}
	}
}

func (idp *IdP) discovery(w http.ResponseWriter, _ *http.Request) {
	doc := map[string]any{
		"issuer":                                idp.Srv.URL,
		"authorization_endpoint":                idp.Srv.URL + "/authorize",
		"token_endpoint":                        idp.Srv.URL + "/token",
		"jwks_uri":                              idp.Srv.URL + "/jwks",
		"id_token_signing_alg_values_supported": []string{"RS256"},
	}
	if !idp.NoEndSession {
		doc["end_session_endpoint"] = idp.Srv.URL + "/logout"
	}
	_ = json.NewEncoder(w).Encode(doc)
}

func (idp *IdP) jwks(w http.ResponseWriter, _ *http.Request) {
	pub := idp.key.PublicKey
	_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]any{{
		"kty": "RSA", "alg": "RS256", "use": "sig", "kid": "k1",
		"n": base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}}})
}

func (idp *IdP) token(w http.ResponseWriter, r *http.Request) {
	idp.mu.Lock()
	idp.TokenCalls++
	delay := idp.TokenDelay
	idp.mu.Unlock()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
			return
		}
	}
	_ = r.ParseForm()
	idp.mu.Lock()
	pc, ok := idp.codes[r.PostForm.Get("code")]
	delete(idp.codes, r.PostForm.Get("code"))
	idp.mu.Unlock()

	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	if !ok || base64.RawURLEncoding.EncodeToString(sum[:]) != pc.challenge {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
		return
	}

	nonce := pc.nonce
	if idp.OverrideNonce != "" {
		nonce = idp.OverrideNonce
	}
	claims := jwt.MapClaims{
		"iss": idp.Srv.URL, "aud": idp.Audience, "iat": time.Now().Unix(),
		"exp": time.Now().Add(5 * time.Minute).Unix(), "nonce": nonce,
	}
	for k, v := range pc.claims {
		claims[k] = v
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = "k1"
	key := idp.key
	if idp.SignWith != nil {
		key = idp.SignWith
	}
	signed, err := tok.SignedString(key)
	if err != nil {
		idp.t.Error(err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": "at", "token_type": "Bearer", "expires_in": 300, "id_token": signed,
	})
}

// Claims for a user at the IdP.
func Claims(sub, email string, extra map[string]any) map[string]any {
	c := map[string]any{"sub": sub, "email": email, "email_verified": true, "name": "Usuario " + sub}
	for k, v := range extra {
		c[k] = v
	}
	return c
}

// Login runs the whole SSO flow for the client and returns the callback response.
func (idp *IdP) Login(c *Client, claims map[string]any) *Response {
	idp.t.Helper()
	start := c.Get("/api/auth/oidc/start?return_to=/users")
	if start.Status != http.StatusFound {
		idp.t.Fatalf("start = %d %s", start.Status, start.Body)
	}
	return c.Get(idp.Authorize(start.Header.Get("Location"), claims))
}

// Authorize plays the user logging in at the IdP: it returns the callback
// path with a code that the token endpoint will exchange for these claims.
func (idp *IdP) Authorize(location string, claims map[string]any) string {
	idp.t.Helper()
	u, err := url.Parse(location)
	if err != nil {
		idp.t.Fatal(err)
	}
	q := u.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("state") == "" || q.Get("nonce") == "" {
		idp.t.Fatalf("authorization request without PKCE/state/nonce: %s", location)
	}
	code := randomCode()
	idp.mu.Lock()
	idp.codes[code] = pendingCode{claims: claims, nonce: q.Get("nonce"), challenge: q.Get("code_challenge")}
	idp.mu.Unlock()
	return "/api/auth/oidc/callback?" + url.Values{"code": {code}, "state": {q.Get("state")}}.Encode()
}

func randomCode() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
