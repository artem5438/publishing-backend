//go:build regression

package api

import "testing"

// проверяем успешную регистрацию пользователя
func TestRegister_Success(t *testing.T) {
	testRegisterSuccess(t)
}

// проверяем запрет повторной регистрации с занятым логином
func TestRegister_DuplicateLogin(t *testing.T) {
	testRegisterDuplicateLogin(t)
}

// проверяем валидацию обязательных полей при регистрации
func TestRegister_MissingFields(t *testing.T) {
	testRegisterMissingFields(t)
}

// проверяем успешный вход в систему
func TestLogin_Success(t *testing.T) {
	testLoginSuccess(t)
}

// проверяем отказ во входе при неверном пароле
func TestLogin_WrongPassword(t *testing.T) {
	testLoginWrongPassword(t)
}

// проверяем доступ к профилю после успешной авторизации
func TestGetMe_Authenticated(t *testing.T) {
	testGetMeAuthenticated(t)
}

// проверяем отказ в доступе к профилю без авторизации
func TestGetMe_Unauthenticated(t *testing.T) {
	testGetMeUnauthenticated(t)
}
