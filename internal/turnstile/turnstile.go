// Package turnstile verifies Cloudflare Turnstile tokens server side.
package turnstile

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

// SiteverifyURL is Cloudflare's endpoint for checking a widget's token.
const SiteverifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// configErrorCodes mean the request itself was bad, not the visitor, so a
// verification that fails with them is an outage rather than a rejection.
var configErrorCodes = []string{
	"missing-input-secret",
	"invalid-input-secret",
	"bad-request",
	"internal-error",
}

type Client struct {
	secretKey string
	verifyURL string
	http      *http.Client
}

// New returns a client that checks tokens against verifyURL, which is
// SiteverifyURL outside of tests.
func New(secretKey, verifyURL string) Client {
	return Client{
		secretKey: secretKey,
		verifyURL: verifyURL,
		http:      &http.Client{Timeout: 10 * time.Second},
	}
}

// Verify reports whether Cloudflare accepts the token. It returns an error
// only when it can't get an answer, such as when Cloudflare is unreachable or
// the secret key is wrong.
func (c Client) Verify(ctx context.Context, token string) (bool, error) {
	if token == "" {
		return false, nil
	}

	form := url.Values{
		"secret":   {c.secretKey},
		"response": {token},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.verifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return false, fmt.Errorf("siteverify request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("siteverify returned status %d", resp.StatusCode)
	}

	var result struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("siteverify returned invalid JSON: %w", err)
	}

	if result.Success {
		return true, nil
	}
	for _, code := range result.ErrorCodes {
		if slices.Contains(configErrorCodes, code) {
			return false, fmt.Errorf("siteverify failed: %s", strings.Join(result.ErrorCodes, ", "))
		}
	}
	return false, nil
}
