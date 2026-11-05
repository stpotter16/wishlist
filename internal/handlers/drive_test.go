package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stpotter16/wishlist/internal/store/db"
)

func TestClosedDriveHidesPublicPagesFromGivers(t *testing.T) {
	for _, tt := range []struct {
		explanation string
		method      string
		path        string
		body        string
		statusWant  int
		bodyWant    string
	}{
		{
			"listing shows that the drive opens soon",
			http.MethodGet,
			"/",
			"",
			http.StatusOK,
			"The gift drive opens soon",
		},
		{
			"household page shows that the drive opens soon",
			http.MethodGet,
			"/households/1",
			"",
			http.StatusOK,
			"The gift drive opens soon",
		},
		{
			"claim is rejected",
			http.MethodPost,
			"/households/1/claim",
			`{"name": "Giver", "email": "giver@example.com", "phone": "555-0100"}`,
			http.StatusForbidden,
			"The gift drive isn't open yet.",
		},
	} {
		t.Run(fmt.Sprintf("%s [%s %s]", tt.explanation, tt.method, tt.path), func(t *testing.T) {
			server, d := newTestServerWithParams(t, testServerParams{driveClosed: true})
			seedOneHousehold(t, d)

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)

			if got, want := rec.Code, tt.statusWant; got != want {
				t.Fatalf("status=%d, want=%d", got, want)
			}
			body := rec.Body.String()
			if !strings.Contains(body, tt.bodyWant) {
				t.Errorf("body is missing %q", tt.bodyWant)
			}
			if strings.Contains(body, "Maria") {
				t.Errorf("body contains recipient name")
			}
		})
	}
}

func TestClosedDriveShowsPublicPagesToAdmins(t *testing.T) {
	for _, tt := range []struct {
		explanation string
		method      string
		path        string
		body        string
		statusWant  int
	}{
		{
			"listing renders",
			http.MethodGet,
			"/",
			"",
			http.StatusOK,
		},
		{
			"household page renders",
			http.MethodGet,
			"/households/1",
			"",
			http.StatusOK,
		},
		{
			"claim goes through",
			http.MethodPost,
			"/households/1/claim",
			`{"name": "Giver", "email": "giver@example.com", "phone": "555-0100"}`,
			http.StatusCreated,
		},
	} {
		t.Run(fmt.Sprintf("%s [%s %s]", tt.explanation, tt.method, tt.path), func(t *testing.T) {
			server, d := newTestServerWithParams(t, testServerParams{driveClosed: true})
			seedOneHousehold(t, d)
			cookie := loginAsAdmin(t, server, d)

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.AddCookie(cookie)
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)

			if got, want := rec.Code, tt.statusWant; got != want {
				t.Fatalf("status=%d, want=%d, body=%q", got, want, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "The gift drive opens soon") {
				t.Errorf("body shows the drive-closed page")
			}
		})
	}
}

// seedOneHousehold adds unclaimed household 1, headed by Maria G.
func seedOneHousehold(t *testing.T, d db.DB) {
	t.Helper()
	if _, err := d.Exec(context.Background(), `
	INSERT INTO household (id, kind, created_time, last_modified_time) VALUES
		(1, 'single', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO household_member
		(household_id, first_name, last_initial, label, created_time, last_modified_time)
	VALUES
		(1, 'Maria', 'G', 'Woman', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}
}
