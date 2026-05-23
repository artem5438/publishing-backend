//go:build integration

package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"publishing-backend/db"
	"publishing-backend/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func registerAndLogin(t *testing.T, server *httptest.Server) ([]*http.Cookie, uint) {
	t.Helper()
	login := uniqueLogin("order_user")
	pass := "secret123"
	regBody := mustMarshal(t, map[string]string{"login": login, "password": pass, "name": "Создатель"})
	require.Equal(t, http.StatusCreated, makeRequest(t, server, http.MethodPost, "/api/auth/register", regBody, nil).Status)

	loginBody := mustMarshal(t, map[string]string{"login": login, "password": pass})
	loginResp := makeRequest(t, server, http.MethodPost, "/api/auth/login", loginBody, nil)
	require.Equal(t, http.StatusOK, loginResp.Status)

	var loginData map[string]any
	parseJSONBody(t, loginResp.Body, &loginData)
	user := loginData["user"].(map[string]any)
	userID := uint(user["id"].(float64))

	return loginResp.Cookies, userID
}

func createWorkInDB(t *testing.T, name string, priceRub int) models.Work {
	t.Helper()
	work := models.Work{
		Name:     name,
		Status:   models.WorkStatusActive,
		PriceRub: priceRub,
		WorkType: "тип",
		Unit:     "шт",
	}
	require.NoError(t, db.DB.Create(&work).Error)
	return work
}

// тестируем полный бизнес-сценарий оформления заказа
func TestSubmitOrder_HappyPath(t *testing.T) {
	setupTestEnv(t)
	server := newTestServer(t)
	defer server.Close()

	cookies, _ := registerAndLogin(t, server)
	w1 := createWorkInDB(t, "Услуга 1", 100)
	w2 := createWorkInDB(t, "Услуга 2", 250)

	addBody1 := mustMarshal(t, map[string]uint{"work_id": w1.ID})
	addResp1 := makeRequest(t, server, http.MethodPost, "/api/publishing-orders/cart/works", addBody1, cookies)
	require.Equal(t, http.StatusCreated, addResp1.Status)
	cookies = mergeCookies(cookies, addResp1.Cookies)

	var add1 map[string]any
	parseJSONBody(t, addResp1.Body, &add1)
	orderID := uint(add1["order_id"].(float64))

	addBody2 := mustMarshal(t, map[string]uint{"work_id": w2.ID})
	addResp2 := makeRequest(t, server, http.MethodPost, "/api/publishing-orders/cart/works", addBody2, cookies)
	require.Equal(t, http.StatusCreated, addResp2.Status)
	cookies = mergeCookies(cookies, addResp2.Cookies)

	updateBody := mustMarshal(t, map[string]any{
		"book_title":  "Моя книга",
		"circulation": 100,
	})
	updateResp := makeRequest(t, server, http.MethodPut,
		fmt.Sprintf("/api/publishing-orders/%d", orderID), updateBody, cookies)
	require.Equal(t, http.StatusOK, updateResp.Status)

	submitResp := makeRequest(t, server, http.MethodPut,
		fmt.Sprintf("/api/publishing-orders/%d/submit", orderID), nil, cookies)
	require.Equal(t, http.StatusOK, submitResp.Status)

	var order map[string]any
	parseJSONBody(t, submitResp.Body, &order)
	assert.Equal(t, "formed", order["status"])

	expectedTotal := float64((100*1 + 250*1) * 100)
	assert.Equal(t, expectedTotal, order["total_price"])
}

// тестируем оформление заказа с пустым названием книги
func TestSubmitOrder_EmptyBookTitle(t *testing.T) {
	setupTestEnv(t)
	server := newTestServer(t)
	defer server.Close()

	cookies, userID := registerAndLogin(t, server)
	w := createWorkInDB(t, "Услуга", 50)

	addBody := mustMarshal(t, map[string]uint{"work_id": w.ID})
	addResp := makeRequest(t, server, http.MethodPost, "/api/publishing-orders/cart/works", addBody, cookies)
	require.Equal(t, http.StatusCreated, addResp.Status)
	cookies = mergeCookies(cookies, addResp.Cookies)

	var add map[string]any
	parseJSONBody(t, addResp.Body, &add)
	orderID := uint(add["order_id"].(float64))

	// Убедимся что book_title пустой (только circulation через API не задаёт title)
	submitResp := makeRequest(t, server, http.MethodPut,
		fmt.Sprintf("/api/publishing-orders/%d/submit", orderID), nil, cookies)
	assert.Equal(t, http.StatusBadRequest, submitResp.Status)

	_ = userID
}

// тестируем оформление заказа без услуг
func TestSubmitOrder_NoWorks(t *testing.T) {
	setupTestEnv(t)
	server := newTestServer(t)
	defer server.Close()

	cookies, userID := registerAndLogin(t, server)

	order := models.PublishingOrder{
		Status:      models.StatusDraft,
		CreatorID:   userID,
		BookTitle:   "Книга",
		Circulation: 10,
	}
	require.NoError(t, db.DB.Create(&order).Error)

	submitResp := makeRequest(t, server, http.MethodPut,
		fmt.Sprintf("/api/publishing-orders/%d/submit", order.ID), nil, cookies)
	assert.Equal(t, http.StatusBadRequest, submitResp.Status)

	var errBody map[string]string
	parseJSONBody(t, submitResp.Body, &errBody)
	assert.Contains(t, errBody["error"], "нет услуг")
}
