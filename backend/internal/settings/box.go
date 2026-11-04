package settings

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
)

// box encrypts the secrets of the settings with AES-256-GCM (RF-5). The key
// derives from the session secret; if that changes, the stored secrets can no
// longer be read and have to be written again.
type box struct {
	aead cipher.AEAD
}

func newBox(sessionSecret []byte) *box {
	key := sha256.Sum256(append([]byte("kairos/settings/v1\x00"), sessionSecret...))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		panic(err) // a 32-byte key never fails
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}
	return &box{aead: aead}
}

// seal returns nil for an empty secret: «not configured».
func (b *box) seal(plain string) ([]byte, error) {
	if plain == "" {
		return nil, nil
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generar nonce: %w", err)
	}
	return b.aead.Seal(nonce, nonce, []byte(plain), nil), nil
}

var errUnreadable = errors.New("secreto ilegible")

func (b *box) open(sealed []byte) (string, error) {
	if len(sealed) == 0 {
		return "", nil
	}
	n := b.aead.NonceSize()
	if len(sealed) < n {
		return "", errUnreadable
	}
	plain, err := b.aead.Open(nil, sealed[:n], sealed[n:], nil)
	if err != nil {
		return "", errUnreadable
	}
	return string(plain), nil
}
