package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"publishing-backend/db"
	"publishing-backend/models"

	"github.com/go-chi/chi/v5"
)

// ─── POST /api/publishing-orders/{id}/works ───────────────────────────────────
// Добавить услугу в заявку-черновик. Если черновика нет — создаётся автоматически.
// AddWorkToOrder godoc
// @Summary     Добавить услугу в корзину
// @Description Добавляет услугу в черновик. Если черновика нет — создаётся автоматически.
// @Tags        order-works
// @Accept      json
// @Produce     json
// @Param       body body object true "work_id"
// @Success     201  {object} map[string]interface{}
// @Failure     400  {object} map[string]string
// @Failure     404  {object} map[string]string
// @Router      /publishing-orders/cart/works [post]
func AddWorkToOrder(w http.ResponseWriter, r *http.Request) {
	creatorID := getCreatorID()

	var body struct {
		WorkID uint `json:"work_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.WorkID == 0 {
		writeError(w, http.StatusBadRequest, "поле work_id обязательно")
		return
	}

	// Проверяем что услуга существует и активна
	var work models.Work
	if res := db.DB.Where("id = ? AND status = ?", body.WorkID, models.WorkStatusActive).First(&work); res.Error != nil {
		writeError(w, http.StatusNotFound, "услуга не найдена")
		return
	}

	// Ищем черновик, если нет — создаём
	var order models.PublishingOrder
	res := db.DB.Where("creator_id = ? AND status = ?", creatorID, models.StatusDraft).First(&order)
	if res.Error != nil {
		order = models.PublishingOrder{
			Status:    models.StatusDraft,
			CreatorID: creatorID,
		}
		if err := db.DB.Create(&order).Error; err != nil {
			writeError(w, http.StatusInternalServerError, "ошибка создания заявки")
			return
		}
	}

	// Если услуга уже есть в заявке — ошибка
	var existing models.OrderWork
	if db.DB.Where("order_id = ? AND work_id = ?", order.ID, body.WorkID).First(&existing).Error == nil {
		writeError(w, http.StatusConflict, "услуга уже добавлена в заявку")
		return
	}

	orderWork := models.OrderWork{
		OrderID:  order.ID,
		WorkID:   body.WorkID,
		Quantity: 1,
	}
	if err := db.DB.Create(&orderWork).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка добавления услуги")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"order_id": order.ID,
		"work_id":  body.WorkID,
		"quantity": 1,
	})
}

// ─── PUT /api/publishing-orders/{id}/works/{workId} ──────────────────────────
// Изменить количество позиции в заявке (без PK м-м, ищем по order_id + work_id)
// UpdateOrderWork godoc
// @Summary     Изменить позицию в заявке
// @Description Изменяет количество и комментарий позиции М-М (без PK м-м)
// @Tags        order-works
// @Accept      json
// @Produce     json
// @Param       id     path int    true "ID заявки"
// @Param       workId path int    true "ID услуги"
// @Param       body   body object true "quantity, comment"
// @Success     200    {object} map[string]interface{}
// @Failure     403    {object} map[string]string
// @Failure     404    {object} map[string]string
// @Router      /publishing-orders/{id}/works/{workId} [put]
func UpdateOrderWork(w http.ResponseWriter, r *http.Request) {
	creatorID := getCreatorID()

	orderID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || orderID == 0 {
		writeError(w, http.StatusBadRequest, "некорректный id заявки")
		return
	}
	workID, err := strconv.Atoi(chi.URLParam(r, "workId"))
	if err != nil || workID == 0 {
		writeError(w, http.StatusBadRequest, "некорректный workId")
		return
	}

	// Проверяем что заявка принадлежит пользователю и в статусе draft
	var order models.PublishingOrder
	if db.DB.Where("id = ? AND creator_id = ? AND status = ?", orderID, creatorID, models.StatusDraft).First(&order).Error != nil {
		writeError(w, http.StatusForbidden, "заявка не найдена или недоступна")
		return
	}

	var body struct {
		Quantity int    `json:"quantity"`
		Comment  string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	if body.Quantity < 1 {
		writeError(w, http.StatusBadRequest, "quantity должен быть >= 1")
		return
	}

	var ow models.OrderWork
	if db.DB.Where("order_id = ? AND work_id = ?", orderID, workID).First(&ow).Error != nil {
		writeError(w, http.StatusNotFound, "позиция не найдена в заявке")
		return
	}

	if err := db.DB.Model(&ow).Updates(models.OrderWork{
		Quantity: body.Quantity,
		Comment:  body.Comment,
	}).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка обновления")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"order_id": orderID,
		"work_id":  workID,
		"quantity": ow.Quantity,
		"comment":  ow.Comment,
	})
}

// ─── DELETE /api/publishing-orders/{id}/works/{workId} ───────────────────────
// Удалить позицию из заявки (без PK м-м)
// RemoveWorkFromOrder godoc
// @Summary     Удалить услугу из заявки
// @Description Удаляет позицию из черновика (без PK м-м)
// @Tags        order-works
// @Produce     json
// @Param       id     path int true "ID заявки"
// @Param       workId path int true "ID услуги"
// @Success     200    {object} map[string]string
// @Failure     403    {object} map[string]string
// @Failure     404    {object} map[string]string
// @Router      /publishing-orders/{id}/works/{workId} [delete]
func RemoveWorkFromOrder(w http.ResponseWriter, r *http.Request) {
	creatorID := getCreatorID()

	orderID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || orderID == 0 {
		writeError(w, http.StatusBadRequest, "некорректный id заявки")
		return
	}
	workID, err := strconv.Atoi(chi.URLParam(r, "workId"))
	if err != nil || workID == 0 {
		writeError(w, http.StatusBadRequest, "некорректный workId")
		return
	}

	// Проверяем что заявка принадлежит пользователю и в статусе draft
	var order models.PublishingOrder
	if db.DB.Where("id = ? AND creator_id = ? AND status = ?", orderID, creatorID, models.StatusDraft).First(&order).Error != nil {
		writeError(w, http.StatusForbidden, "заявка не найдена или недоступна")
		return
	}

	result := db.DB.Where("order_id = ? AND work_id = ?", orderID, workID).Delete(&models.OrderWork{})
	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, "ошибка удаления")
		return
	}
	if result.RowsAffected == 0 {
		writeError(w, http.StatusNotFound, "позиция не найдена в заявке")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "услуга удалена из заявки"})
}
