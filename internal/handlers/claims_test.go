package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClaimHouseholdGivesGiverPrivateLink(t *testing.T) {
	server, d := newTestServer(t)

	if _, err := d.Exec(context.Background(), `
	INSERT INTO household (id, kind, created_time, last_modified_time) VALUES
		(1, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO household_member
		(id, household_id, first_name, label, created_time, last_modified_time)
	VALUES
		(10, 1, 'Maria', 'Mom', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO wishlist_item (
		household_member_id, kind, name, created_time, last_modified_time
	) VALUES
		(10, 'need', 'Winter coat', '2026-01-01T00:00:00Z',
			'2026-01-01T00:00:00Z');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/households/1/claim", strings.NewReader(
		`{"name": "Giver", "email": "giver@example.com", "phone": "555-0100"}`))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusCreated; got != want {
		t.Fatalf("claim status=%d, want=%d, body=%q", got, want, rec.Body.String())
	}
	var claim struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&claim); err != nil {
		t.Fatalf("decode claim response: %v", err)
	}
	if !strings.HasPrefix(claim.URL, "/claims/") {
		t.Fatalf("url=%q, want prefix /claims/", claim.URL)
	}

	// The private link shows the household.
	req = httptest.NewRequest(http.MethodGet, claim.URL, nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("private link status=%d, want=%d", got, want)
	}
	// Collapse whitespace like a browser does, so template formatting doesn't
	// matter.
	body := strings.Join(strings.Fields(rec.Body.String()), " ")
	if !strings.Contains(body, "You adopted Household 1") {
		t.Errorf("private link is missing household heading")
	}
	if !strings.Contains(body, "Maria") {
		t.Errorf("private link is missing member name")
	}
	if !strings.Contains(body, "Winter coat") {
		t.Errorf("private link is missing need")
	}
	if got, want := rec.Header().Get("Cache-Control"), "no-store"; got != want {
		t.Errorf("Cache-Control=%q, want=%q", got, want)
	}
	if got, want := rec.Header().Get("Referrer-Policy"), "no-referrer"; got != want {
		t.Errorf("Referrer-Policy=%q, want=%q", got, want)
	}

	// The household leaves the public listing and its public page.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if strings.Contains(rec.Body.String(), "Household 1") {
		t.Errorf("public listing still shows claimed household")
	}

	req = httptest.NewRequest(http.MethodGet, "/households/1", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if got, want := rec.Code, http.StatusNotFound; got != want {
		t.Errorf("public household page status=%d, want=%d", got, want)
	}
}

func TestClaimHouseholdRejectsInvalidClaims(t *testing.T) {
	for _, tt := range []struct {
		explanation string
		path        string
		body        string
		statusWant  int
	}{
		{
			"body that isn't JSON is rejected",
			"/households/1/claim",
			`not json`,
			http.StatusBadRequest,
		},
		{
			"claim without a name is rejected",
			"/households/1/claim",
			`{"name": "  ", "email": "new@example.com", "phone": "555-0199"}`,
			http.StatusBadRequest,
		},
		{
			"claim without a phone is rejected",
			"/households/1/claim",
			`{"name": "New", "email": "new@example.com", "phone": ""}`,
			http.StatusBadRequest,
		},
		{
			"claim with a malformed email is rejected",
			"/households/1/claim",
			`{"name": "New", "email": "not-an-email", "phone": "555-0199"}`,
			http.StatusBadRequest,
		},
		{
			"claim with a display-name email is rejected",
			"/households/1/claim",
			`{"name": "New", "email": "New <new@example.com>", "phone": "555-0199"}`,
			http.StatusBadRequest,
		},
		{
			"claim on a household that doesn't exist is not found",
			"/households/99/claim",
			`{"name": "New", "email": "new@example.com", "phone": "555-0199"}`,
			http.StatusNotFound,
		},
		{
			"claim on a non-numeric household ID is not found",
			"/households/abc/claim",
			`{"name": "New", "email": "new@example.com", "phone": "555-0199"}`,
			http.StatusNotFound,
		},
		{
			"claim on an already claimed household conflicts",
			"/households/2/claim",
			`{"name": "New", "email": "new@example.com", "phone": "555-0199"}`,
			http.StatusConflict,
		},
		{
			"claim with an email that already claimed a household conflicts",
			"/households/1/claim",
			`{"name": "New", "email": "giver@example.com", "phone": "555-0199"}`,
			http.StatusConflict,
		},
	} {
		t.Run(fmt.Sprintf("%s [%s]", tt.explanation, tt.path), func(t *testing.T) {
			server, d := newTestServer(t)

			// Household 1 is unclaimed. giver@example.com claimed household 2.
			if _, err := d.Exec(context.Background(), `
			INSERT INTO household (id, kind, created_time, last_modified_time) VALUES
				(1, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
				(2, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
			INSERT INTO household_member
				(household_id, first_name, label, created_time, last_modified_time)
			VALUES
				(1, 'Maria', 'Mom', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
				(2, 'Dan', 'Dad', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
			INSERT INTO claim (
				household_id, giver_name, giver_email, giver_phone,
				access_token_hash, created_time
			) VALUES (
				2, 'Giver', 'giver@example.com', '555-0100', 'existing-hash',
				'2026-01-01T00:00:00Z'
			);
			`); err != nil {
				t.Fatalf("seed: %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)

			if got, want := rec.Code, tt.statusWant; got != want {
				t.Errorf("status=%d, want=%d, body=%q", got, want, rec.Body.String())
			}
		})
	}
}

func TestClaimPageUnknownTokenIsNotFound(t *testing.T) {
	server, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/claims/not-a-real-token", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusNotFound; got != want {
		t.Errorf("status=%d, want=%d", got, want)
	}
}

func TestClaimRateLimitPerNetwork(t *testing.T) {
	for _, tt := range []struct {
		explanation     string
		priorAttemptsIP string
		priorAttempts   int
		requestIP       string
		elapsed         time.Duration
		statusExpected  int
	}{
		{
			"attempts under the limit reach the claim handler",
			"203.0.113.1",
			9,
			"203.0.113.1",
			0,
			http.StatusNotFound,
		},
		{
			"an attempt past the limit is rejected",
			"203.0.113.1",
			10,
			"203.0.113.1",
			0,
			http.StatusTooManyRequests,
		},
		{
			"one network's attempts don't limit another network",
			"203.0.113.1",
			10,
			"203.0.113.2",
			0,
			http.StatusNotFound,
		},
		{
			"the limit resets after an hour",
			"203.0.113.1",
			10,
			"203.0.113.1",
			time.Hour,
			http.StatusNotFound,
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			server, _ := newTestServerWithClock(t, func() time.Time { return now })

			// Claims on a missing household still count toward the limit.
			claim := func(ip string) *httptest.ResponseRecorder {
				req := httptest.NewRequest(http.MethodPost, "/households/999/claim", strings.NewReader(
					`{"name": "Giver", "email": "giver@example.com", "phone": "555-0100"}`))
				req.Header.Set("Fly-Client-IP", ip)
				rec := httptest.NewRecorder()
				server.ServeHTTP(rec, req)
				return rec
			}
			for range tt.priorAttempts {
				if got, want := claim(tt.priorAttemptsIP).Code, http.StatusNotFound; got != want {
					t.Fatalf("prior attempt status=%d, want=%d", got, want)
				}
			}

			now = now.Add(tt.elapsed)
			rec := claim(tt.requestIP)
			if got, want := rec.Code, tt.statusExpected; got != want {
				t.Errorf("status=%d, want=%d, body=%q", got, want, rec.Body.String())
			}
		})
	}
}

func TestClaimHouseholdRequiresBotCheck(t *testing.T) {
	for _, tt := range []struct {
		explanation      string
		verifier         *fakeHumanVerifier
		statusWant       int
		claimCreatedWant bool
	}{
		{
			"giver who passes the bot check claims the household",
			&fakeHumanVerifier{isHuman: true},
			http.StatusCreated,
			true,
		},
		{
			"giver who fails the bot check is forbidden",
			&fakeHumanVerifier{isHuman: false},
			http.StatusForbidden,
			false,
		},
		{
			"claim is refused when the bot check is unavailable",
			&fakeHumanVerifier{err: errors.New("dummy outage")},
			http.StatusServiceUnavailable,
			false,
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			server, d := newTestServerWithVerifier(t, func() time.Time {
				return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			}, tt.verifier)

			if _, err := d.Exec(context.Background(), `
			INSERT INTO household (id, kind, created_time, last_modified_time) VALUES
				(1, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
			INSERT INTO household_member
				(household_id, first_name, label, created_time, last_modified_time)
			VALUES
				(1, 'Maria', 'Mom', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
			`); err != nil {
				t.Fatalf("seed: %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/households/1/claim", strings.NewReader(
				`{"name": "Giver", "email": "giver@example.com", "phone": "555-0100", "turnstileToken": "dummy-token"}`))
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)

			if got, want := rec.Code, tt.statusWant; got != want {
				t.Errorf("status=%d, want=%d, body=%q", got, want, rec.Body.String())
			}
			if got, want := tt.verifier.gotToken, "dummy-token"; got != want {
				t.Errorf("verified token=%q, want=%q", got, want)
			}

			var claims int
			if err := d.QueryRow(context.Background(),
				`SELECT COUNT(*) FROM claim WHERE household_id = 1`).Scan(&claims); err != nil {
				t.Fatalf("count claims: %v", err)
			}
			if got, want := claims == 1, tt.claimCreatedWant; got != want {
				t.Errorf("claim created=%v, want=%v", got, want)
			}
		})
	}
}
