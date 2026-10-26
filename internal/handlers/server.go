package handlers

import (
	"net/http"
	"time"

	"github.com/stpotter16/wishlist/internal/handlers/authentication"
	"github.com/stpotter16/wishlist/internal/handlers/middleware"
	"github.com/stpotter16/wishlist/internal/handlers/sessions"
	"github.com/stpotter16/wishlist/internal/store"
)

func NewServer(
	store store.Store,
	sessionManager sessions.SessionManger,
	authenticator authentication.Authenticator,
	now func() time.Time,
) http.Handler {
	mux := http.NewServeMux()
	addRoutes(mux, store, sessionManager, authenticator, now)
	// CrossOriginProtection rejects cross-origin, state-changing browser
	// requests, which protects every POST, PUT, and DELETE from CSRF. It relies
	// on the browser sending Sec-Fetch-Site or Origin, and lets requests with
	// neither header through. AGENTS.md documents these assumptions under
	// "Browser support".
	handler := http.NewCrossOriginProtection().Handler(mux)
	handler = middleware.CspMiddleware(handler)
	handler = middleware.LoggingWrapper(handler)
	return handler
}
