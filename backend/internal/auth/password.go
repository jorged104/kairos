package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	MinPasswordLength = 6
	MaxPasswordLength = 128

	saltBytes = 16
	keyBytes  = 32
)

var errMalformedHash = errors.New("hash de contraseña con formato no válido")

// Hasher hashes passwords with argon2id and a random salt; the cost comes
// from the configuration and is stored with each hash.
type Hasher struct {
	memory uint32
	time   uint32
	// dummy is compared when the email does not exist, so both cases take as long (RF-8).
	dummy string
}

func NewHasher(memoryKiB, time uint32) *Hasher {
	h := &Hasher{memory: memoryKiB, time: time}
	h.dummy, _ = h.Hash("kairos-dummy-password")
	return h
}

// Hash returns the PHC string $argon2id$v=19$m=..,t=..,p=1$salt$key.
func (h *Hasher) Hash(password string) (string, error) {
	salt := make([]byte, saltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generar sal: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, h.time, h.memory, 1, keyBytes)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=1$%s$%s", argon2.Version, h.memory, h.time,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// Verify compares in constant time; an empty hash checks against a dummy one.
func (h *Hasher) Verify(password, encoded string) bool {
	if encoded == "" {
		_, _ = verify(password, h.dummy)
		return false
	}
	ok, err := verify(password, encoded)
	return err == nil && ok
}

func verify(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errMalformedHash
	}
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, errMalformedHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, errMalformedHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, errMalformedHash
	}
	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// CheckPasswordLength enforces 6 to 128 characters (RF-14).
func CheckPasswordLength(password string) error {
	n := utf8.RuneCountInString(password)
	if n < MinPasswordLength {
		return errPasswordTooShort
	}
	if n > MaxPasswordLength {
		return errPasswordTooLong
	}
	return nil
}
