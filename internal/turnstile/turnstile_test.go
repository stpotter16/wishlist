package turnstile_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stpotter16/wishlist/internal/turnstile"
)

func TestVerify(t *testing.T) {
	for _, tt := range []struct {
		explanation    string
		token          string
		status         int
		body           string
		acceptedWanted bool
		errWanted      bool
		// secretWanted is the secret Cloudflare should receive, or empty if
		// the client shouldn't call Cloudflare at all.
		secretWanted string
	}{
		{
			"token Cloudflare accepts is accepted",
			"good-token",
			http.StatusOK,
			`{"success": true, "error-codes": []}`,
			true,
			false,
			"dummy-secret",
		},
		{
			"token Cloudflare rejects is rejected",
			"bad-token",
			http.StatusOK,
			`{"success": false, "error-codes": ["invalid-input-response"]}`,
			false,
			false,
			"dummy-secret",
		},
		{
			"token that was already used is rejected",
			"used-token",
			http.StatusOK,
			`{"success": false, "error-codes": ["timeout-or-duplicate"]}`,
			false,
			false,
			"dummy-secret",
		},
		{
			"empty token is rejected without asking Cloudflare",
			"",
			http.StatusInternalServerError,
			``,
			false,
			false,
			"",
		},
		{
			"wrong secret key is an error, not a rejection",
			"good-token",
			http.StatusOK,
			`{"success": false, "error-codes": ["invalid-input-secret"]}`,
			false,
			true,
			"dummy-secret",
		},
		{
			"Cloudflare server error is an error",
			"good-token",
			http.StatusInternalServerError,
			``,
			false,
			true,
			"dummy-secret",
		},
		{
			"malformed response is an error",
			"good-token",
			http.StatusOK,
			`not json`,
			false,
			true,
			"dummy-secret",
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			var gotSecret, gotResponse string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotSecret = r.FormValue("secret")
				gotResponse = r.FormValue("response")
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := turnstile.New("dummy-secret", server.URL)
			accepted, err := client.Verify(context.Background(), tt.token)
			if got, want := err != nil, tt.errWanted; got != want {
				t.Fatalf("err=%v, want error=%v", err, want)
			}
			if got, want := accepted, tt.acceptedWanted; got != want {
				t.Errorf("accepted=%v, want=%v", got, want)
			}
			if got, want := gotSecret, tt.secretWanted; got != want {
				t.Errorf("secret=%q, want=%q", got, want)
			}
			if got, want := gotResponse, tt.token; got != want {
				t.Errorf("response=%q, want=%q", got, want)
			}
		})
	}
}
