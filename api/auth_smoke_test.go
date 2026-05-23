//go:build smoke

package api

import "testing"

// проверяем успешную регистрацию пользователя
func TestRegister_Success(t *testing.T) {
	testRegisterSuccess(t)
}

// проверяем успешный вход в систему
func TestLogin_Success(t *testing.T) {
	testLoginSuccess(t)
}

// проверяем получение профиля после входа
func TestGetMe_Authenticated(t *testing.T) {
	testGetMeAuthenticated(t)
}
