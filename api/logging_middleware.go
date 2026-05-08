package api

import (
	"net/http"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		lrw := &responseWriter{ResponseWriter: w, statusCode: 0}
		next.ServeHTTP(lrw, r)

		status := lrw.statusCode
		if status == 0 {
			status = http.StatusOK
		}
		durationMs := time.Since(start).Milliseconds()

		outcome := "success"
		attrs := []any{
			"event", "http.request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", durationMs,
			"client_ip", clientIP(r),
		}
		if userID, ok := GetUserIDFromCtx(r); ok && userID != 0 {
			attrs = append(attrs, "user_id", userID)
		}
		if role := GetUserRoleFromCtx(r); role != "" {
			attrs = append(attrs, "user_role", role)
		}

		switch {
		case status >= 500:
			outcome = "error"
			attrs = append(attrs, "outcome", outcome)
			Logger.Error("http.request", attrs...)
		case status >= 400:
			outcome = "failure"
			attrs = append(attrs, "outcome", outcome)
			Logger.Warn("http.request", attrs...)
		default:
			attrs = append(attrs, "outcome", outcome)
			Logger.Debug("http.request", attrs...)
		}
	})
}
