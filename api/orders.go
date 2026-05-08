package api

import (
	"encoding/json"
	"net/http"
	"time"

	"publishing-backend/db"
	"publishing-backend/models"

	"github.com/go-chi/chi/v5"
)

//  Response-структуры

type OrderWorkResponse struct {
	WorkID   uint   `json:"work_id"`
	WorkName string `json:"work_name"`
	PriceRub int    `json:"price_rub"`
	Quantity int    `json:"quantity"`
	Comment  string `json:"comment"`
	ImageURL string `json:"image_url,omitempty"`
}

type OrderResponse struct {
	ID             uint                `json:"id"`
	Status         models.OrderStatus  `json:"status"`
	CreatorLogin   string              `json:"creator_login"`
	ModeratorLogin string              `json:"moderator_login,omitempty"`
	BookTitle      string              `json:"book_title"`
	Circulation    int                 `json:"circulation"`
	TotalPrice     *int                `json:"total_price,omitempty"`
	CreatedAt      time.Time           `json:"created_at"`
	FormedAt       *time.Time          `json:"formed_at,omitempty"`
	CompletedAt    *time.Time          `json:"completed_at,omitempty"`
	Works          []OrderWorkResponse `json:"works,omitempty"`
	// Вычисляемое поле: кол-во позиций м-м с непустым комментарием
	FilledWorksCount int    `json:"filled_works_count"`
	UserRole         string `json:"user_role,omitempty"`
}

func toOrderResponse(m models.PublishingOrder, includeWorks bool) OrderResponse {
	resp := OrderResponse{
		ID:          m.ID,
		Status:      m.Status,
		BookTitle:   m.BookTitle,
		Circulation: m.Circulation,
		TotalPrice:  m.TotalPrice,
		CreatedAt:   m.CreatedAt,
		FormedAt:    m.FormedAt,
		CompletedAt: m.CompletedAt,
	}

	// Логины вместо ID
	resp.CreatorLogin = m.Creator.Login
	if m.Moderator != nil {
		resp.ModeratorLogin = m.Moderator.Login
	}

	// Вычисляемое поле: кол-во позиций с непустым комментарием
	filled := 0
	if includeWorks {
		resp.Works = make([]OrderWorkResponse, 0, len(m.Works))
		for _, ow := range m.Works {
			imageURL := ""
			if ow.Work.ImageKey != nil && *ow.Work.ImageKey != "" {
				imageURL = minioBaseURL + "/" + *ow.Work.ImageKey
			}
			resp.Works = append(resp.Works, OrderWorkResponse{
				WorkID:   ow.WorkID,
				WorkName: ow.Work.Name,
				PriceRub: ow.Work.PriceRub,
				Quantity: ow.Quantity,
				Comment:  ow.Comment,
				ImageURL: imageURL,
			})
			if ow.Comment != "" {
				filled++
			}
		}
	} else {
		for _, ow := range m.Works {
			if ow.Comment != "" {
				filled++
			}
		}
	}
	resp.FilledWorksCount = filled

	return resp
}

//	GET /api/publishing-orders/cart
//
// GetCart godoc
// @Summary     Иконка корзины
// @Description Возвращает id черновика и количество услуг в нём
// @Tags        orders
// @Produce     json
// @Success     200 {object} map[string]interface{}
// @Security CookieAuth
// @Router      /publishing-orders/cart [get]
func GetCart(w http.ResponseWriter, r *http.Request) {
	creatorID := getCreatorID(r)

	var order models.PublishingOrder
	res := db.DB.
		Where("creator_id = ? AND status = ?", creatorID, models.StatusDraft).
		Preload("Creator").
		Preload("Works.Work").
		First(&order)

	if res.Error != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"order_id":    nil,
			"works_count": 0,
			"works":       []interface{}{},
			"user_role":   GetUserRoleFromCtx(r),
		})
		return
	}

	resp := toOrderResponse(order, true)
	resp.UserRole = GetUserRoleFromCtx(r)
	writeJSON(w, http.StatusOK, resp)
}

