package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stpotter16/wishlist/internal/auth"
	"github.com/stpotter16/wishlist/internal/handlers"
	"github.com/stpotter16/wishlist/internal/handlers/authentication"
	"github.com/stpotter16/wishlist/internal/handlers/sessions"
	"github.com/stpotter16/wishlist/internal/store/db"
	"github.com/stpotter16/wishlist/internal/store/sqlite"
)

// newTestServer returns the server and its database, so tests can seed rows
// directly.
func newTestServer(t *testing.T) (http.Handler, db.DB) {
	t.Helper()

	d, err := db.New(t.TempDir())
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}

	store, err := sqlite.New(d)
	if err != nil {
		t.Fatalf("sqlite.New: %v", err)
	}

	sm, err := sessions.New(d, func(key string) string {
		if key == sessions.SESSION_ENV_KEY {
			return "test-hmac-secret-key"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("sessions.New: %v", err)
	}

	return handlers.NewServer(store, sm, authentication.New(store)), d
}

// loginAsAdmin seeds an admin and logs in through POST /login, returning the
// session cookie for authenticated requests.
func loginAsAdmin(t *testing.T, server http.Handler, d db.DB) *http.Cookie {
	t.Helper()

	passwordHash, err := auth.HashPassword("dummy-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if _, err := d.Exec(context.Background(), `
	INSERT INTO admin (username, password_hash, created_time, last_modified_time)
	VALUES ('admin', ?, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
	`, passwordHash); err != nil {
		t.Fatalf("seed admin: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(
		`{"username": "admin", "password": "dummy-password"}`))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("login status=%d, want=%d", got, want)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("login set %d cookies, want 1", len(cookies))
	}
	return cookies[0]
}

func TestLoginPageRenders(t *testing.T) {
	server, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Errorf("status=%d, want=%d", got, want)
	}
}

func TestAdminRequiresLogin(t *testing.T) {
	server, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusTemporaryRedirect; got != want {
		t.Fatalf("status=%d, want=%d", got, want)
	}
	if got, want := rec.Header().Get("Location"), "/login"; got != want {
		t.Errorf("Location=%q, want=%q", got, want)
	}
}

func TestCrossOriginRequestsAreRejected(t *testing.T) {
	for _, tt := range []struct {
		explanation string
		header      string
		value       string
		statusWant  int
	}{
		{
			"browser-flagged cross-site request is rejected",
			"Sec-Fetch-Site",
			"cross-site",
			http.StatusForbidden,
		},
		{
			"request from a foreign origin is rejected",
			"Origin",
			"https://evil.example",
			http.StatusForbidden,
		},
		{
			"browser-flagged same-origin request is allowed",
			"Sec-Fetch-Site",
			"same-origin",
			http.StatusCreated,
		},
	} {
		t.Run(fmt.Sprintf("%s [%s: %s]", tt.explanation, tt.header, tt.value), func(t *testing.T) {
			server, d := newTestServer(t)
			cookie := loginAsAdmin(t, server, d)

			req := httptest.NewRequest(http.MethodPost, "/admin/households", strings.NewReader(
				`{"kind": "single", "members": [{"firstName": "Rosa", "lastInitial": "M"}]}`))
			req.AddCookie(cookie)
			req.Header.Set(tt.header, tt.value)
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)

			if got, want := rec.Code, tt.statusWant; got != want {
				t.Errorf("status=%d, want=%d", got, want)
			}
		})
	}
}

func TestLogoutEndsSession(t *testing.T) {
	server, d := newTestServer(t)
	cookie := loginAsAdmin(t, server, d)

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusNoContent; got != want {
		t.Fatalf("logout status=%d, want=%d", got, want)
	}
	cleared := rec.Result().Cookies()
	if len(cleared) != 1 || cleared[0].MaxAge >= 0 {
		t.Errorf("logout cookies=%+v, want one expired session cookie", cleared)
	}

	// The old cookie no longer grants access, even if the browser kept it.
	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusTemporaryRedirect; got != want {
		t.Errorf("admin status after logout=%d, want=%d", got, want)
	}
}

func TestLogoutRequiresLogin(t *testing.T) {
	server, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusUnauthorized; got != want {
		t.Errorf("status=%d, want=%d", got, want)
	}
}

func TestFaviconServes(t *testing.T) {
	server, _ := newTestServer(t)

	for _, tt := range []struct {
		description  string
		path         string
		wantStatus   int
		wantLocation string
	}{
		{
			description: "serves the SVG favicon",
			path:        "/static/assets/favicon.svg",
			wantStatus:  http.StatusOK,
		},
		{
			description:  "redirects /favicon.ico to the SVG favicon",
			path:         "/favicon.ico",
			wantStatus:   http.StatusMovedPermanently,
			wantLocation: "/static/assets/favicon.svg",
		},
	} {
		t.Run(tt.description, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)

			if got, want := rec.Code, tt.wantStatus; got != want {
				t.Errorf("status=%d, want=%d", got, want)
			}
			if got, want := rec.Header().Get("Location"), tt.wantLocation; got != want {
				t.Errorf("Location=%q, want=%q", got, want)
			}
		})
	}
}
