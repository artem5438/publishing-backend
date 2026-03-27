package api

import (
	"encoding/json"
	"net/http"
	"time"

	"publishing-backend/db"
	"publishing-backend/models"

	"github.com/go-chi/chi/v5"
)

// ─── Response-структуры ───────────────────────────────────────────────────────

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
	FilledWorksCount int `json:"filled_works_count"`
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

// ─── GET /api/publishing-orders/cart ─────────────────────────────────────────
// Без параметров: возвращает id черновика и кол-во услуг в нём

func GetCart(w http.ResponseWriter, r *http.Request) {
	creatorID := getCreatorID()

	var order models.PublishingOrder
	res := db.DB.Where("creator_id = ? AND status = ?", creatorID, models.StatusDraft).
		Preload("Works").
		First(&order)

	if res.Error != nil {
		// Черновика нет — возвращаем пустую корзину
		writeJSON(w, http.StatusOK, map[string]any{
			"order_id":    nil,
			"works_count": 0,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"order_id":    order.ID,
		"works_count": len(order.Works),
	})
}

// ─── GET /api/publishing-orders ───────────────────────────────────────────────
// Список заявок (без черновиков и удалённых), фильтр по статусу и диапазону даты

func GetOrders(w http.ResponseWriter, r *http.Request) {
	q := db.DB.
		Preload("Creator").
		Preload("Moderator").
		Preload("Works").
		Where("status != ? AND status != ?", models.StatusDraft, models.StatusDeleted)

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
			// до конца дня включительно
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

// ─── GET /api/publishing-orders/{id} ─────────────────────────────────────────
// Одна заявка + список её услуг с картинками

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

	writeJSON(w, http.StatusOK, toOrderResponse(order, true))
}

// ─── PUT /api/publishing-orders/{id} ─────────────────────────────────────────
// Изменить тематические поля заявки (системные поля — запрещены)

func UpdateOrder(w http.ResponseWriter, r *http.Request) {
	creatorID := getCreatorID()
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

// ─── PUT /api/publishing-orders/{id}/submit ───────────────────────────────────
// Создатель формирует черновик: проверки → расчёт стоимости → draft → formed

func SubmitOrder(w http.ResponseWriter, r *http.Request) {
	creatorID := getCreatorID()
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

// ─── PUT /api/publishing-orders/{id}/moderate ────────────────────────────────
// Модератор завершает или отклоняет сформированную заявку

func ModerateOrder(w http.ResponseWriter, r *http.Request) {
	// В лаб. 3 модератор тоже зафиксирован константой
	moderatorID := getCreatorID()
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

// ─── DELETE /api/publishing-orders/{id} ───────────────────────────────────────
// Логическое удаление черновика создателем

func DeleteOrder(w http.ResponseWriter, r *http.Request) {
	creatorID := getCreatorID()
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
