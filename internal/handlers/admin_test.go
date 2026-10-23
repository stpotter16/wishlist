package handlers_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
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
	body := rec.Body.String()
	if !strings.Contains(body, `data-submit-method="POST"`) {
		t.Errorf("form doesn't submit with POST")
	}
	if !strings.Contains(body, `data-submit-url="/admin/families"`) {
		t.Errorf("form doesn't submit to the create endpoint")
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

	// The server labels members itself: the first is always Mom, with no age
	// or gender, and the rest are children labeled by age.
	req := httptest.NewRequest(http.MethodPost, "/admin/families", strings.NewReader(`{
		"members": [{
			"label": "Grandma",
			"age": 40,
			"gender": "F",
			"needs": ["Winter coat", "  ", ""]
		}, {
			"label": "Ignored",
			"age": 7,
			"gender": "Girl",
			"shirtSize": "Youth M",
			"prefersUnscented": true,
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

	var momLabel, momGender string
	var momAge sql.NullInt64
	if err := d.QueryRow(context.Background(),
		`SELECT label, age, gender FROM family_member ORDER BY id LIMIT 1`,
	).Scan(&momLabel, &momAge, &momGender); err != nil {
		t.Fatalf("read mom: %v", err)
	}
	if got, want := momLabel, "Mom"; got != want {
		t.Errorf("first member label=%q, want=%q", got, want)
	}
	if momAge.Valid {
		t.Errorf("first member age=%d, want NULL", momAge.Int64)
	}
	if got, want := momGender, ""; got != want {
		t.Errorf("first member gender=%q, want=%q", got, want)
	}

	var childLabel, childGender string
	var childAge int
	if err := d.QueryRow(context.Background(),
		`SELECT label, age, gender FROM family_member ORDER BY id LIMIT 1 OFFSET 1`,
	).Scan(&childLabel, &childAge, &childGender); err != nil {
		t.Fatalf("read child: %v", err)
	}
	if got, want := childLabel, "Child, 7"; got != want {
		t.Errorf("child label=%q, want=%q", got, want)
	}
	if got, want := childAge, 7; got != want {
		t.Errorf("child age=%d, want=%d", got, want)
	}
	if got, want := childGender, "Girl"; got != want {
		t.Errorf("child gender=%q, want=%q", got, want)
	}

	// The new family appears on its public page.
	req = httptest.NewRequest(http.MethodGet, "/families/1", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("public status=%d, want=%d", got, want)
	}
	body := rec.Body.String()
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
			"child without an age is rejected",
			`{"members": [{}, {"gender": "Girl"}]}`,
		},
		{
			"child with a negative age is rejected",
			`{"members": [{}, {"age": -1}]}`,
		},
		{
			"child with an implausible age is rejected",
			`{"members": [{}, {"age": 131}]}`,
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
	if !strings.Contains(body, `datetime="2026-11-05T14:30:00Z"`) {
		t.Errorf("body is missing machine-readable claim time")
	}
	if !strings.Contains(body, "Nov 5, 2026") {
		t.Errorf("body is missing fallback claim date")
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

func TestAdminEditFamilyPageIsPrefilled(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)

	if _, err := d.Exec(context.Background(), `
	INSERT INTO family (id, created_time, last_modified_time) VALUES
		(1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO family_member (
		id, family_id, label, age, shirt_size, prefers_unscented,
		created_time, last_modified_time
	) VALUES (
		9, 1, 'Mom', NULL, '', 0,
		'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'
	), (
		10, 1, 'Child, 7', 7, 'Youth M', 1,
		'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'
	);
	INSERT INTO wishlist_item (
		family_member_id, kind, name, created_time, last_modified_time
	) VALUES
		(10, 'need', 'Winter coat', '2026-01-01T00:00:00Z',
			'2026-01-01T00:00:00Z'),
		(10, 'need', 'Rain boots', '2026-01-01T00:00:00Z',
			'2026-01-01T00:00:00Z');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/families/1/edit", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status=%d, want=%d", got, want)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `data-submit-method="PUT"`) {
		t.Errorf("form doesn't submit with PUT")
	}
	if !strings.Contains(body, `data-submit-url="/admin/families/1"`) {
		t.Errorf("form doesn't submit to the family")
	}
	if !regexp.MustCompile(`data-member-heading[^>]*>\s*Child, 7\s*</h2>`).MatchString(body) {
		t.Errorf("form is missing the child's heading")
	}
	if !strings.Contains(body, `value="7"`) {
		t.Errorf("form is missing member age")
	}
	if !strings.Contains(body, `value="Youth M"`) {
		t.Errorf("form is missing shirt size")
	}
	if !strings.Contains(body, "Winter coat\nRain boots</textarea") {
		t.Errorf("form is missing needs, one per line")
	}
	if !regexp.MustCompile(`data-field="prefersUnscented"\s+checked`).MatchString(body) {
		t.Errorf("form doesn't check the unscented checkbox")
	}
}

func TestAdminUpdateFamilyKeepsClaim(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)
	seedClaimedAndUnclaimedFamilies(t, d)

	req := httptest.NewRequest(http.MethodPut, "/admin/families/1", strings.NewReader(
		`{"members": [{"label": "Mom", "needs": ["Winter coat"]}]}`))
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status=%d, want=%d, body=%q", got, want, rec.Body.String())
	}
	if got, want := strings.TrimSpace(rec.Body.String()), `{"id":1}`; got != want {
		t.Errorf("body=%s, want=%s", got, want)
	}

	// The giver's existing link still works and shows the edit.
	req = httptest.NewRequest(http.MethodGet, "/claims/old-token", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("private link status=%d, want=%d", got, want)
	}
	if got, want := rec.Body.String(), "Winter coat"; !strings.Contains(got, want) {
		t.Errorf("private link is missing edited need %q", want)
	}
}

