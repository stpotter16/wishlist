package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stpotter16/wishlist/internal/auth"
	"github.com/stpotter16/wishlist/internal/store/db"
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

// seedClaimedAndUnclaimedFamilies seeds family 1, claimed by Pat Doe, whose
// private link token is "old-token"; family 2, unclaimed; and family 3, with
// no members.
func seedClaimedAndUnclaimedFamilies(t *testing.T, d db.DB) {
	t.Helper()

	if _, err := d.Exec(context.Background(), `
	INSERT INTO family (id, created_time, last_modified_time) VALUES
		(1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(2, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(3, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO family_member
		(family_id, label, created_time, last_modified_time)
	VALUES
		(1, 'Mom', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(2, 'Dad', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO claim (
		family_id, giver_name, giver_email, giver_phone, access_token_hash,
		created_time
	) VALUES (
		1, 'Pat Doe', 'giver@example.com', '555-0100', ?,
		'2026-11-05T14:30:00Z'
	);
	`, auth.HashAccessToken("old-token")); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestAdminDashboardLinksToFamilyPages(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)
	seedClaimedAndUnclaimedFamilies(t, d)

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Body.String(), `href="/admin/families/1"`; !strings.Contains(got, want) {
		t.Errorf("body is missing %q", want)
	}
}

func TestAdminFamilyPageRequiresLogin(t *testing.T) {
	server, d := newTestServer(t)
	seedClaimedAndUnclaimedFamilies(t, d)

	req := httptest.NewRequest(http.MethodGet, "/admin/families/1", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusTemporaryRedirect; got != want {
		t.Errorf("status=%d, want=%d", got, want)
	}
}

func TestAdminFamilyPageShowsGiverOfClaimedFamily(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)
	seedClaimedAndUnclaimedFamilies(t, d)

	req := httptest.NewRequest(http.MethodGet, "/admin/families/1", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status=%d, want=%d", got, want)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Pat Doe") {
		t.Errorf("body is missing giver name")
	}
	if !strings.Contains(body, `href="mailto:giver@example.com"`) {
		t.Errorf("body is missing giver email link")
	}
	if !strings.Contains(body, "555-0100") {
		t.Errorf("body is missing giver phone")
	}
	if !strings.Contains(body, "Nov 5, 2026") {
		t.Errorf("body is missing claim date")
	}
	if !strings.Contains(body, "Mom") {
		t.Errorf("body is missing member label")
	}
}

func TestAdminFamilyPageOfUnclaimedFamily(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)
	seedClaimedAndUnclaimedFamilies(t, d)

	req := httptest.NewRequest(http.MethodGet, "/admin/families/2", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status=%d, want=%d", got, want)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `href="/families/2"`) {
		t.Errorf("body is missing link to public page")
	}
	if strings.Contains(body, "Release claim") {
		t.Errorf("body offers to release a claim that doesn't exist")
	}
}

func TestAdminFamilyPageOfMemberlessFamily(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)
	seedClaimedAndUnclaimedFamilies(t, d)

	req := httptest.NewRequest(http.MethodGet, "/admin/families/3", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status=%d, want=%d", got, want)
	}
	if got, want := rec.Body.String(), "No members"; !strings.Contains(got, want) {
		t.Errorf("body is missing %q", want)
	}
}

func TestAdminFamilyPageNotFound(t *testing.T) {
	for _, tt := range []struct {
		explanation string
		path        string
	}{
		{
			"family that doesn't exist is not found",
			"/admin/families/99",
		},
		{
			"non-numeric family ID is not found",
			"/admin/families/abc",
		},
	} {
		t.Run(fmt.Sprintf("%s [%s]", tt.explanation, tt.path), func(t *testing.T) {
			server, d := newTestServer(t)
			cookie := loginAsAdmin(t, server, d)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.AddCookie(cookie)
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)

			if got, want := rec.Code, http.StatusNotFound; got != want {
				t.Errorf("status=%d, want=%d", got, want)
			}
		})
	}
}

func TestAdminReleaseClaimReturnsFamilyToPublic(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)
	seedClaimedAndUnclaimedFamilies(t, d)

	req := httptest.NewRequest(http.MethodDelete, "/admin/families/1/claim", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusNoContent; got != want {
		t.Fatalf("release status=%d, want=%d", got, want)
	}

	req = httptest.NewRequest(http.MethodGet, "/families/1", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if got, want := rec.Code, http.StatusOK; got != want {
		t.Errorf("public page status=%d, want=%d", got, want)
	}

	req = httptest.NewRequest(http.MethodGet, "/claims/old-token", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if got, want := rec.Code, http.StatusNotFound; got != want {
		t.Errorf("old private link status=%d, want=%d", got, want)
	}
}

func TestAdminIssueNewLinkReplacesOldLink(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)
	seedClaimedAndUnclaimedFamilies(t, d)

	req := httptest.NewRequest(http.MethodPost, "/admin/families/1/claim/token", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("issue status=%d, want=%d", got, want)
	}
	var claim struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&claim); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, claim.URL, nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if got, want := rec.Code, http.StatusOK; got != want {
		t.Errorf("new private link status=%d, want=%d", got, want)
	}

	req = httptest.NewRequest(http.MethodGet, "/claims/old-token", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if got, want := rec.Code, http.StatusNotFound; got != want {
		t.Errorf("old private link status=%d, want=%d", got, want)
	}
}

func TestAdminClaimActions(t *testing.T) {
	for _, tt := range []struct {
		explanation string
		method      string
		path        string
		isLoggedIn  bool
		statusWant  int
	}{
		{
			"releasing a claim requires login",
			http.MethodDelete,
			"/admin/families/1/claim",
			false,
			http.StatusUnauthorized,
		},
		{
			"issuing a new link requires login",
			http.MethodPost,
			"/admin/families/1/claim/token",
			false,
			http.StatusUnauthorized,
		},
		{
			"releasing an unclaimed family is not found",
			http.MethodDelete,
			"/admin/families/2/claim",
			true,
			http.StatusNotFound,
		},
		{
			"issuing a link for an unclaimed family is not found",
			http.MethodPost,
			"/admin/families/2/claim/token",
			true,
			http.StatusNotFound,
		},
	} {
		t.Run(fmt.Sprintf("%s [%s %s]", tt.explanation, tt.method, tt.path), func(t *testing.T) {
			server, d := newTestServer(t)
			seedClaimedAndUnclaimedFamilies(t, d)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.isLoggedIn {
				req.AddCookie(loginAsAdmin(t, server, d))
			}
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)

			if got, want := rec.Code, tt.statusWant; got != want {
				t.Errorf("status=%d, want=%d", got, want)
			}
		})
	}
}
