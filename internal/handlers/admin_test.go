package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminListsAllFamiliesWithClaimStatus(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)

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

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status=%d, want=%d", got, want)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Family 1") {
		t.Errorf("body is missing unclaimed family")
	}
	if !strings.Contains(body, "Family 2") {
		t.Errorf("body is missing claimed family")
	}
	if !strings.Contains(body, "Claimed") {
		t.Errorf("body is missing claimed status")
	}
	if !strings.Contains(body, "Unclaimed") {
		t.Errorf("body is missing unclaimed status")
	}
}

func TestAdminNewFamilyPageRenders(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)

	req := httptest.NewRequest(http.MethodGet, "/admin/families/new", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status=%d, want=%d", got, want)
	}
	if got, want := rec.Body.String(), `id="new-family"`; !strings.Contains(got, want) {
		t.Errorf("body is missing %q", want)
	}
}

func TestAdminCreateFamilyRequiresLogin(t *testing.T) {
	server, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/families", strings.NewReader(
		`{"members": [{"label": "Mom"}]}`))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusUnauthorized; got != want {
		t.Errorf("status=%d, want=%d", got, want)
	}
}

func TestAdminCreateFamilyPublishesIt(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)

	req := httptest.NewRequest(http.MethodPost, "/admin/families", strings.NewReader(`{
		"members": [{
			"label": "  Child, 7  ",
			"age": 7,
			"shirtSize": "Youth M",
			"prefersUnscented": true,
			"needs": ["Winter coat", "  ", ""],
			"wishes": ["LEGO set"]
		}]
	}`))
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusCreated; got != want {
		t.Fatalf("status=%d, want=%d, body=%q", got, want, rec.Body.String())
	}
	if got, want := strings.TrimSpace(rec.Body.String()), `{"id":1}`; got != want {
		t.Fatalf("body=%s, want=%s", got, want)
	}

	// The new family appears on its public page.
	req = httptest.NewRequest(http.MethodGet, "/families/1", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("public status=%d, want=%d", got, want)
	}
	body := rec.Body.String()
	if !strings.Contains(body, ">Child, 7<") {
		t.Errorf("public page is missing trimmed member label")
	}
	if !strings.Contains(body, "Winter coat") {
		t.Errorf("public page is missing need")
	}
	if !strings.Contains(body, "LEGO set") {
		t.Errorf("public page is missing wish")
	}
}

func TestAdminCreateFamilyRejectsInvalidInput(t *testing.T) {
	for _, tt := range []struct {
		explanation string
		body        string
	}{
		{
			"body that isn't JSON is rejected",
			`not json`,
		},
		{
			"family with no members is rejected",
			`{"members": []}`,
		},
		{
			"member without a label is rejected",
			`{"members": [{"label": "   "}]}`,
		},
		{
			"member with a negative age is rejected",
			`{"members": [{"label": "Mom", "age": -1}]}`,
		},
		{
			"member with an implausible age is rejected",
			`{"members": [{"label": "Mom", "age": 131}]}`,
		},
	} {
		t.Run(fmt.Sprintf("%s [%s]", tt.explanation, tt.body), func(t *testing.T) {
			server, d := newTestServer(t)
			cookie := loginAsAdmin(t, server, d)

			req := httptest.NewRequest(http.MethodPost, "/admin/families", strings.NewReader(tt.body))
			req.AddCookie(cookie)
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)

			if got, want := rec.Code, http.StatusBadRequest; got != want {
				t.Errorf("status=%d, want=%d", got, want)
			}
		})
	}
}
