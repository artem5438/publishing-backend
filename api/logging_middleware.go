package api

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
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
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}
		ctx := context.WithValue(r.Context(), ctxRequestID, reqID)
		w.Header().Set("X-Request-ID", reqID)

		rWithCtx := r.WithContext(ctx)

		start := time.Now()
		lrw := &responseWriter{ResponseWriter: w, statusCode: 0}
		next.ServeHTTP(lrw, rWithCtx)

		status := lrw.statusCode
		if status == 0 {
			status = http.StatusOK
		}
		durationMs := time.Since(start).Milliseconds()

		attrs := []any{
			"request_id", reqID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", durationMs,
			"client_ip", clientIP(r),
		}
		if userID, ok := GetUserIDFromCtx(rWithCtx); ok && userID != 0 {
			attrs = append(attrs, "user_id", userID)
		}
		if role := GetUserRoleFromCtx(rWithCtx); role != "" {
			attrs = append(attrs, "user_role", role)
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
