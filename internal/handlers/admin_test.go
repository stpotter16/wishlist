package handlers_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/stpotter16/wishlist/internal/auth"
	"github.com/stpotter16/wishlist/internal/store/db"
)

func TestAdminListsAllHouseholdsWithClaimStatus(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)

	// Household 1 is unclaimed. Household 2 is claimed.
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
		household_id, giver_name, giver_email, giver_phone, access_token_hash,
		created_time
	) VALUES (
		2, 'Pat Jones', 'giver@example.com', '555-0100', 'dummy-hash',
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
	body := strings.Join(strings.Fields(rec.Body.String()), " ")
	if !strings.Contains(body, "Blackbird Place Resident/Family 1") {
		t.Errorf("body is missing unclaimed household")
	}
	if !strings.Contains(body, "Blackbird Place Resident/Family 2") {
		t.Errorf("body is missing claimed household")
	}
	if !strings.Contains(body, "Claimed") {
		t.Errorf("body is missing claimed status")
	}
	if !strings.Contains(body, "Unclaimed") {
		t.Errorf("body is missing unclaimed status")
	}
	// Collapse whitespace like a browser does, so template formatting doesn't
	// matter.
	if !strings.Contains(body, "Claimed by Pat Jones - giver@example.com") {
		t.Errorf("body is missing the claimed household's giver")
	}
}

func TestAdminNewHouseholdPageRenders(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)

	req := httptest.NewRequest(http.MethodGet, "/admin/households/new", nil)
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
	if !strings.Contains(body, `data-submit-url="/admin/households"`) {
		t.Errorf("form doesn't submit to the create endpoint")
	}
}

func TestAdminCreateHouseholdRequiresLogin(t *testing.T) {
	server, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/households", strings.NewReader(
		`{"kind": "single", "members": [{"firstName": "Rosa", "lastInitial": "M"}]}`))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusUnauthorized; got != want {
		t.Errorf("status=%d, want=%d", got, want)
	}
}

