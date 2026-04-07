package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"publishing-backend/db"

	"github.com/google/uuid"
)

// ─── Ключи контекста ──────────────────────────────────────────────────────────

type contextKey string

const (
	ctxUserID    contextKey = "userID"
	ctxUserRole  contextKey = "userRole"
	ctxUserLogin contextKey = "userLogin"
)

// ─── Структура сессии в Redis ─────────────────────────────────────────────────

type SessionData struct {
	UserID    uint   `json:"user_id"`
	UserLogin string `json:"user_login"`
	UserRole  string `json:"user_role"`
}

const sessionTTL = 24 * time.Hour
const sessionCookieName = "session_id"

// ─── Создать сессию в Redis, вернуть session_id ───────────────────────────────

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

// ─── Получить данные сессии из Redis ─────────────────────────────────────────

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

// ─── Удалить сессию из Redis ──────────────────────────────────────────────────

func DeleteSession(sessionID string) error {
	return db.Redis.Del(context.Background(), "session:"+sessionID).Err()
}

// ─── Middleware: читает куку, кладёт userID и role в контекст ─────────────────
// Если куки нет или сессия невалидна — продолжает без пользователя (гостевой доступ)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		session, err := GetSession(cookie.Value)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		ctx := context.WithValue(r.Context(), ctxUserID, session.UserID)
		ctx = context.WithValue(ctx, ctxUserRole, session.UserRole)
		ctx = context.WithValue(ctx, ctxUserLogin, session.UserLogin)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ─── Требовать авторизацию (401 если нет) ────────────────────────────────────

func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Context().Value(ctxUserID) == nil {
			writeError(w, http.StatusUnauthorized, "требуется авторизация")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ─── Требовать роль модератора (403 если не модератор) ───────────────────────

func RequireModerator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, _ := r.Context().Value(ctxUserRole).(string)
		if role != "moderator" {
			writeError(w, http.StatusForbidden, "доступ только для модератора")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ─── Хелперы для получения данных из контекста ───────────────────────────────

func GetUserIDFromCtx(r *http.Request) (uint, bool) {
	val, ok := r.Context().Value(ctxUserID).(uint)
	return val, ok
}

func GetUserRoleFromCtx(r *http.Request) string {
	role, _ := r.Context().Value(ctxUserRole).(string)
	return role
}
