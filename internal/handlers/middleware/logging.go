package middleware

import (
	"log"
	"net/http"
	"strings"
	"time"
)

// claimPathPrefix starts every giver's private link. The rest of the path is
// the giver's access token, which must stay out of the logs.
const claimPathPrefix = "/claims/"

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func LoggingWrapper(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{
			ResponseWriter: w,
			statusCode:     200,
		}
		next.ServeHTTP(recorder, r)
		log.Printf("%s %s %s %d %s", r.Method, r.Host, redactPath(r.URL.Path), recorder.statusCode, time.Since(start))
	})
}

func redactPath(path string) string {
	if strings.HasPrefix(path, claimPathPrefix) {
		return claimPathPrefix + "[redacted]"
	}
	return path
}