//	GET /api/publishing-orders
//
// GetOrders godoc
// @Summary     Список заявок
// @Description Список без черновиков и удалённых. Создатель видит только свои заявки, модератор — все.
// @Tags        orders
// @Produce     json
// @Param       status query string false "Статус (formed/completed/rejected)"
// @Param       from   query string false "Дата от (2006-01-02)"
// @Param       to     query string false "Дата до (2006-01-02)"
// @Success     200 {array}  OrderResponse
// @Failure     401 {object} map[string]string
// @Security    CookieAuth
// @Router      /publishing-orders [get]
func GetOrders(w http.ResponseWriter, r *http.Request) {
	userID, authenticated := GetUserIDFromCtx(r)
	role := GetUserRoleFromCtx(r)

	Logger.Debug("orders.list.inspect_access",
		"event", "orders.list.inspect_access",
		"authenticated", authenticated,
		"user_id", userID,
		"role", role,
	)

	if !authenticated {
		writeError(w, http.StatusUnauthorized, "требуется авторизация")
		return
	}

	q := db.DB.
		Preload("Creator").
		Preload("Moderator").
		Preload("Works").
		Where("status != ? AND status != ?", models.StatusDraft, models.StatusDeleted)

	// Создатель видит только свои заявки
	if role != string(models.RoleModerator) {
		q = q.Where("creator_id = ?", userID)
	}

	// Фильтр по статусу
	if status := r.URL.Query().Get("status"); status != "" {
		q = q.Where("status = ?", status)
	}

	// Фильтр по диапазону даты формирования
	if from := r.URL.Query().Get("from"); from != "" {
		t, err := time.Parse("2006-01-02", from)
		if err == nil {
			q = q.Where("formed_at >= ?", t)
		}
	}
	if to := r.URL.Query().Get("to"); to != "" {
		t, err := time.Parse("2006-01-02", to)
		if err == nil {
			q = q.Where("formed_at <= ?", t.Add(24*time.Hour-time.Second))
		}
	}

	var orders []models.PublishingOrder
	q.Find(&orders)

	result := make([]OrderResponse, 0, len(orders))
	for _, o := range orders {
		result = append(result, toOrderResponse(o, false))
	}

	writeJSON(w, http.StatusOK, result)
}

//	GET /api/publishing-orders/{id}
//
// GetOrder godoc
// @Summary     Одна заявка
// @Description Возвращает заявку с полным списком услуг и картинками. Доступ: свой черновик; сформированные и прочие — создателю своих или модератору (чужие черновики недоступны).
// @Tags        orders
// @Produce     json
// @Param       id path int true "ID заявки"
// @Success     200 {object} OrderResponse
// @Failure     401 {object} map[string]string
// @Failure     404 {object} map[string]string
// @Security CookieAuth
// @Router      /publishing-orders/{id} [get]
func GetOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var order models.PublishingOrder
	res := db.DB.
		Preload("Creator").
		Preload("Moderator").
		Preload("Works.Work").
		Where("id = ? AND status != ?", id, models.StatusDeleted).
		First(&order)

	if res.Error != nil {
		writeError(w, http.StatusNotFound, "заявка не найдена")
		return
	}

	userID, ok := GetUserIDFromCtx(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "требуется авторизация")
		return
	}
	role := GetUserRoleFromCtx(r)
	if !OrderVisibleToUser(&order, userID, role) {
		writeError(w, http.StatusNotFound, "заявка не найдена")
		return
	}

	writeJSON(w, http.StatusOK, toOrderResponse(order, true))
}