func TestAdminDeleteUnclaimedFamily(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)
	seedClaimedAndUnclaimedFamilies(t, d)

	req := httptest.NewRequest(http.MethodDelete, "/admin/families/2", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusNoContent; got != want {
		t.Fatalf("status=%d, want=%d", got, want)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/families/2", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if got, want := rec.Code, http.StatusNotFound; got != want {
		t.Errorf("admin page status=%d, want=%d", got, want)
	}
}

func TestAdminFamilyPageDeleteControls(t *testing.T) {
	for _, tt := range []struct {
		explanation      string
		path             string
		isDeleteExpected bool
	}{
		{
			"claimed family asks to release before deleting",
			"/admin/families/1",
			false,
		},
		{
			"unclaimed family offers delete",
			"/admin/families/2",
			true,
		},
	} {
		t.Run(fmt.Sprintf("%s [%s]", tt.explanation, tt.path), func(t *testing.T) {
			server, d := newTestServer(t)
			cookie := loginAsAdmin(t, server, d)
			seedClaimedAndUnclaimedFamilies(t, d)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.AddCookie(cookie)
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)

			body := rec.Body.String()
			if got, want := strings.Contains(body, `id="delete"`), tt.isDeleteExpected; got != want {
				t.Errorf("offers delete=%v, want=%v", got, want)
			}
			if got, want := strings.Contains(body, tt.path+"/edit"), true; got != want {
				t.Errorf("links to edit page=%v, want=%v", got, want)
			}
		})
	}
}

