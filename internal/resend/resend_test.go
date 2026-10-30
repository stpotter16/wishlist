package resend_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stpotter16/wishlist/internal/resend"
)

func TestSend(t *testing.T) {
	for _, tt := range []struct {
		explanation string
		status      int
		body        string
		errWanted   bool
	}{
		{
			"email Resend accepts is sent",
			http.StatusOK,
			`{"id": "dummy-id"}`,
			false,
		},
		{
			"email Resend rejects is an error",
			http.StatusForbidden,
			`{"statusCode": 403, "name": "validation_error", "message": "The example.org domain is not verified."}`,
			true,
		},
		{
			"Resend server error is an error",
			http.StatusInternalServerError,
			``,
			true,
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			var gotAuth string
			var got struct {
				From    string   `json:"from"`
				To      []string `json:"to"`
				ReplyTo string   `json:"reply_to"`
				Subject string   `json:"subject"`
				Text    string   `json:"text"`
				HTML    string   `json:"html"`
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuth = r.Header.Get("Authorization")
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Errorf("decode request: %v", err)
				}
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := resend.New(resend.Params{
				APIKey:  "dummy-key",
				From:    "Wishlist <noreply@example.org>",
				ReplyTo: "organizers@example.org",
				URL:     server.URL,
			})
			err := client.Send(context.Background(), "giver@example.com", "Dummy subject",
				"dummy text with https://example.org/claims/secret-token",
				`<a href="https://example.org/claims/secret-token">dummy html</a>`)
			if got, want := err != nil, tt.errWanted; got != want {
				t.Fatalf("err=%v, want error=%v", err, want)
			}
			if err != nil && strings.Contains(err.Error(), "secret-token") {
				t.Errorf("err=%v, want no email body in the error", err)
			}

			if got, want := gotAuth, "Bearer dummy-key"; got != want {
				t.Errorf("Authorization=%q, want=%q", got, want)
			}
			if got, want := got.From, "Wishlist <noreply@example.org>"; got != want {
				t.Errorf("from=%q, want=%q", got, want)
			}
			if got, want := strings.Join(got.To, ","), "giver@example.com"; got != want {
				t.Errorf("to=%q, want=%q", got, want)
			}
			if got, want := got.ReplyTo, "organizers@example.org"; got != want {
				t.Errorf("reply_to=%q, want=%q", got, want)
			}
			if got, want := got.Subject, "Dummy subject"; got != want {
				t.Errorf("subject=%q, want=%q", got, want)
			}
			if got, want := got.Text, "dummy text with https://example.org/claims/secret-token"; got != want {
				t.Errorf("text=%q, want=%q", got, want)
			}
			if got, want := got.HTML, `<a href="https://example.org/claims/secret-token">dummy html</a>`; got != want {
				t.Errorf("html=%q, want=%q", got, want)
			}
		})
	}
}
