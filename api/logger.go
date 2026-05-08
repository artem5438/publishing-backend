package api

import (
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
)

// Logger — глобальный структурированный логгер (инициализируется InitLogger).
var Logger *slog.Logger

// Имена доменных событий для единого поиска в логах/Loki/Grafana.
const (
	EventAuthLoginSuccess = "auth.login.success"
	EventAuthLoginFailed  = "auth.login.failed"
	EventAuthTokenMissing = "auth.token.missing"
	EventAuthTokenInvalid = "auth.token.invalid"
	EventAuthzForbidden   = "authz.forbidden"
	EventUserCreated      = "user.created"
)

// InitLogger настраивает JSON-лог в stdout; уровень из LOG_LEVEL (debug|info|warn|error), по умолчанию info.
func InitLogger() {
	level := parseLogLevel(os.Getenv("LOG_LEVEL"))
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	Logger = slog.New(h)
	slog.SetDefault(Logger)
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "info", "":
		return slog.LevelInfo
	default:
		return slog.LevelInfo
	}
}

// clientIP извлекает реальный IP клиента с учётом nginx-прокси.
// Порядок: X-Forwarded-For (первый адрес) -> X-Real-IP -> RemoteAddr.
func clientIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		if comma := strings.IndexByte(xff, ','); comma >= 0 {
			return strings.TrimSpace(xff[:comma])
		}
		return xff
	}
	if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
		return xrip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
