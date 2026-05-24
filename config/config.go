package config

import (
	"os"
	"strings"
)

func getEnv(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}

func MinioEndpoint() string {
	return getEnv("MINIO_ENDPOINT", "localhost:9000")
}

func MinioAccessKey() string {
	return getEnv("MINIO_ACCESS_KEY", "minioadmin")
}

func MinioSecretKey() string {
	return getEnv("MINIO_SECRET_KEY", "minioadmin123")
}

func MinioBucket() string {
	return getEnv("MINIO_BUCKET", "publishing-media")
}

func MinioUseSSL() bool {
	return strings.EqualFold(getEnv("MINIO_USE_SSL", "false"), "true")
}

func MinioPublicURL() string {
	return strings.TrimRight(getEnv("MINIO_PUBLIC_URL", "http://localhost:9000/publishing-media"), "/")
}

func JWTSecret() string {
	return getEnv("JWT_SECRET", "dev-jwt-secret")
}

func CORSOrigin() string {
	return getEnv("CORS_ORIGIN", "http://localhost:5173")
}

func CORSOrigins() []string {
	raw := getEnv("CORS_ORIGIN", "http://localhost:5173")
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		origins = append(origins, trimmed)
	}
	if len(origins) == 0 {
		return []string{"http://localhost:5173"}
	}
	return origins
}

func HTTPAddr() string {
	return getEnv("HTTP_ADDR", ":8080")
}
