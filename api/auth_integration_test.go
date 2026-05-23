//go:build integration

package api

import (
	"testing"
)

// тестируем регистрацию пользователя с дубликатным логином
func TestRegister_DuplicateLogin(t *testing.T) {
	testRegisterDuplicateLogin(t)
}

// тестируем регистрацию пользователя без пароля
func TestRegister_MissingFields(t *testing.T) {
	testRegisterMissingFields(t)
}

// тестируем вход в систему с неверным паролем
func TestLogin_WrongPassword(t *testing.T) {
	testLoginWrongPassword(t)
}

// тестируем получение профиля пользователя без авторизации
func TestGetMe_Unauthenticated(t *testing.T) {
	testGetMeUnauthenticated(t)
}
