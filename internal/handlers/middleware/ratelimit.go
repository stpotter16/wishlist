package middleware

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// flyClientIPHeader carries the client's address. Fly's proxy sets it, so
// RemoteAddr is always the proxy itself in production.
const flyClientIPHeader = "Fly-Client-IP"

type RateLimitParams struct {
	// Limit is the number of requests each client IP may make per window.
	Limit  int
	Window time.Duration
	Now    func() time.Time
}

type rateLimitWindow struct {
	start time.Time
	count int
}

// NewRateLimitMiddleware limits each client IP to a fixed number of requests
// per window. It keeps counts in memory, which works because the app runs on
// exactly one machine.
func NewRateLimitMiddleware(params RateLimitParams) func(http.Handler) http.Handler {
	var mu sync.Mutex
	windows := map[string]rateLimitWindow{}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			now := params.Now()
			ip := clientIP(r)

			mu.Lock()
			// Drop expired windows so the map can't grow without bound.
			for key, window := range windows {
				if now.Sub(window.start) >= params.Window {
					delete(windows, key)
				}
			}
			window, ok := windows[ip]
			if !ok {
				window = rateLimitWindow{start: now}
			}
			allowed := window.count < params.Limit
			if allowed {
				window.count++
				windows[ip] = window
			}
			mu.Unlock()

			if !allowed {
				retryAfter := window.start.Add(params.Window).Sub(now)
				w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
				http.Error(w, "Too many attempts from your network. Please try again later.", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP reads the address Fly's proxy forwards. Requests that don't come
// through the proxy, such as local development, use the connection's address.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get(flyClientIPHeader); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
