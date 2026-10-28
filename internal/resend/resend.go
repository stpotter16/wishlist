// Package resend sends plain-text email through Resend's HTTP API.
package resend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// EmailsURL is Resend's endpoint for sending one email.
const EmailsURL = "https://api.resend.com/emails"

type Params struct {
	APIKey string
	// From is the sender, such as "Wishlist <noreply@example.org>". Its domain
	// must be verified in Resend.
	From    string
	ReplyTo string
	// URL is EmailsURL outside of tests.
	URL string
}

type Client struct {
	params Params
	http   *http.Client
}

func New(params Params) Client {
	return Client{
		params: params,
		http:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Send emails a plain-text message. Its errors never include the message, so
// callers can log them even when the body carries a secret link.
func (c Client) Send(ctx context.Context, to, subject, body string) error {
	payload, err := json.Marshal(struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		ReplyTo string   `json:"reply_to"`
		Subject string   `json:"subject"`
		Text    string   `json:"text"`
	}{
		From:    c.params.From,
		To:      []string{to},
		ReplyTo: c.params.ReplyTo,
		Subject: subject,
		Text:    body,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.params.URL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.params.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("resend request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Resend's error bodies describe the problem, such as an unverified
		// domain, without echoing the email.
		var result struct {
			Message string `json:"message"`
		}
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = json.Unmarshal(respBody, &result)
		return fmt.Errorf("resend returned status %d: %s", resp.StatusCode, result.Message)
	}
	return nil
}
