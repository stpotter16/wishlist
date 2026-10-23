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
	mux.HandleFunc("GET /{$}", familiesGet(store))
	mux.HandleFunc("GET /families/{id}", familyGet(store))
	mux.HandleFunc("GET /claims/{token}", claimGet(store))
	mux.HandleFunc("GET /login", loginGet())

	// Views that need authentication
	viewAuthRequired := middleware.NewViewAuthenticationRequiredMiddleware(sessionManager)
	mux.Handle("GET /admin", viewAuthRequired(adminGet(store)))
	mux.Handle("GET /admin/families/new", viewAuthRequired(adminNewFamilyGet()))
	mux.Handle("GET /admin/families/{id}", viewAuthRequired(adminFamilyGet(store)))
	mux.Handle("GET /admin/families/{id}/edit", viewAuthRequired(adminEditFamilyGet(store)))

	// Public API endpoints
	mux.HandleFunc("POST /families/{id}/claim", familyClaimPost(store))

	// Auth
	mux.HandleFunc("POST /login", loginPost(authenticator, sessionManager))

	// Session authenticated API endpoints
	apiAuthRequired := middleware.NewApiAuthenticationRequiredMiddleware(sessionManager)
	mux.Handle("POST /logout", apiAuthRequired(logoutPost(sessionManager)))
	mux.Handle("POST /admin/families", apiAuthRequired(adminFamiliesPost(store)))
	mux.Handle("PUT /admin/families/{id}", apiAuthRequired(adminFamilyPut(store)))
	mux.Handle("DELETE /admin/families/{id}", apiAuthRequired(adminFamilyDelete(store)))
	mux.Handle("DELETE /admin/families/{id}/claim", apiAuthRequired(adminClaimDelete(store)))
	mux.Handle("POST /admin/families/{id}/claim/token", apiAuthRequired(adminClaimTokenPost(store)))
}
