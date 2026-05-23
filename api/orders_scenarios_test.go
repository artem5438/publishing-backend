//go:build integration || regression

package api

import (
	"fmt"
	"net/http"
	"testing"

	"publishing-backend/db"
	"publishing-backend/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: создаём активную услугу напрямую в тестовой БД
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

// сценарий: оформление заявки с двумя услугами и проверкой total_price
func testSubmitOrderHappyPath(t *testing.T) {
	t.Helper()
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

// сценарий: submit должен вернуть ошибку, если не заполнен book_title
func testSubmitOrderEmptyBookTitle(t *testing.T) {
	t.Helper()
	setupTestEnv(t)
	server := newTestServer(t)
	defer server.Close()

	cookies, _ := registerAndLogin(t, server)
	w := createWorkInDB(t, "Услуга", 50)

	addBody := mustMarshal(t, map[string]uint{"work_id": w.ID})
	addResp := makeRequest(t, server, http.MethodPost, "/api/publishing-orders/cart/works", addBody, cookies)
	require.Equal(t, http.StatusCreated, addResp.Status)
	cookies = mergeCookies(cookies, addResp.Cookies)

	var add map[string]any
	parseJSONBody(t, addResp.Body, &add)
	orderID := uint(add["order_id"].(float64))

	submitResp := makeRequest(t, server, http.MethodPut,
		fmt.Sprintf("/api/publishing-orders/%d/submit", orderID), nil, cookies)
	assert.Equal(t, http.StatusBadRequest, submitResp.Status)
}

// сценарий: submit должен вернуть ошибку, если в заявке нет услуг
func testSubmitOrderNoWorks(t *testing.T) {
	t.Helper()
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
