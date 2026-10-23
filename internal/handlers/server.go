package handlers

import (
	"net/http"

	"github.com/stpotter16/wishlist/internal/handlers/authentication"
	"github.com/stpotter16/wishlist/internal/handlers/middleware"
	"github.com/stpotter16/wishlist/internal/handlers/sessions"
	"github.com/stpotter16/wishlist/internal/store"
)

func NewServer(
	store store.Store,
	sessionManager sessions.SessionManger,
	authenticator authentication.Authenticator,
) http.Handler {
	mux := http.NewServeMux()
	addRoutes(mux, store, sessionManager, authenticator)
	// CrossOriginProtection rejects cross-origin, state-changing browser
	// requests, which protects every POST, PUT, and DELETE from CSRF.
	handler := http.NewCrossOriginProtection().Handler(mux)
	handler = middleware.CspMiddleware(handler)
	handler = middleware.LoggingWrapper(handler)
	return handler
}