func TestAdminFamilyEditAndDeleteErrors(t *testing.T) {
	for _, tt := range []struct {
		explanation string
		method      string
		path        string
		body        string
		isLoggedIn  bool
		statusWant  int
	}{
		{
			"edit page requires login",
			http.MethodGet,
			"/admin/families/2/edit",
			"",
			false,
			http.StatusTemporaryRedirect,
		},
		{
			"updating requires login",
			http.MethodPut,
			"/admin/families/2",
			`{"members": [{"label": "Mom"}]}`,
			false,
			http.StatusUnauthorized,
		},
		{
			"deleting requires login",
			http.MethodDelete,
			"/admin/families/2",
			"",
			false,
			http.StatusUnauthorized,
		},
		{
			"edit page for a family that doesn't exist is not found",
			http.MethodGet,
			"/admin/families/99/edit",
			"",
			true,
			http.StatusNotFound,
		},
		{
			"updating a family that doesn't exist is not found",
			http.MethodPut,
			"/admin/families/99",
			`{"members": [{"label": "Mom"}]}`,
			true,
			http.StatusNotFound,
		},
		{
			"updating with no members is rejected",
			http.MethodPut,
			"/admin/families/2",
			`{"members": []}`,
			true,
			http.StatusBadRequest,
		},
		{
			"deleting a family that doesn't exist is not found",
			http.MethodDelete,
			"/admin/families/99",
			"",
			true,
			http.StatusNotFound,
		},
		{
			"deleting a claimed family conflicts",
			http.MethodDelete,
			"/admin/families/1",
			"",
			true,
			http.StatusConflict,
		},
	} {
		t.Run(fmt.Sprintf("%s [%s %s]", tt.explanation, tt.method, tt.path), func(t *testing.T) {
			server, d := newTestServer(t)
			seedClaimedAndUnclaimedFamilies(t, d)

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if tt.isLoggedIn {
				req.AddCookie(loginAsAdmin(t, server, d))
			}
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)

			if got, want := rec.Code, tt.statusWant; got != want {
				t.Errorf("status=%d, want=%d, body=%q", got, want, rec.Body.String())
			}
		})
	}
}

func TestAdminPagesOfferLogout(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Body.String(), `id="logout"`; !strings.Contains(got, want) {
		t.Errorf("body is missing %q", want)
	}
}

func TestFamilyFormStartsWithMom(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)

	req := httptest.NewRequest(http.MethodGet, "/admin/families/new", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status=%d, want=%d", got, want)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<h2 class="text-lg font-semibold">Mom</h2>`) {
		t.Errorf("form doesn't start with a Mom section")
	}
	// The server builds every label, so the form has no label fields.
	if strings.Contains(body, `data-field="label"`) {
		t.Errorf("form has a label field")
	}
	// Mom's section has no age or gender fields, so only the blank section
	// that "Add member" clones has them.
	if got, want := strings.Count(body, `data-field="age"`), 1; got != want {
		t.Errorf("age fields=%d, want=%d", got, want)
	}
	if got, want := strings.Count(body, `data-field="gender"`), 1; got != want {
		t.Errorf("gender fields=%d, want=%d", got, want)
	}
}

func TestEditFamilyFormTreatsFirstMemberAsMom(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)

	if _, err := d.Exec(context.Background(), `
	INSERT INTO family (id, created_time, last_modified_time) VALUES
		(1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO family_member
		(family_id, label, age, created_time, last_modified_time)
	VALUES
		(1, 'Mom', NULL, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(1, 'Child, 7', 7, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/families/1/edit", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `<h2 class="text-lg font-semibold">Mom</h2>`) {
		t.Errorf("form doesn't render the first member as Mom")
	}
	// The child's section and the "Add member" template have age fields.
	if got, want := strings.Count(body, `data-field="age"`), 2; got != want {
		t.Errorf("age fields=%d, want=%d", got, want)
	}
	if !regexp.MustCompile(`data-member-heading[^>]*>\s*Child, 7\s*</h2>`).MatchString(body) {
		t.Errorf("form is missing the child's heading")
	}
}

func TestAdminCreateFamilyLabelsAgeZeroAsInfant(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)

	req := httptest.NewRequest(http.MethodPost, "/admin/families", strings.NewReader(
		`{"members": [{}, {"age": 0}]}`))
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusCreated; got != want {
		t.Fatalf("status=%d, want=%d, body=%q", got, want, rec.Body.String())
	}

	var label string
	if err := d.QueryRow(context.Background(),
		`SELECT label FROM family_member ORDER BY id LIMIT 1 OFFSET 1`,
	).Scan(&label); err != nil {
		t.Fatalf("read child: %v", err)
	}
	if got, want := label, "Infant"; got != want {
		t.Errorf("label=%q, want=%q", got, want)
	}
}
