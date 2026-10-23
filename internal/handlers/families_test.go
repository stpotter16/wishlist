package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFamiliesListsOnlyUnclaimedFamilies(t *testing.T) {
	server, d := newTestServer(t)

	// Family 1 is unclaimed. Family 2 is claimed.
	if _, err := d.Exec(context.Background(), `
	INSERT INTO family (id, created_time, last_modified_time) VALUES
		(1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(2, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO family_member
		(family_id, label, created_time, last_modified_time)
	VALUES
		(1, 'Mom', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(2, 'Dad', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO claim (
		family_id, giver_name, giver_email, giver_phone, access_token_hash,
		created_time
	) VALUES (
		2, 'Giver', 'giver@example.com', '555-0100', 'dummy-hash',
		'2026-01-01T00:00:00Z'
	);
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status=%d, want=%d", got, want)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Family 1") {
		t.Errorf("body is missing unclaimed family's label")
	}
	if !strings.Contains(body, "Mom") {
		t.Errorf("body is missing unclaimed family's member label")
	}
	if !strings.Contains(body, `href="/families/1"`) {
		t.Errorf("body is missing link to unclaimed family")
	}
	if strings.Contains(body, "Family 2") {
		t.Errorf("body contains claimed family's label")
	}
	if strings.Contains(body, "Dad") {
		t.Errorf("body contains claimed family's member label")
	}
}

func TestFamiliesShowsEmptyState(t *testing.T) {
	server, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status=%d, want=%d", got, want)
	}
	if got, want := rec.Body.String(), "No families available"; !strings.Contains(got, want) {
		t.Errorf("body is missing %q", want)
	}
}
