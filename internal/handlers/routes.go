package handlers

import (
	"net/http"
	"time"

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
	turnstileSiteKey string,
	humanVerifier HumanVerifier,
	mailer Mailer,
	baseURL string,
	now func() time.Time,
) {
	// Static
	mux.Handle("GET /static/", http.StripPrefix("/static/", serveStaticFiles()))
	mux.Handle("GET /favicon.ico", http.RedirectHandler("/static/assets/favicon.svg", http.StatusMovedPermanently))

	// Views
	mux.HandleFunc("GET /{$}", householdsGet(store))
	mux.HandleFunc("GET /households/{id}", householdGet(store, turnstileSiteKey))
	mux.HandleFunc("GET /claims/{token}", claimGet(store))
	mux.HandleFunc("GET /login", loginGet())

	// Views that need authentication
	viewAuthRequired := middleware.NewViewAuthenticationRequiredMiddleware(sessionManager)
	mux.Handle("GET /admin", viewAuthRequired(adminGet(store)))
	mux.Handle("GET /admin/households/new", viewAuthRequired(adminNewHouseholdGet()))
	mux.Handle("GET /admin/households/{id}", viewAuthRequired(adminHouseholdGet(store)))
	mux.Handle("GET /admin/households/{id}/edit", viewAuthRequired(adminEditHouseholdGet(store)))

	// Public API endpoints
	// Claiming needs no login, so limit how many claims one network can
	// attempt. Shared networks, like a church's Wi-Fi, still need room for
	// several givers.
	claimRateLimit := middleware.NewRateLimitMiddleware(middleware.RateLimitParams{
		Limit:  10,
		Window: time.Hour,
		Now:    now,
	})
	mux.Handle("POST /households/{id}/claim", claimRateLimit(householdClaimPost(store, humanVerifier, mailer, baseURL)))

	// Auth
	mux.HandleFunc("POST /login", loginPost(authenticator, sessionManager))

	// Session authenticated API endpoints
	apiAuthRequired := middleware.NewApiAuthenticationRequiredMiddleware(sessionManager)
	mux.Handle("POST /logout", apiAuthRequired(logoutPost(sessionManager)))
	mux.Handle("POST /admin/households", apiAuthRequired(adminHouseholdsPost(store)))
	mux.Handle("PUT /admin/households/{id}", apiAuthRequired(adminHouseholdPut(store)))
	mux.Handle("DELETE /admin/households/{id}", apiAuthRequired(adminHouseholdDelete(store)))
	mux.Handle("DELETE /admin/households/{id}/claim", apiAuthRequired(adminClaimDelete(store)))
	mux.Handle("POST /admin/households/{id}/claim/token", apiAuthRequired(adminClaimTokenPost(store, mailer, baseURL)))
}
