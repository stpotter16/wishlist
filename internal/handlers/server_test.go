package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stpotter16/wishlist/internal/handlers"
	"github.com/stpotter16/wishlist/internal/handlers/authentication"
	"github.com/stpotter16/wishlist/internal/handlers/sessions"
	"github.com/stpotter16/wishlist/internal/store/db"
	"github.com/stpotter16/wishlist/internal/store/sqlite"
)

func newTestServer(t *testing.T) http.Handler {
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

	return handlers.NewServer(store, sm, authentication.New(store))
}

func TestLoginPageRenders(t *testing.T) {
	server := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Errorf("status=%d, want=%d", got, want)
	}
}
