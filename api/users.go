package api

import (
	"encoding/json"
	"net/http"

	"publishing-backend/db"
	"publishing-backend/models"

	"golang.org/x/crypto/bcrypt"
)

// ─── POST /api/auth/register ──────────────────────────────────────────────────
// Реальная регистрация: логин, пароль (bcrypt), имя, роль
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
		Role     string `json:"role"` // "creator" или "moderator"
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}

	if body.Login == "" || body.Password == "" || body.Name == "" {
		writeError(w, http.StatusBadRequest, "login, password и name обязательны")
		return
	}

	// Проверяем что логин не занят
	var existing models.User
	if db.DB.Where("login = ?", body.Login).First(&existing).Error == nil {
		writeError(w, http.StatusConflict, "пользователь с таким логином уже существует")
		return
	}

	// Хэшируем пароль
	hashed, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка хэширования пароля")
		return
	}

	role := models.RoleCreator
	if body.Role == string(models.RoleModerator) {
		role = models.RoleModerator
	}

	user := models.User{
		Login:    body.Login,
		Password: string(hashed),
		Name:     body.Name,
		Role:     role,
	}
	if err := db.DB.Create(&user).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка создания пользователя")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":    user.ID,
		"login": user.Login,
		"name":  user.Name,
		"role":  user.Role,
	})
}

// ─── POST /api/auth/login ─────────────────────────────────────────────────────
// Заглушка для лаб. 4 (JWT будет добавлен позже)
// Login godoc
// @Summary     Аутентификация (заглушка)
// @Description Заглушка для лаб. 4, JWT будет добавлен позже
// @Tags        auth
// @Produce     json
// @Success     200 {object} map[string]string
// @Router      /auth/login [post]
func Login(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "ok (заглушка, JWT будет в лаб. 4)",
	})
}

// ─── POST /api/auth/logout ────────────────────────────────────────────────────
// Заглушка для лаб. 4
// Logout godoc
// @Summary     Деавторизация (заглушка)
// @Description Заглушка для лаб. 4
// @Tags        auth
// @Produce     json
// @Success     200 {object} map[string]string
// @Router      /auth/logout [post]
func Logout(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "ok (заглушка, JWT будет в лаб. 4)",
	})
}
