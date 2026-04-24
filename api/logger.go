package api

import (
	"log/slog"
	"os"
	"strings"
)

// Logger — глобальный структурированный логгер (инициализируется InitLogger).
var Logger *slog.Logger

// InitLogger настраивает JSON-лог в stdout; уровень из LOG_LEVEL (debug|info|warn|error), по умолчанию info.
func InitLogger() {
	level := parseLogLevel(os.Getenv("LOG_LEVEL"))
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	Logger = slog.New(h)
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
