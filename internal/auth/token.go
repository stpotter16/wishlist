package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

const accessTokenBytes = 32

// NewAccessToken returns a random token for a giver's private link, along
// with the hash to store. Only the hash is ever persisted.
func NewAccessToken() (token string, tokenHash string, err error) {
	b := make([]byte, accessTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashAccessToken(token), nil
}

// HashAccessToken returns the hex-encoded SHA-256 of a token. A fast hash is
// safe here because tokens are random, unlike passwords.
func HashAccessToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
