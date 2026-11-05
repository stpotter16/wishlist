package middleware

import (
	"net/http"

	"github.com/stpotter16/wishlist/internal/handlers/sessions"
)

type DriveOpenParams struct {
	IsOpen         bool
	SessionManager sessions.SessionManger
	// Closed serves requests from non-admins while the drive is closed.
	Closed http.Handler
}

// NewDriveOpenMiddleware hides public pages until the drive opens. Admins
// still get through, so they can check the public pages before launch.
func NewDriveOpenMiddleware(params DriveOpenParams) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if params.IsOpen {
				next.ServeHTTP(w, r)
				return
			}
			ctx, err := params.SessionManager.PopulateSessionContext(r)
			if err != nil {
				params.Closed.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
