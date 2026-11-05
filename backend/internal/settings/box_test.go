package settings

import (
	"bytes"
	"testing"
)

func TestBoxRoundTrip(t *testing.T) {
	b := newBox([]byte("test-session-secret-0123456789abcdef"))
	sealed, err := b.seal("s3cr3t")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("s3cr3t")) {
		t.Fatal("the secret is stored in clear (RF-5)")
	}
	if got, err := b.open(sealed); err != nil || got != "s3cr3t" {
		t.Fatalf("open = %q, %v", got, err)
	}
	again, _ := b.seal("s3cr3t")
	if bytes.Equal(sealed, again) {
		t.Fatal("two seals of the same secret are equal: the nonce is not random")
	}
	if sealed, _ := b.seal(""); sealed != nil {
		t.Fatal("an empty secret must be stored as none")
	}
	other := newBox([]byte("another-session-secret-0123456789ab"))
	if _, err := other.open(sealed); err == nil {
		t.Fatal("another key opened the secret")
	}
}
