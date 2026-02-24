package main

import (
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"publishing-backend/db"
	"publishing-backend/models"

	"github.com/go-chi/chi/v5"
)

// ─── View-структуры (шаблоны не меняем!) ───────────────────────────────────

type WorkParams struct {
	Deadline string
	Quantity string
	Unit     string
	Format   string
}

type Work struct {
	ID          int
	Name        string
	PriceRub    int
	Description string
	ImageKey    string
	VideoKey    string
	Tags        []string
	Params      WorkParams
}

type OrderItem struct {
	WorkID   int
	WorkName string
	PriceRub int
	Quantity int
	ImageKey string
}

type PublishingOrder struct {
	ID         int
	Items      []OrderItem
	ResultText string
}

// ─── Вспомогательная функция ────────────────────────────────────────────────

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ─── Маппинг: models.Work (GORM) → Work (view) ─────────────────────────────

func toViewWork(m models.Work) Work {
	return Work{
		ID:          int(m.ID),
		Name:        m.Name,
		PriceRub:    m.PriceRub,
		Description: m.Description,
		ImageKey:    strVal(m.ImageKey),
		VideoKey:    strVal(m.VideoKey),
		Tags:        []string{m.Tag1, m.Tag2, m.Tag3},
		Params: WorkParams{
			Deadline: m.ParamDeadline,
			Quantity: m.ParamQuantity,
			Unit:     m.ParamUnit,
			Format:   m.ParamFormat,
		},
	}
}

// ─── Маппинг: models.PublishingOrder (GORM) → PublishingOrder (view) ────────

func toViewOrder(m models.PublishingOrder) PublishingOrder {
	items := []OrderItem{}
	total := 0
	for _, ow := range m.Works {
		items = append(items, OrderItem{
			WorkID:   int(ow.WorkID),
			WorkName: ow.Work.Name,
			PriceRub: ow.Work.PriceRub,
			Quantity: ow.Quantity,
			ImageKey: strVal(ow.Work.ImageKey),
		})
		total += ow.Work.PriceRub * ow.Quantity
	}
	return PublishingOrder{
		ID:         int(m.ID),
		Items:      items,
		ResultText: "Ориентировочная стоимость: " + strconv.Itoa(total) + " ₽",
	}
}

// ─── Константы ──────────────────────────────────────────────────────────────

const minioURL = "http://localhost:9000/publishing-media"
const creatorID = 1 // пока константа, в лабе 4 заменим на сессию

// ─── main ───────────────────────────────────────────────────────────────────

func main() {
	db.Connect()
	db.Migrate()

	r := chi.NewRouter()
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/works", http.StatusSeeOther)
	})
	r.Get("/works", worksListHandler)
	r.Get("/works/{id}", workDetailHandler)
	r.Get("/publishing-orders/{id}", orderDetailHandler)
	r.Post("/publishing-orders/add-work", addWorkToOrderHandler)
	r.Post("/publishing-orders/{id}/delete", deleteOrderHandler)

	log.Println("Сервер запущен: http://localhost:8080")
	log.Println("Minio консоль:  http://localhost:9001")
	http.ListenAndServe(":8080", r)
}

// ─── GET /works ──────────────────────────────────────────────────────────────

func worksListHandler(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(r.URL.Query().Get("query"))

	var dbWorks []models.Work
	tx := db.DB.Where("status = ?", models.WorkStatusActive)
	if query != "" {
		tx = tx.Where("LOWER(name) LIKE ?", "%"+query+"%")
	}
	tx.Find(&dbWorks)

	works := []Work{}
	for _, dw := range dbWorks {
		works = append(works, toViewWork(dw))
	}

	var currentOrder models.PublishingOrder
	cartCount := 0
	orderID := 0
	res := db.DB.Where("creator_id = ? AND status = ?", creatorID, models.StatusDraft).
		Preload("Works").
		First(&currentOrder)
	if res.Error == nil {
		cartCount = len(currentOrder.Works)
		orderID = int(currentOrder.ID)
	}

	data := struct {
		Works          []Work
		Query          string
		MinioURL       string
		CartCount      int
		CurrentOrderID int
	}{
		Works:          works,
		Query:          query,
		MinioURL:       minioURL,
		CartCount:      cartCount,
		CurrentOrderID: orderID,
	}

	tmpl := template.Must(template.ParseFiles("templates/works_list.html"))
	tmpl.Execute(w, data)
}

// ─── GET /works/{id} ─────────────────────────────────────────────────────────

func workDetailHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var dbWork models.Work
	res := db.DB.Where("id = ? AND status = ?", id, models.WorkStatusActive).First(&dbWork)
	if res.Error != nil {
		http.NotFound(w, r)
		return
	}

	data := struct {
		Work     Work
		MinioURL string
	}{
		Work:     toViewWork(dbWork),
		MinioURL: minioURL,
	}

	tmpl := template.Must(template.ParseFiles("templates/work_detail.html"))
	tmpl.Execute(w, data)
}

// ─── GET /publishing-orders/{id} ─────────────────────────────────────────────

func orderDetailHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var dbOrder models.PublishingOrder
	res := db.DB.Where("id = ? AND status != ?", id, models.StatusDeleted).
		Preload("Works.Work").
		First(&dbOrder)
	if res.Error != nil {
		http.NotFound(w, r)
		return
	}

	data := struct {
		Order    PublishingOrder
		MinioURL string
	}{
		Order:    toViewOrder(dbOrder),
		MinioURL: minioURL,
	}

	tmpl := template.Must(template.ParseFiles("templates/order_detail.html"))
	tmpl.Execute(w, data)
}

// ─── POST /publishing-orders/add-work ────────────────────────────────────────
// заглушка — реализуем в этапе 7

func addWorkToOrderHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/works", http.StatusSeeOther)
}

// ─── POST /publishing-orders/{id}/delete ─────────────────────────────────────
// заглушка — реализуем в этапе 8

func deleteOrderHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/works", http.StatusSeeOther)
}
