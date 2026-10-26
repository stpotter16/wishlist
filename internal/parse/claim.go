package parse

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"

	"github.com/stpotter16/wishlist/internal/types"
)

const maxClaimPostBytes = 4 << 10

const maxGiverFieldLength = 200

// ParseClaimPost decodes a giver's contact information and Turnstile token
// from a JSON request body. Name, email, and phone are all required. The
// token is returned as-is, since only Cloudflare can tell whether it's valid.
func ParseClaimPost(w http.ResponseWriter, r *http.Request) (types.Giver, string, error) {
	body := struct {
		Name           string `json:"name"`
		Email          string `json:"email"`
		Phone          string `json:"phone"`
		TurnstileToken string `json:"turnstileToken"`
	}{}

	r.Body = http.MaxBytesReader(w, r.Body, maxClaimPostBytes)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return types.Giver{}, "", errors.New("request body is not valid JSON")
	}

	giver := types.Giver{
		Name:  strings.TrimSpace(body.Name),
		Email: strings.TrimSpace(body.Email),
		Phone: strings.TrimSpace(body.Phone),
	}

	if giver.Name == "" || giver.Email == "" || giver.Phone == "" {
		return types.Giver{}, "", errors.New("name, email, and phone are all required")
	}
	if len(giver.Name) > maxGiverFieldLength ||
		len(giver.Email) > maxGiverFieldLength ||
		len(giver.Phone) > maxGiverFieldLength {
		return types.Giver{}, "", errors.New("name, email, and phone must each be under 200 characters")
	}

	// ParseAddress also accepts forms like "Name <a@b.com>", so require the
	// input to be a bare address.
	addr, err := mail.ParseAddress(giver.Email)
	if err != nil || addr.Address != giver.Email {
		return types.Giver{}, "", errors.New("email address is not valid")
	}

	return giver, body.TurnstileToken, nil
}
