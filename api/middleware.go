package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"publishing-backend/db"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

//  Ключи контекста

type contextKey string

const (
	ctxUserID    contextKey = "userID"
	ctxUserRole  contextKey = "userRole"
	ctxUserLogin contextKey = "userLogin"
)

//  Структура сессии в Redis

type SessionData struct {
	UserID    uint   `json:"user_id"`
	UserLogin string `json:"user_login"`
	UserRole  string `json:"user_role"`
}

const sessionTTL = 24 * time.Hour
const sessionCookieName = "session_id"
const authTokenCookieName = "auth_token"

func jwtSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return []byte("dev-jwt-secret")
	}
	return []byte(secret)
}

func parseJWTFromRequest(r *http.Request) (*jwt.Token, error) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	tokenString := ""

	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenString = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	}

	if tokenString == "" {
		cookie, err := r.Cookie(authTokenCookieName)
		if err != nil {
			if errors.Is(err, http.ErrNoCookie) {
				return nil, err
			}
			return nil, err
		}
		tokenString = strings.TrimSpace(cookie.Value)
	}

	if tokenString == "" {
		return nil, jwt.ErrTokenMalformed
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return jwtSecret(), nil
	})
	if err != nil {
		return nil, err
	}
	return token, nil
}

// CredentialsFromRequest — user id и роль из валидного JWT (Authorization: Bearer или кука auth_token).
func CredentialsFromRequest(r *http.Request) (userID uint, role string, ok bool) {
	token, err := parseJWTFromRequest(r) // парсим токен из запроса
	if err != nil || !token.Valid {
		return 0, "", false
	}
	claims, okClaims := token.Claims.(jwt.MapClaims)
	if !okClaims {
		return 0, "", false
	}
	userIDFloat, okID := claims["user_id"].(float64)
	if !okID || userIDFloat <= 0 {
		return 0, "", false
	}
	role, _ = claims["user_role"].(string)
	return uint(userIDFloat), role, true
}

func isPublicServicesGet(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	path := r.URL.Path
	return path == "/api/services" || strings.HasPrefix(path, "/api/services/")
}

func isPublicAuthEndpoint(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	switch r.URL.Path {
	case "/api/auth/login", "/api/auth/register", "/api/auth/logout":
		return true
	default:
		return false
	}
}

//  Создать сессию в Redis, вернуть session_id ─

func CreateSession(userID uint, login, role string) (string, error) {
	sessionID := uuid.New().String()
	data := SessionData{
		UserID:    userID,
		UserLogin: login,
		UserRole:  role,
	}
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	if err := db.Redis.Set(context.Background(),
		"session:"+sessionID, string(jsonBytes), sessionTTL).Err(); err != nil {
		return "", err
	}
	return sessionID, nil
}

//  Получить данные сессии из Redis

func GetSession(sessionID string) (*SessionData, error) {
	val, err := db.Redis.Get(context.Background(), "session:"+sessionID).Result()
	if err != nil {
		return nil, err
	}
	var data SessionData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

//  Удалить сессию из Redis

func DeleteSession(sessionID string) error {
	return db.Redis.Del(context.Background(), "session:"+sessionID).Err()
}

//  Middleware: читает куку, кладёт userID и role в контекст
// Если куки нет или сессия невалидна — продолжает без пользователя (гостевой доступ)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicServicesGet(r) || isPublicAuthEndpoint(r) {
			next.ServeHTTP(w, r)
			return
		}

		token, err := parseJWTFromRequest(r)
		if err != nil {
			if errors.Is(err, http.ErrNoCookie) {
				Logger.Debug(EventAuthTokenMissing,
					"event", EventAuthTokenMissing,
					"outcome", "denied",
					"method", r.Method, "path", r.URL.Path,
					"client_ip", clientIP(r))
				writeError(w, http.StatusUnauthorized, "требуется авторизация")
				return
			}
			Logger.Warn(EventAuthTokenInvalid,
				"event", EventAuthTokenInvalid,
				"outcome", "denied",
				"method", r.Method, "path", r.URL.Path,
				"client_ip", clientIP(r),
				"reason", "parse_error")
			writeError(w, http.StatusUnauthorized, "некорректный токен")
			return
		}
		if !token.Valid {
			Logger.Warn(EventAuthTokenInvalid,
				"event", EventAuthTokenInvalid,
				"outcome", "denied",
				"method", r.Method, "path", r.URL.Path,
				"client_ip", clientIP(r),
				"reason", "token_invalid")
			writeError(w, http.StatusUnauthorized, "некорректный токен")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			Logger.Warn(EventAuthTokenInvalid,
				"event", EventAuthTokenInvalid,
				"outcome", "denied",
				"method", r.Method, "path", r.URL.Path,
				"client_ip", clientIP(r),
				"reason", "claims_type")
			writeError(w, http.StatusUnauthorized, "некорректный токен")
			return
		}

		userIDFloat, ok := claims["user_id"].(float64)
		if !ok || userIDFloat <= 0 {
			Logger.Warn(EventAuthTokenInvalid,
				"event", EventAuthTokenInvalid,
				"outcome", "denied",
				"method", r.Method, "path", r.URL.Path,
				"client_ip", clientIP(r),
				"reason", "missing_user_id")
			writeError(w, http.StatusUnauthorized, "некорректный токен")
			return
		}
		userRole, _ := claims["user_role"].(string)
		userLogin, _ := claims["user_login"].(string)

		ctx := context.WithValue(r.Context(), ctxUserID, uint(userIDFloat))
		ctx = context.WithValue(ctx, ctxUserRole, userRole)
		ctx = context.WithValue(ctx, ctxUserLogin, userLogin)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

//  Требовать авторизацию (401 если нет)

func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Context().Value(ctxUserID) == nil {
			Logger.Debug(EventAuthTokenMissing,
				"event", EventAuthTokenMissing,
				"outcome", "denied",
				"method", r.Method, "path", r.URL.Path,
				"client_ip", clientIP(r),
				"stage", "require_auth")
			writeError(w, http.StatusUnauthorized, "требуется авторизация")
			return
		}
		next.ServeHTTP(w, r)
	})
}

//  Требовать роль модератора (403 если не модератор)

func RequireModerator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, _ := r.Context().Value(ctxUserRole).(string)
		if role != "moderator" {
			uid, _ := GetUserIDFromCtx(r)
			Logger.Warn(EventAuthzForbidden,
				"event", EventAuthzForbidden,
				"outcome", "denied",
				"method", r.Method, "path", r.URL.Path,
				"user_id", uid,
				"user_role", role,
				"required_role", "moderator",
				"client_ip", clientIP(r))
			writeError(w, http.StatusForbidden, "доступ только для модератора")
			return
		}
		next.ServeHTTP(w, r)
	})
}

//  Хелперы для получения данных из контекста ─

func GetUserIDFromCtx(r *http.Request) (uint, bool) {
	val, ok := r.Context().Value(ctxUserID).(uint)
	return val, ok
}

func GetUserRoleFromCtx(r *http.Request) string {
	role, _ := r.Context().Value(ctxUserRole).(string)
	return role
}