//	PUT /api/publishing-orders/{id}
//
// UpdateOrder godoc
// @Summary     Изменить заявку
// @Description Изменяет тематические поля черновика (book_title, circulation)
// @Tags        orders
// @Accept      json
// @Produce     json
// @Param       id   path     int  true "ID заявки"
// @Param       body body     object true "Поля для обновления"
// @Success     200  {object} OrderResponse
// @Failure     403  {object} map[string]string
// @Security CookieAuth
// @Router      /publishing-orders/{id} [put]
func UpdateOrder(w http.ResponseWriter, r *http.Request) {
	creatorID := getCreatorID(r)
	id := chi.URLParam(r, "id")

	var order models.PublishingOrder
	if db.DB.Where("id = ? AND creator_id = ? AND status = ?", id, creatorID, models.StatusDraft).First(&order).Error != nil {
		writeError(w, http.StatusForbidden, "заявка не найдена или недоступна для изменения")
		return
	}

	var body struct {
		BookTitle   string `json:"book_title"`
		Circulation int    `json:"circulation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}

	if err := db.DB.Model(&order).Updates(map[string]any{
		"book_title":  body.BookTitle,
		"circulation": body.Circulation,
	}).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка обновления")
		return
	}

	db.DB.Preload("Creator").Preload("Moderator").Preload("Works.Work").First(&order)
	writeJSON(w, http.StatusOK, toOrderResponse(order, true))
}

//	PUT /api/publishing-orders/{id}/submit
//
// SubmitOrder godoc
// @Summary     Сформировать заявку
// @Description Переводит черновик в статус formed. Проверяет обязательные поля и рассчитывает total_price.
// @Tags        orders
// @Produce     json
// @Param       id path int true "ID заявки"
// @Success     200 {object} OrderResponse
// @Failure     400 {object} map[string]string
// @Failure     403 {object} map[string]string
// @Security CookieAuth
// @Router      /publishing-orders/{id}/submit [put]
func SubmitOrder(w http.ResponseWriter, r *http.Request) {
	creatorID := getCreatorID(r)
	id := chi.URLParam(r, "id")

	var order models.PublishingOrder
	if db.DB.
		Preload("Works.Work").
		Where("id = ? AND creator_id = ? AND status = ?", id, creatorID, models.StatusDraft).
		First(&order).Error != nil {
		writeError(w, http.StatusForbidden, "черновик не найден")
		return
	}

	// Проверка обязательных полей
	if order.BookTitle == "" {
		writeError(w, http.StatusBadRequest, "укажите название книги (book_title)")
		return
	}
	if order.Circulation <= 0 {
		writeError(w, http.StatusBadRequest, "укажите тираж (circulation > 0)")
		return
	}
	if len(order.Works) == 0 {
		writeError(w, http.StatusBadRequest, "в заявке нет услуг")
		return
	}

	// Расчёт итоговой стоимости: сумма(цена × количество) × тираж
	total := 0
	for _, ow := range order.Works {
		total += ow.Work.PriceRub * ow.Quantity
	}
	total = total * order.Circulation

	now := time.Now()
	if err := db.DB.Model(&order).Updates(map[string]any{
		"status":      models.StatusFormed,
		"formed_at":   now,
		"total_price": total,
	}).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка формирования заявки")
		return
	}

	db.DB.Preload("Creator").Preload("Moderator").Preload("Works.Work").First(&order)
	writeJSON(w, http.StatusOK, toOrderResponse(order, true))
}

//	PUT /api/publishing-orders/{id}/moderate
//
// ModerateOrder godoc
// @Summary     Завершить или отклонить заявку
// @Description Модератор завершает (complete) или отклоняет (reject) сформированную заявку
// @Tags        orders
// @Accept      json
// @Produce     json
// @Param       id   path int    true "ID заявки"
// @Param       body body object true "action: complete или reject"
// @Success     200  {object} OrderResponse
// @Failure     400  {object} map[string]string
// @Security CookieAuth
// @Router      /publishing-orders/{id}/moderate [put]
func ModerateOrder(w http.ResponseWriter, r *http.Request) {
	// В лаб. 3 модератор тоже зафиксирован константой
	moderatorID := getCreatorID(r)
	id := chi.URLParam(r, "id")

	var order models.PublishingOrder
	if db.DB.Where("id = ? AND status = ?", id, models.StatusFormed).First(&order).Error != nil {
		writeError(w, http.StatusBadRequest, "заявка не найдена или не в статусе 'сформирована'")
		return
	}

	var body struct {
		Action string `json:"action"` // "complete" или "reject"
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}

	var newStatus models.OrderStatus
	switch body.Action {
	case "complete":
		newStatus = models.StatusCompleted
	case "reject":
		newStatus = models.StatusRejected
	default:
		writeError(w, http.StatusBadRequest, "action должен быть 'complete' или 'reject'")
		return
	}

	now := time.Now()
	if err := db.DB.Model(&order).Updates(map[string]any{
		"status":       newStatus,
		"moderator_id": moderatorID,
		"completed_at": now,
	}).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка модерации")
		return
	}

	db.DB.Preload("Creator").Preload("Moderator").Preload("Works.Work").First(&order)
	writeJSON(w, http.StatusOK, toOrderResponse(order, true))
}

//	DELETE /api/publishing-orders/{id}
//
// Логическое удаление черновика создателем
// DeleteOrder godoc
// @Summary     Удалить заявку
// @Description Логическое удаление черновика создателем
// @Tags        orders
// @Produce     json
// @Param       id path int true "ID заявки"
// @Success     200 {object} map[string]string
// @Failure     403 {object} map[string]string
// @Security CookieAuth
// @Router      /publishing-orders/{id} [delete]
func DeleteOrder(w http.ResponseWriter, r *http.Request) {
	creatorID := getCreatorID(r)
	id := chi.URLParam(r, "id")

	var order models.PublishingOrder
	if db.DB.Where("id = ? AND creator_id = ? AND status = ?", id, creatorID, models.StatusDraft).First(&order).Error != nil {
		writeError(w, http.StatusForbidden, "черновик не найден или недоступен")
		return
	}

	now := time.Now()
	if err := db.DB.Model(&order).Updates(map[string]any{
		"status":    models.StatusDeleted,
		"formed_at": now,
	}).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка удаления")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "заявка удалена"})
}

// OrderVisibleToUser — правила чтения заявки (согласованы со списком GetOrders).
func OrderVisibleToUser(order *models.PublishingOrder, userID uint, role string) bool {
	if order.Status == models.StatusDeleted {
		return false
	}
	if order.Status == models.StatusDraft {
		return order.CreatorID == userID
	}
	if role == string(models.RoleModerator) {
		return true
	}
	return order.CreatorID == userID
}
