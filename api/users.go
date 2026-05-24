package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"publishing-backend/db"
	"publishing-backend/models"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Register godoc
// @Summary     Регистрация
// @Description Создаёт нового пользователя с хэшированным паролем
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       body body object true "login, password, name, role"
// @Success     201  {object} map[string]interface{}
// @Failure     400  {object} map[string]string
// @Failure     409  {object} map[string]string
// @Router      /auth/register [post]
func Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Login    string `json:"login"`
		Password string `json:"password"`
		Name     string `json:"name"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	if body.Login == "" || body.Password == "" || body.Name == "" {
		writeError(w, http.StatusBadRequest, "login, password и name обязательны")
		return
	}

	var existing models.User
	if db.DB.Where("login = ?", body.Login).First(&existing).Error == nil {
		writeError(w, http.StatusConflict, "пользователь с таким логином уже существует")
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost) // хэшируем пароль
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка хэширования пароля")
		return
	}

	user := models.User{
		Login:    body.Login,
		Password: string(hashed),
		Name:     body.Name,
		Role:     models.RoleCreator,
	}
	if err := db.DB.Create(&user).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка создания пользователя")
		return
	}

	Logger.Info(EventUserCreated,
		"event", EventUserCreated,
		"outcome", "success",
		"method", r.Method, "path", r.URL.Path,
		"user_id", user.ID,
		"login", user.Login,
		"user_role", string(user.Role),
		"client_ip", clientIP(r))

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":    user.ID,
		"login": user.Login,
		"name":  user.Name,
		"role":  user.Role,
	})
}

// Login godoc
// @Summary     Аутентификация
// @Description Проверяет логин/пароль, создаёт сессию в Redis, устанавливает куку session_id
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       body body object true "login, password"
// @Success     200  {object} map[string]interface{}
// @Failure     400  {object} map[string]string
// @Failure     401  {object} map[string]string
// @Router      /auth/login [post]
func Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	if body.Login == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "login и password обязательны")
		return
	}

	var user models.User
	if db.DB.Where("login = ?", body.Login).First(&user).Error != nil {
		Logger.Info(EventAuthLoginFailed,
			"event", EventAuthLoginFailed,
			"outcome", "failure",
			"method", r.Method, "path", r.URL.Path,
			"login", body.Login,
			"reason", "user_not_found",
			"client_ip", clientIP(r))
		writeError(w, http.StatusUnauthorized, "неверный логин или пароль")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(body.Password)); err != nil {
		Logger.Info(EventAuthLoginFailed,
			"event", EventAuthLoginFailed,
			"outcome", "failure",
			"method", r.Method, "path", r.URL.Path,
			"login", body.Login,
			"user_id", user.ID,
			"reason", "invalid_password",
			"client_ip", clientIP(r))
		writeError(w, http.StatusUnauthorized, "неверный логин или пароль")
		return
	}

	sessionID, err := CreateSession(user.ID, user.Login, string(user.Role))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка создания сессии")
		return
	}

	tokenString, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{ // создаем токен
		"user_id":    user.ID,
		"user_login": user.Login,
		"user_role":  string(user.Role),
		"exp":        time.Now().Add(24 * time.Hour).Unix(),
	}).SignedString(jwtSecret())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка создания токена")
		return
	}
	// Устанавливаем куки сессии и токена
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(24 * time.Hour),
	})
	http.SetCookie(w, &http.Cookie{
		Name:     authTokenCookieName,
		Value:    tokenString,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(24 * time.Hour),
	})

	Logger.Info(EventAuthLoginSuccess, // успешный вход
		"event", EventAuthLoginSuccess,
		"outcome", "success",
		"method", r.Method, "path", r.URL.Path,
		"user_id", user.ID,
		"login", user.Login,
		"user_role", string(user.Role),
		"client_ip", clientIP(r))

	writeJSON(w, http.StatusOK, map[string]any{
		"message":    "успешная авторизация",
		"session_id": sessionID,
		"token":      tokenString,
		"user": map[string]any{
			"id":    user.ID,
			"login": user.Login,
			"name":  user.Name,
			"role":  user.Role,
		},
	})
}

// GetMe godoc
// @Summary     Текущий пользователь
// @Description Возвращает профиль по JWT из куки auth_token (для восстановления сессии на фронте)
// @Tags        auth
// @Produce     json
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} map[string]string
// @Security    CookieAuth
// @Router      /auth/me [get]
func GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromCtx(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "требуется авторизация")
		return
	}

	var user models.User
	if err := db.DB.First(&user, userID).Error; err != nil {
		writeError(w, http.StatusUnauthorized, "пользователь не найден")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":    user.ID,
		"login": user.Login,
		"name":  user.Name,
		"role":  user.Role,
	})
}

// Logout godoc
// @Summary     Деавторизация
// @Description Удаляет сессию из Redis и очищает куку
// @Tags        auth
// @Produce     json
// @Success     200 {object} map[string]string
// @Router      /auth/logout [post]
func Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		_ = DeleteSession(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     authTokenCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})

	writeJSON(w, http.StatusOK, map[string]string{"message": "выход выполнен"})
}

// UpdateProfile godoc
// @Summary     Обновить профиль
// @Description Изменяет имя и/или пароль текущего пользователя
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       body body object true "name, password"
// @Success     200 {object} map[string]interface{}
// @Failure     400 {object} map[string]string
// @Failure     401 {object} map[string]string
// @Security    CookieAuth
// @Router      /auth/profile [put]
func UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserIDFromCtx(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "требуется авторизация")
		return
	}

	var body struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}

	name := strings.TrimSpace(body.Name)
	password := strings.TrimSpace(body.Password)
	if name == "" && password == "" {
		writeError(w, http.StatusBadRequest, "передайте хотя бы одно поле: name или password")
		return
	}

	var user models.User
	if err := db.DB.First(&user, userID).Error; err != nil {
		writeError(w, http.StatusUnauthorized, "пользователь не найден")
		return
	}

	if name != "" {
		user.Name = name
	}
	if password != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "ошибка хэширования пароля")
			return
		}
		user.Password = string(hashed)
	}

	if err := db.DB.Save(&user).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "не удалось обновить профиль")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":    user.ID,
		"login": user.Login,
		"name":  user.Name,
		"role":  user.Role,
	})
}
