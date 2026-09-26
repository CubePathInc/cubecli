package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// randomString returns n random bytes, base64url encoded without padding.
func randomString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// newPKCE returns a code verifier (43 chars, RFC 7636 minimum entropy) and its
// S256 challenge.
func newPKCE() (verifier, challenge string) {
	verifier = randomString(32)
	return verifier, s256(verifier)
}

func s256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
