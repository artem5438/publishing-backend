package api

import (
	"net/http"
	"time"
)

// responseWriter перехватывает код статуса ответа.
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

// LoggingMiddleware логирует method, path, status, duration_ms; уровень по статусу (5xx Error, 4xx Warn, иначе Info).
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &responseWriter{ResponseWriter: w, statusCode: 0}
		next.ServeHTTP(lrw, r)

		status := lrw.statusCode
		if status == 0 {
			status = http.StatusOK
		}
		durationMs := time.Since(start).Milliseconds()

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", durationMs,
		}

		switch {
		case status >= 500:
			Logger.Error("request", attrs...)
		case status >= 400:
			Logger.Warn("request", attrs...)
		default:
			Logger.Info("request", attrs...)
		}
	})
}