func TestAdminCreateHouseholdPublishesIt(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)

	// The server labels members itself: a family's first member is always
	// Mom, with no age or gender, and the rest are children labeled by age and
	// named by first name only.
	req := httptest.NewRequest(http.MethodPost, "/admin/households", strings.NewReader(`{
		"kind": "family",
		"members": [{
			"firstName": " Maria ",
			"lastInitial": "g.",
			"label": "Grandma",
			"age": 40,
			"gender": "F",
			"needs": ["Winter coat", "  ", ""]
		}, {
			"firstName": "Sofia",
			"lastInitial": "R",
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

	var kind string
	if err := d.QueryRow(context.Background(),
		`SELECT kind FROM household WHERE id = 1`,
	).Scan(&kind); err != nil {
		t.Fatalf("read household: %v", err)
	}
	if got, want := kind, "family"; got != want {
		t.Errorf("kind=%q, want=%q", got, want)
	}

	var momFirstName, momLastInitial, momLabel, momGender string
	var momAge sql.NullInt64
	if err := d.QueryRow(context.Background(),
		`SELECT first_name, last_initial, label, age, gender FROM household_member ORDER BY id LIMIT 1`,
	).Scan(&momFirstName, &momLastInitial, &momLabel, &momAge, &momGender); err != nil {
		t.Fatalf("read mom: %v", err)
	}
	if got, want := momFirstName, "Maria"; got != want {
		t.Errorf("first member first name=%q, want=%q", got, want)
	}
	if got, want := momLastInitial, "G"; got != want {
		t.Errorf("first member last initial=%q, want=%q", got, want)
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

	var childFirstName, childLastInitial, childLabel, childGender string
	var childAge int
	if err := d.QueryRow(context.Background(),
		`SELECT first_name, last_initial, label, age, gender FROM household_member ORDER BY id LIMIT 1 OFFSET 1`,
	).Scan(&childFirstName, &childLastInitial, &childLabel, &childAge, &childGender); err != nil {
		t.Fatalf("read child: %v", err)
	}
	if got, want := childFirstName, "Sofia"; got != want {
		t.Errorf("child first name=%q, want=%q", got, want)
	}
	if got, want := childLastInitial, ""; got != want {
		t.Errorf("child last initial=%q, want=%q", got, want)
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

	// The new household appears on its public page.
	req = httptest.NewRequest(http.MethodGet, "/households/1", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("public status=%d, want=%d", got, want)
	}
	body := rec.Body.String()
	for _, want := range []string{"Family", "Maria G.", "Sofia", "Winter coat", "LEGO set"} {
		if !strings.Contains(body, want) {
			t.Errorf("public page is missing %q", want)
		}
	}
}

func TestAdminCreateSingleWomanHousehold(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)

	req := httptest.NewRequest(http.MethodPost, "/admin/households", strings.NewReader(`{
		"kind": "single",
		"members": [{
			"firstName": "Rosa",
			"lastInitial": "M",
			"needs": ["Slippers"]
		}]
	}`))
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusCreated; got != want {
		t.Fatalf("status=%d, want=%d, body=%q", got, want, rec.Body.String())
	}

	// The listing shows only her anonymous label. Her name appears on the
	// household's own page.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	body := strings.Join(strings.Fields(rec.Body.String()), " ")
	for _, want := range []string{"Blackbird Place Resident/Family 1", "Single woman", "Woman"} {
		if !strings.Contains(body, want) {
			t.Errorf("listing is missing %q", want)
		}
	}
	if strings.Contains(body, "Rosa") {
		t.Errorf("listing shows her name")
	}

	req = httptest.NewRequest(http.MethodGet, "/households/1", nil)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	body = rec.Body.String()
	for _, want := range []string{"Single woman", "Rosa M.", "Woman", "Slippers"} {
		if !strings.Contains(body, want) {
			t.Errorf("household page is missing %q", want)
		}
	}
}

func TestAdminCreateHouseholdRejectsInvalidInput(t *testing.T) {
	for _, tt := range []struct {
		explanation string
		body        string
	}{
		{
			"body that isn't JSON is rejected",
			`not json`,
		},
		{
			"household without a kind is rejected",
			`{"members": [{"firstName": "Rosa", "lastInitial": "M"}]}`,
		},
		{
			"household with an unknown kind is rejected",
			`{"kind": "couple", "members": [{"firstName": "Rosa", "lastInitial": "M"}]}`,
		},
		{
			"single woman's household with no members is rejected",
			`{"kind": "single", "members": []}`,
		},
		{
			"single woman's household with a child is rejected",
			`{"kind": "single", "members": [{"firstName": "Rosa", "lastInitial": "M"}, {"firstName": "Sofia", "age": 7}]}`,
		},
		{
			"family without a child is rejected",
			`{"kind": "family", "members": [{"firstName": "Maria", "lastInitial": "G"}]}`,
		},
		{
			"head without a first name is rejected",
			`{"kind": "single", "members": [{"firstName": "  ", "lastInitial": "M"}]}`,
		},
		{
			"head without a last initial is rejected",
			`{"kind": "single", "members": [{"firstName": "Rosa"}]}`,
		},
		{
			"head with a full last name is rejected",
			`{"kind": "single", "members": [{"firstName": "Rosa", "lastInitial": "Martinez"}]}`,
		},
		{
			"head with a last initial that isn't a letter is rejected",
			`{"kind": "single", "members": [{"firstName": "Rosa", "lastInitial": "7"}]}`,
		},
		{
			"child without a first name is rejected",
			`{"kind": "family", "members": [{"firstName": "Maria", "lastInitial": "G"}, {"age": 7}]}`,
		},
		{
			"child without an age is rejected",
			`{"kind": "family", "members": [{"firstName": "Maria", "lastInitial": "G"}, {"firstName": "Sofia", "gender": "Girl"}]}`,
		},
		{
			"child with a negative age is rejected",
			`{"kind": "family", "members": [{"firstName": "Maria", "lastInitial": "G"}, {"firstName": "Sofia", "age": -1}]}`,
		},
		{
			"child with an implausible age is rejected",
			`{"kind": "family", "members": [{"firstName": "Maria", "lastInitial": "G"}, {"firstName": "Sofia", "age": 131}]}`,
		},
	} {
		t.Run(fmt.Sprintf("%s [%s]", tt.explanation, tt.body), func(t *testing.T) {
			server, d := newTestServer(t)
			cookie := loginAsAdmin(t, server, d)

			req := httptest.NewRequest(http.MethodPost, "/admin/households", strings.NewReader(tt.body))
			req.AddCookie(cookie)
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)

			if got, want := rec.Code, http.StatusBadRequest; got != want {
				t.Errorf("status=%d, want=%d", got, want)
			}
		})
	}
}

// seedClaimedAndUnclaimedHouseholds seeds household 1, claimed by Pat Doe, whose
// private link token is "old-token"; household 2, unclaimed; and household 3, with
// no members.
func seedClaimedAndUnclaimedHouseholds(t *testing.T, d db.DB) {
	t.Helper()

	if _, err := d.Exec(context.Background(), `
	INSERT INTO household (id, kind, created_time, last_modified_time) VALUES
		(1, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(2, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(3, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO household_member
		(household_id, first_name, label, created_time, last_modified_time)
	VALUES
		(1, 'Maria', 'Mom', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(2, 'Dan', 'Dad', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO claim (
		household_id, giver_name, giver_email, giver_phone, access_token_hash,
		created_time
	) VALUES (
		1, 'Pat Doe', 'giver@example.com', '555-0100', ?,
		'2026-11-05T14:30:00Z'
	);
	`, auth.HashAccessToken("old-token")); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestAdminDashboardLinksToHouseholdPages(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)
	seedClaimedAndUnclaimedHouseholds(t, d)

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Body.String(), `href="/admin/households/1"`; !strings.Contains(got, want) {
		t.Errorf("body is missing %q", want)
	}
}

func TestAdminHouseholdPageRequiresLogin(t *testing.T) {
	server, d := newTestServer(t)
	seedClaimedAndUnclaimedHouseholds(t, d)

	req := httptest.NewRequest(http.MethodGet, "/admin/households/1", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusTemporaryRedirect; got != want {
		t.Errorf("status=%d, want=%d", got, want)
	}
}

func TestAdminHouseholdPageShowsGiverOfClaimedHousehold(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)
	seedClaimedAndUnclaimedHouseholds(t, d)

	req := httptest.NewRequest(http.MethodGet, "/admin/households/1", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status=%d, want=%d", got, want)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Maria") {
		t.Errorf("body is missing member name")
	}
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

func TestAdminHouseholdPageOfUnclaimedHousehold(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)
	seedClaimedAndUnclaimedHouseholds(t, d)

	req := httptest.NewRequest(http.MethodGet, "/admin/households/2", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status=%d, want=%d", got, want)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `href="/households/2"`) {
		t.Errorf("body is missing link to public page")
	}
	if strings.Contains(body, "Release claim") {
		t.Errorf("body offers to release a claim that doesn't exist")
	}
}

func TestAdminHouseholdPageOfMemberlessHousehold(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)
	seedClaimedAndUnclaimedHouseholds(t, d)

	req := httptest.NewRequest(http.MethodGet, "/admin/households/3", nil)
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

func TestAdminHouseholdPageNotFound(t *testing.T) {
	for _, tt := range []struct {
		explanation string
		path        string
	}{
		{
			"household that doesn't exist is not found",
			"/admin/households/99",
		},
		{
			"non-numeric household ID is not found",
			"/admin/households/abc",
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

func TestAdminReleaseClaimReturnsHouseholdToPublic(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)
	seedClaimedAndUnclaimedHouseholds(t, d)

	req := httptest.NewRequest(http.MethodDelete, "/admin/households/1/claim", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusNoContent; got != want {
		t.Fatalf("release status=%d, want=%d", got, want)
	}

	req = httptest.NewRequest(http.MethodGet, "/households/1", nil)
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
	seedClaimedAndUnclaimedHouseholds(t, d)

	req := httptest.NewRequest(http.MethodPost, "/admin/households/1/claim/token", nil)
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
			"/admin/households/1/claim",
			false,
			http.StatusUnauthorized,
		},
		{
			"issuing a new link requires login",
			http.MethodPost,
			"/admin/households/1/claim/token",
			false,
			http.StatusUnauthorized,
		},
		{
			"releasing an unclaimed household is not found",
			http.MethodDelete,
			"/admin/households/2/claim",
			true,
			http.StatusNotFound,
		},
		{
			"issuing a link for an unclaimed household is not found",
			http.MethodPost,
			"/admin/households/2/claim/token",
			true,
			http.StatusNotFound,
		},
	} {
		t.Run(fmt.Sprintf("%s [%s %s]", tt.explanation, tt.method, tt.path), func(t *testing.T) {
			server, d := newTestServer(t)
			seedClaimedAndUnclaimedHouseholds(t, d)

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

func TestAdminEditHouseholdPageIsPrefilled(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)

	if _, err := d.Exec(context.Background(), `
	INSERT INTO household (id, kind, created_time, last_modified_time) VALUES
		(1, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO household_member (
		id, household_id, first_name, last_initial, label, age, shirt_size,
		prefers_unscented, created_time, last_modified_time
	) VALUES (
		9, 1, 'Maria', 'G', 'Mom', NULL, '', 0,
		'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'
	), (
		10, 1, 'Sofia', '', 'Child, 7', 7, 'Youth M', 1,
		'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'
	);
	INSERT INTO wishlist_item (
		household_member_id, kind, name, created_time, last_modified_time
	) VALUES
		(10, 'need', 'Winter coat', '2026-01-01T00:00:00Z',
			'2026-01-01T00:00:00Z'),
		(10, 'need', 'Rain boots', '2026-01-01T00:00:00Z',
			'2026-01-01T00:00:00Z');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/households/1/edit", nil)
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
	if !strings.Contains(body, `data-submit-url="/admin/households/1"`) {
		t.Errorf("form doesn't submit to the household")
	}
	if !regexp.MustCompile(`value="family"\s+checked`).MatchString(body) {
		t.Errorf("form doesn't select the household's kind")
	}
	if !strings.Contains(body, `value="Maria"`) {
		t.Errorf("form is missing mom's first name")
	}
	if !strings.Contains(body, `value="G"`) {
		t.Errorf("form is missing mom's last initial")
	}
	if !strings.Contains(body, `value="Sofia"`) {
		t.Errorf("form is missing the child's first name")
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

func TestAdminUpdateHouseholdKeepsClaim(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)
	seedClaimedAndUnclaimedHouseholds(t, d)

	req := httptest.NewRequest(http.MethodPut, "/admin/households/1", strings.NewReader(
		`{"kind": "single", "members": [{"firstName": "Rosa", "lastInitial": "M", "needs": ["Winter coat"]}]}`))
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

func TestAdminDeleteUnclaimedHousehold(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)
	seedClaimedAndUnclaimedHouseholds(t, d)

	req := httptest.NewRequest(http.MethodDelete, "/admin/households/2", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusNoContent; got != want {
		t.Fatalf("status=%d, want=%d", got, want)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin/households/2", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if got, want := rec.Code, http.StatusNotFound; got != want {
		t.Errorf("admin page status=%d, want=%d", got, want)
	}
}

func TestAdminHouseholdPageDeleteControls(t *testing.T) {
	for _, tt := range []struct {
		explanation      string
		path             string
		isDeleteExpected bool
	}{
		{
			"claimed household asks to release before deleting",
			"/admin/households/1",
			false,
		},
		{
			"unclaimed household offers delete",
			"/admin/households/2",
			true,
		},
	} {
		t.Run(fmt.Sprintf("%s [%s]", tt.explanation, tt.path), func(t *testing.T) {
			server, d := newTestServer(t)
			cookie := loginAsAdmin(t, server, d)
			seedClaimedAndUnclaimedHouseholds(t, d)

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

func TestAdminHouseholdEditAndDeleteErrors(t *testing.T) {
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
			"/admin/households/2/edit",
			"",
			false,
			http.StatusTemporaryRedirect,
		},
		{
			"updating requires login",
			http.MethodPut,
			"/admin/households/2",
			`{"kind": "single", "members": [{"firstName": "Rosa", "lastInitial": "M"}]}`,
			false,
			http.StatusUnauthorized,
		},
		{
			"deleting requires login",
			http.MethodDelete,
			"/admin/households/2",
			"",
			false,
			http.StatusUnauthorized,
		},
		{
			"edit page for a household that doesn't exist is not found",
			http.MethodGet,
			"/admin/households/99/edit",
			"",
			true,
			http.StatusNotFound,
		},
		{
			"updating a household that doesn't exist is not found",
			http.MethodPut,
			"/admin/households/99",
			`{"kind": "single", "members": [{"firstName": "Rosa", "lastInitial": "M"}]}`,
			true,
			http.StatusNotFound,
		},
		{
			"updating with no members is rejected",
			http.MethodPut,
			"/admin/households/2",
			`{"kind": "single", "members": []}`,
			true,
			http.StatusBadRequest,
		},
		{
			"deleting a household that doesn't exist is not found",
			http.MethodDelete,
			"/admin/households/99",
			"",
			true,
			http.StatusNotFound,
		},
		{
			"deleting a claimed household conflicts",
			http.MethodDelete,
			"/admin/households/1",
			"",
			true,
			http.StatusConflict,
		},
	} {
		t.Run(fmt.Sprintf("%s [%s %s]", tt.explanation, tt.method, tt.path), func(t *testing.T) {
			server, d := newTestServer(t)
			seedClaimedAndUnclaimedHouseholds(t, d)

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

func TestHouseholdFormStartsWithFamilyAndMom(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)

	req := httptest.NewRequest(http.MethodGet, "/admin/households/new", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status=%d, want=%d", got, want)
	}
	body := rec.Body.String()
	if !regexp.MustCompile(`value="family"\s+checked`).MatchString(body) {
		t.Errorf("form doesn't start as a family")
	}
	if !regexp.MustCompile(`data-head-heading[^>]*>\s*Mom\s*</h2>`).MatchString(body) {
		t.Errorf("form doesn't start with a Mom section")
	}
	// The server builds every label, so the form has no label fields.
	if strings.Contains(body, `data-field="label"`) {
		t.Errorf("form has a label field")
	}
	// Mom's section has no age or gender fields, so only the blank section
	// that "Add child" clones has them. Only Mom has a last initial.
	if got, want := strings.Count(body, `data-field="age"`), 1; got != want {
		t.Errorf("age fields=%d, want=%d", got, want)
	}
	if got, want := strings.Count(body, `data-field="gender"`), 1; got != want {
		t.Errorf("gender fields=%d, want=%d", got, want)
	}
	if got, want := strings.Count(body, `data-field="lastInitial"`), 1; got != want {
		t.Errorf("last initial fields=%d, want=%d", got, want)
	}
	if got, want := strings.Count(body, `data-field="firstName"`), 2; got != want {
		t.Errorf("first name fields=%d, want=%d", got, want)
	}
}

func TestEditHouseholdFormTreatsFirstMemberAsMom(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)

	if _, err := d.Exec(context.Background(), `
	INSERT INTO household (id, kind, created_time, last_modified_time) VALUES
		(1, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO household_member
		(household_id, first_name, label, age, created_time, last_modified_time)
	VALUES
		(1, 'Maria', 'Mom', NULL, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(1, 'Sofia', 'Child, 7', 7, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/households/1/edit", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !regexp.MustCompile(`data-head-heading[^>]*>\s*Mom\s*</h2>`).MatchString(body) {
		t.Errorf("form doesn't render the first member as Mom")
	}
	// The child's section and the "Add child" template have age fields.
	if got, want := strings.Count(body, `data-field="age"`), 2; got != want {
		t.Errorf("age fields=%d, want=%d", got, want)
	}
	if !regexp.MustCompile(`data-member-heading[^>]*>\s*Child, 7\s*</h2>`).MatchString(body) {
		t.Errorf("form is missing the child's heading")
	}
}

func TestEditHouseholdFormOfSingleWoman(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)

	if _, err := d.Exec(context.Background(), `
	INSERT INTO household (id, kind, created_time, last_modified_time) VALUES
		(1, 'single', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO household_member (
		household_id, first_name, last_initial, label, created_time,
		last_modified_time
	) VALUES (
		1, 'Rosa', 'M', 'Woman', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'
	);
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/households/1/edit", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !regexp.MustCompile(`value="single"\s+checked`).MatchString(body) {
		t.Errorf("form doesn't select single woman")
	}
	if regexp.MustCompile(`value="family"\s+checked`).MatchString(body) {
		t.Errorf("form selects family")
	}
	if !regexp.MustCompile(`data-head-heading[^>]*>\s*Woman\s*</h2>`).MatchString(body) {
		t.Errorf("form doesn't render the first member as Woman")
	}
}

func TestAdminCreateHouseholdLabelsAgeZeroAsInfant(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)

	req := httptest.NewRequest(http.MethodPost, "/admin/households", strings.NewReader(
		`{"kind": "family", "members": [{"firstName": "Maria", "lastInitial": "G"}, {"firstName": "Leo", "age": 0}]}`))
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusCreated; got != want {
		t.Fatalf("status=%d, want=%d, body=%q", got, want, rec.Body.String())
	}

	var label string
	if err := d.QueryRow(context.Background(),
		`SELECT label FROM household_member ORDER BY id LIMIT 1 OFFSET 1`,
	).Scan(&label); err != nil {
		t.Fatalf("read child: %v", err)
	}
	if got, want := label, "Infant"; got != want {
		t.Errorf("label=%q, want=%q", got, want)
	}
}

func TestAdminIssueNewLinkEmailsGiver(t *testing.T) {
	for _, tt := range []struct {
		explanation   string
		mailer        *fakeMailer
		emailSentWant bool
	}{
		{
			"admin learns the new link was emailed",
			&fakeMailer{},
			true,
		},
		{
			"admin still gets the link when the email fails",
			&fakeMailer{err: errors.New("dummy send failure")},
			false,
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			server, d := newTestServerWithParams(t, testServerParams{mailer: tt.mailer})
			cookie := loginAsAdmin(t, server, d)
			seedClaimedAndUnclaimedHouseholds(t, d)

			req := httptest.NewRequest(http.MethodPost, "/admin/households/1/claim/token", nil)
			req.AddCookie(cookie)
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)

			if got, want := rec.Code, http.StatusOK; got != want {
				t.Fatalf("status=%d, want=%d", got, want)
			}
			var claim struct {
				URL       string `json:"url"`
				EmailSent bool   `json:"emailSent"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&claim); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if got, want := claim.EmailSent, tt.emailSentWant; got != want {
				t.Errorf("emailSent=%v, want=%v", got, want)
			}

			if got, want := len(tt.mailer.sent), 1; got != want {
				t.Fatalf("sent %d emails, want %d", got, want)
			}
			email := tt.mailer.sent[0]
			if got, want := email.to, "giver@example.com"; got != want {
				t.Errorf("to=%q, want=%q", got, want)
			}
			if got, want := email.subject, "Your new link for Blackbird Place Resident/Family 1"; got != want {
				t.Errorf("subject=%q, want=%q", got, want)
			}
			if !strings.Contains(email.body, testBaseURL+claim.URL) {
				t.Errorf("email body is missing the new link %q", testBaseURL+claim.URL)
			}
		})
	}
}
