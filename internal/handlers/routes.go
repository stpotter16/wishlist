package handlers

import (
	"net/http"

	"github.com/stpotter16/wishlist/internal/handlers/authentication"
	"github.com/stpotter16/wishlist/internal/handlers/middleware"
	"github.com/stpotter16/wishlist/internal/handlers/sessions"
	"github.com/stpotter16/wishlist/internal/store"
)

func addRoutes(
	mux *http.ServeMux,
	store store.Store,
	sessionManager sessions.SessionManger,
	authenticator authentication.Authenticator,
) {
	// Static
	mux.Handle("GET /static/", http.StripPrefix("/static/", serveStaticFiles()))

	// Views
	mux.HandleFunc("GET /{$}", householdsGet(store))
	mux.HandleFunc("GET /households/{id}", householdGet(store))
	mux.HandleFunc("GET /claims/{token}", claimGet(store))
	mux.HandleFunc("GET /login", loginGet())

	// Views that need authentication
	viewAuthRequired := middleware.NewViewAuthenticationRequiredMiddleware(sessionManager)
	mux.Handle("GET /admin", viewAuthRequired(adminGet(store)))
	mux.Handle("GET /admin/households/new", viewAuthRequired(adminNewHouseholdGet()))
	mux.Handle("GET /admin/households/{id}", viewAuthRequired(adminHouseholdGet(store)))
	mux.Handle("GET /admin/households/{id}/edit", viewAuthRequired(adminEditHouseholdGet(store)))

	// Public API endpoints
	mux.HandleFunc("POST /households/{id}/claim", householdClaimPost(store))

	// Auth
	mux.HandleFunc("POST /login", loginPost(authenticator, sessionManager))

	// Session authenticated API endpoints
	apiAuthRequired := middleware.NewApiAuthenticationRequiredMiddleware(sessionManager)
	mux.Handle("POST /logout", apiAuthRequired(logoutPost(sessionManager)))
	mux.Handle("POST /admin/households", apiAuthRequired(adminHouseholdsPost(store)))
	mux.Handle("PUT /admin/households/{id}", apiAuthRequired(adminHouseholdPut(store)))
	mux.Handle("DELETE /admin/households/{id}", apiAuthRequired(adminHouseholdDelete(store)))
	mux.Handle("DELETE /admin/households/{id}/claim", apiAuthRequired(adminClaimDelete(store)))
	mux.Handle("POST /admin/households/{id}/claim/token", apiAuthRequired(adminClaimTokenPost(store)))
}
