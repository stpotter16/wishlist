package handlers_test

import (
	"context"
	"fmt"
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

func TestFamiliesErrorPageOmitsAdminNav(t *testing.T) {
	server, d := newTestServer(t)

	// Dropping a table the listing query reads forces the handler's error path.
	if _, err := d.Exec(context.Background(), `DROP TABLE claim`); err != nil {
		t.Fatalf("drop claim table: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusInternalServerError; got != want {
		t.Fatalf("status=%d, want=%d", got, want)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Something went wrong") {
		t.Errorf("body is missing error message")
	}
	if strings.Contains(body, `href="/admin"`) {
		t.Errorf("body contains link to admin dashboard")
	}
}

func TestFamilyShowsMembersAndWishlists(t *testing.T) {
	server, d := newTestServer(t)

	if _, err := d.Exec(context.Background(), `
	INSERT INTO family (id, created_time, last_modified_time) VALUES
		(1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO family_member (
		id, family_id, label, age, shirt_size, prefers_unscented,
		created_time, last_modified_time
	) VALUES (
		10, 1, 'Child, 7', 7, 'Youth M', 1,
		'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'
	);
	INSERT INTO wishlist_item (
		family_member_id, kind, name, created_time, last_modified_time
	) VALUES
		(10, 'need', 'Winter coat', '2026-01-01T00:00:00Z',
			'2026-01-01T00:00:00Z'),
		(10, 'wish', 'LEGO set', '2026-01-01T00:00:00Z',
			'2026-01-01T00:00:00Z');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/families/1", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status=%d, want=%d", got, want)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Child, 7") {
		t.Errorf("body is missing member label")
	}
	if !strings.Contains(body, "Winter coat") {
		t.Errorf("body is missing need")
	}
	if !strings.Contains(body, "LEGO set") {
		t.Errorf("body is missing wish")
	}
	if !strings.Contains(body, "Youth M") {
		t.Errorf("body is missing shirt size")
	}
	if !strings.Contains(body, "Prefers unscented products") {
		t.Errorf("body is missing unscented preference")
	}
	// The label already says the child's age.
	if strings.Contains(body, "Age 7") {
		t.Errorf("body repeats the age from the label")
	}
}

func TestFamilyNotFound(t *testing.T) {
	for _, tt := range []struct {
		explanation string
		path        string
	}{
		{
			"family that doesn't exist is not found",
			"/families/99",
		},
		{
			"claimed family is not found",
			"/families/2",
		},
		{
			"non-numeric family ID is not found",
			"/families/abc",
		},
	} {
		t.Run(fmt.Sprintf("%s [%s]", tt.explanation, tt.path), func(t *testing.T) {
			server, d := newTestServer(t)

			// Family 2 is claimed.
			if _, err := d.Exec(context.Background(), `
			INSERT INTO family (id, created_time, last_modified_time) VALUES
				(2, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
			INSERT INTO family_member
				(family_id, label, created_time, last_modified_time)
			VALUES
				(2, 'Dad', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
			INSERT INTO claim (
				family_id, giver_name, giver_email, giver_phone,
				access_token_hash, created_time
			) VALUES (
				2, 'Giver', 'giver@example.com', '555-0100', 'dummy-hash',
				'2026-01-01T00:00:00Z'
			);
			`); err != nil {
				t.Fatalf("seed: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)

			if got, want := rec.Code, http.StatusNotFound; got != want {
				t.Fatalf("status=%d, want=%d", got, want)
			}
			if got, want := rec.Body.String(), "Page not found"; !strings.Contains(got, want) {
				t.Errorf("body is missing %q", want)
			}
		})
	}
}
