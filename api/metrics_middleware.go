package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// MetricsMiddleware записывает http_requests_total и http_request_duration_seconds.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		mrw := &responseWriter{ResponseWriter: w, statusCode: 0}
		next.ServeHTTP(mrw, r)

		status := mrw.statusCode
		if status == 0 {
			status = http.StatusOK
		}

		path := "unknown"
		if rc := chi.RouteContext(r.Context()); rc != nil {
			if p := rc.RoutePattern(); p != "" {
				path = p
			}
		}

		statusStr := strconv.Itoa(status)
		HTTPRequestsTotal.WithLabelValues(r.Method, path, statusStr).Inc()
		HTTPRequestDuration.WithLabelValues(r.Method, path).Observe(time.Since(start).Seconds())
	})
}
