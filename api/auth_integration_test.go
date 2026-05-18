//go:build integration

package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegister_Success(t *testing.T) {
	setupTestEnv(t)
	server := newTestServer(t)
	defer server.Close()

	login := uniqueLogin("reg_ok")
	body := mustMarshal(t, map[string]string{
		"login":    login,
		"password": "secret123",
		"name":     "Тест",
	})

	resp := makeRequest(t, server, http.MethodPost, "/api/auth/register", body, nil)
	assert.Equal(t, http.StatusCreated, resp.Status)

	var data map[string]any
	parseJSONBody(t, resp.Body, &data)
	assert.NotZero(t, data["id"])
	assert.Equal(t, login, data["login"])
	assert.Equal(t, "creator", data["role"])
}

func TestRegister_DuplicateLogin(t *testing.T) {
	setupTestEnv(t)
	server := newTestServer(t)
	defer server.Close()

	login := uniqueLogin("reg_dup")
	body := mustMarshal(t, map[string]string{
		"login":    login,
		"password": "secret123",
		"name":     "Тест",
	})

	resp1 := makeRequest(t, server, http.MethodPost, "/api/auth/register", body, nil)
	require.Equal(t, http.StatusCreated, resp1.Status)

	resp2 := makeRequest(t, server, http.MethodPost, "/api/auth/register", body, nil)
	assert.Equal(t, http.StatusConflict, resp2.Status)
}

func TestRegister_MissingFields(t *testing.T) {
	setupTestEnv(t)
	server := newTestServer(t)
	defer server.Close()

	body := mustMarshal(t, map[string]string{
		"login": uniqueLogin("reg_miss"),
		"name":  "Тест",
	})

	resp := makeRequest(t, server, http.MethodPost, "/api/auth/register", body, nil)
	assert.Equal(t, http.StatusBadRequest, resp.Status)
}

func TestLogin_Success(t *testing.T) {
	setupTestEnv(t)
	server := newTestServer(t)
	defer server.Close()

	login := uniqueLogin("login_ok")
	pass := "secret123"
	regBody := mustMarshal(t, map[string]string{"login": login, "password": pass, "name": "Тест"})
	regResp := makeRequest(t, server, http.MethodPost, "/api/auth/register", regBody, nil)
	require.Equal(t, http.StatusCreated, regResp.Status)

	loginBody := mustMarshal(t, map[string]string{"login": login, "password": pass})
	loginResp := makeRequest(t, server, http.MethodPost, "/api/auth/login", loginBody, nil)
	assert.Equal(t, http.StatusOK, loginResp.Status)

	var data map[string]any
	parseJSONBody(t, loginResp.Body, &data)
	assert.NotEmpty(t, data["token"])

	var authCookie *http.Cookie
	for _, c := range loginResp.Cookies {
		if c.Name == "auth_token" {
			authCookie = c
			break
		}
	}
	require.NotNil(t, authCookie)
	assert.NotEmpty(t, authCookie.Value)
}

func TestLogin_WrongPassword(t *testing.T) {
	setupTestEnv(t)
	server := newTestServer(t)
	defer server.Close()

	login := uniqueLogin("login_wrong")
	regBody := mustMarshal(t, map[string]string{
		"login": login, "password": "secret123", "name": "Тест",
	})
	require.Equal(t, http.StatusCreated, makeRequest(t, server, http.MethodPost, "/api/auth/register", regBody, nil).Status)

	loginBody := mustMarshal(t, map[string]string{"login": login, "password": "wrong"})
	resp := makeRequest(t, server, http.MethodPost, "/api/auth/login", loginBody, nil)
	assert.Equal(t, http.StatusUnauthorized, resp.Status)
}

func TestGetMe_Authenticated(t *testing.T) {
	setupTestEnv(t)
	server := newTestServer(t)
	defer server.Close()

	login := uniqueLogin("me_ok")
	pass := "secret123"
	regBody := mustMarshal(t, map[string]string{"login": login, "password": pass, "name": "Иван"})
	require.Equal(t, http.StatusCreated, makeRequest(t, server, http.MethodPost, "/api/auth/register", regBody, nil).Status)

	loginBody := mustMarshal(t, map[string]string{"login": login, "password": pass})
	loginResp := makeRequest(t, server, http.MethodPost, "/api/auth/login", loginBody, nil)
	require.Equal(t, http.StatusOK, loginResp.Status)

	meResp := makeRequest(t, server, http.MethodGet, "/api/auth/me", nil, loginResp.Cookies)
	assert.Equal(t, http.StatusOK, meResp.Status)

	var data map[string]any
	parseJSONBody(t, meResp.Body, &data)
	assert.Equal(t, login, data["login"])
}

func TestGetMe_Unauthenticated(t *testing.T) {
	setupTestEnv(t)
	server := newTestServer(t)
	defer server.Close()

	resp := makeRequest(t, server, http.MethodGet, "/api/auth/me", nil, nil)
	assert.Equal(t, http.StatusUnauthorized, resp.Status)
}
