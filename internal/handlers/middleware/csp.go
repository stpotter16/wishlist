package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
)

type contextKey struct {
	name string
}

const CSP_KEY = "csp"

const turnstileOrigin = "https://challenges.cloudflare.com"

func CspMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce, err := generateNonce()
		if err != nil {
			log.Printf("Could not generate nonce: %v", err)
			http.Error(w, "Error generating session information", http.StatusInternalServerError)
			return
		}

		type cspDirective struct {
			name   string
			values []string
		}

		directives := []cspDirective{
			{
				name: "script-src",
				values: []string{
					"'nonce-" + nonce + "'",
					// Cloudflare Turnstile guards the claim form. Its loader
					// carries the nonce, but Cloudflare's docs also allow
					// the host for the scripts it loads.
					turnstileOrigin,
				},
			},
			{
				name: "object-src",
				values: []string{
					"'none'",
				},
			},
			{
				name: "base-uri",
				values: []string{
					"'none'",
				},
			},
		}

		policyParts := []string{}
		for _, directive := range directives {
			policyParts = append(policyParts, fmt.Sprintf("%s %s", directive.name, strings.Join(directive.values, " ")))
		}
		policy := strings.Join(policyParts, "; ") + ";"

		w.Header().Set("Content-Security-Policy", policy)
		ctxKey := contextKey{CSP_KEY}
		ctx := context.WithValue(r.Context(), ctxKey, nonce)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func NonceFromContext(ctx context.Context) (string, error) {
	ctxKey := contextKey{CSP_KEY}
	nonce, ok := ctx.Value(ctxKey).(string)
	if !ok {
		return "", errors.New("could not extract csp nonce")
	}
	return nonce, nil
}

func generateNonce() (string, error) {
	randomReader := rand.Reader
	byteSlice := make([]byte, 16)
	if _, err := randomReader.Read(byteSlice); err != nil {
		return "", err
	}
	token := base64.URLEncoding.EncodeToString(byteSlice)
	return token, nil
}
