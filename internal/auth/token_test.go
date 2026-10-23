package auth_test

import (
	"testing"

	"github.com/stpotter16/wishlist/internal/auth"
)

func TestNewAccessTokenHashMatchesToken(t *testing.T) {
	token, tokenHash, err := auth.NewAccessToken()
	if err != nil {
		t.Fatalf("NewAccessToken: %v", err)
	}

	if got, want := tokenHash, auth.HashAccessToken(token); got != want {
		t.Errorf("tokenHash=%q, want=%q", got, want)
	}
}

func TestNewAccessTokenIsUnique(t *testing.T) {
	first, _, err := auth.NewAccessToken()
	if err != nil {
		t.Fatalf("NewAccessToken: %v", err)
	}
	second, _, err := auth.NewAccessToken()
	if err != nil {
		t.Fatalf("NewAccessToken: %v", err)
	}

	if first == second {
		t.Errorf("two tokens are both %q", first)
	}
}

func TestHashAccessToken(t *testing.T) {
	// SHA-256 of "abc" from FIPS 180-2.
	if got, want := auth.HashAccessToken("abc"), "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"; got != want {
		t.Errorf("HashAccessToken=%q, want=%q", got, want)
	}
}
