package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	h := NewHasher(1024, 1)
	hash, err := h.Hash("una-contrasena-larga")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=1024,t=1,p=1$") {
		t.Fatalf("hash = %q, want PHC argon2id", hash)
	}
	if strings.Contains(hash, "una-contrasena-larga") {
		t.Fatal("hash contains the password")
	}
	if !h.Verify("una-contrasena-larga", hash) {
		t.Fatal("Verify rejected the right password")
	}
	if h.Verify("otra-contrasena-larga", hash) {
		t.Fatal("Verify accepted a wrong password")
	}
	if h.Verify("una-contrasena-larga", "") || h.Verify("x", "$bcrypt$broken") {
		t.Fatal("Verify accepted an empty or malformed hash")
	}
	other, _ := h.Hash("una-contrasena-larga")
	if other == hash {
		t.Fatal("two hashes of the same password are equal: missing salt")
	}
}

func TestVerifyUsesTheCostStoredInTheHash(t *testing.T) {
	hash, _ := NewHasher(2048, 2).Hash("una-contrasena-larga")
	if !NewHasher(1024, 1).Verify("una-contrasena-larga", hash) {
		t.Fatal("a cost change made old hashes unverifiable")
	}
}

func TestPasswordLength(t *testing.T) {
	cases := map[string]error{
		strings.Repeat("a", 11):  errPasswordTooShort,
		strings.Repeat("a", 12):  nil,
		strings.Repeat("ñ", 12):  nil, // counted in characters, not bytes
		strings.Repeat("a", 128): nil,
		strings.Repeat("a", 129): errPasswordTooLong,
	}
	for pw, want := range cases {
		if got := CheckPasswordLength(pw); !errors.Is(got, want) {
			t.Errorf("len %d: got %v, want %v", len([]rune(pw)), got, want)
		}
	}
}
