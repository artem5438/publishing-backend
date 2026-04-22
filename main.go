// @title           Publishing Backend API
// @version         2.0
// @description     REST API для книжного издательства (лаб. 4). Swagger UI на том же host:port, что и API, отправляет обычные браузерные куки — после входа остаётся auth_token, и запросы идут как у авторизованного пользователя. Чтобы проверить сценарий гостя, удалите куки для этого origin или откройте приватное окно.
// @host            localhost:8080
// @BasePath        /api

// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name auth_token

package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	_ "publishing-backend/docs"

	"publishing-backend/api"
	"publishing-backend/db"
	"publishing-backend/models"

	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger"
)

//  View-структуры (SSR)

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
	Comment  string
}

type PublishingOrder struct {
	ID          int
	Items       []OrderItem
	ResultText  string
	BookTitle   string
	Circulation int
}

//  Вспомогательные функции (SSR) ─

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

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
			Comment:  ow.Comment,
		})
		total += ow.Work.PriceRub * ow.Quantity
	}
	return PublishingOrder{
		ID:          int(m.ID),
		Items:       items,
		ResultText:  "Ориентировочная стоимость: " + strconv.Itoa(total) + " ₽",
		BookTitle:   m.BookTitle,
		Circulation: m.Circulation,
	}
}

//  Константы (SSR)

const minioURL = "http://localhost:9000/publishing-media"
const creatorID = 1

func getCartInfo() (cartCount int, orderID int) {
	var currentOrder models.PublishingOrder
	res := db.DB.Where("creator_id = ? AND status = ?", creatorID, models.StatusDraft).
		Preload("Works").
		First(&currentOrder)
	if res.Error == nil {
		cartCount = len(currentOrder.Works)
		orderID = int(currentOrder.ID)
	}
	return
}

//  main ─

func main() {
	db.Connect()
	db.Migrate()
	db.ConnectRedis()

	r := chi.NewRouter()

	//  CORS (для фронтенда на localhost:5173)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	//  Swagger UI ─
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("http://localhost:8080/swagger/doc.json"),
	))

	//  SSR маршруты (лаб. 1–2)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/works", http.StatusSeeOther)
	})
	r.Get("/works", worksListHandler)
	r.Get("/works/{id}", workDetailHandler)
	r.Get("/publishing-orders/{id}", orderDetailHandler)
	r.Post("/publishing-orders/add-work", addWorkToOrderHandler)
	r.Post("/publishing-orders/{id}/delete", deleteOrderHandler)
	r.Post("/publishing-orders/{id}/update-work", updateWorkQuantityHandler)

	//  REST API маршруты (лаб. 3)
	r.Route("/api", func(r chi.Router) {
		r.Use(api.AuthMiddleware)

		//  Публичные (без авторизации): только auth
		r.Post("/auth/register", api.Register)
		r.Post("/auth/login", api.Login)
		r.Post("/auth/logout", api.Logout)

		//  Требуется авторизация (creator + moderator) ─
		r.Group(func(r chi.Router) {
			r.Use(api.RequireAuth)

			r.Get("/works", api.GetWorks)
			r.Get("/works/{id}", api.GetWork)
			r.Post("/works", api.CreateWork)

			r.Get("/publishing-orders", api.GetOrders)
			r.Get("/publishing-orders/cart", api.GetCart)
			r.Get("/publishing-orders/{id}", api.GetOrder)
			r.Put("/publishing-orders/{id}", api.UpdateOrder)
			r.Put("/publishing-orders/{id}/submit", api.SubmitOrder)
			r.Delete("/publishing-orders/{id}", api.DeleteOrder)

			r.Post("/publishing-orders/cart/works", api.AddWorkToOrder)
			r.Put("/publishing-orders/{id}/works/{workId}", api.UpdateOrderWork)
			r.Delete("/publishing-orders/{id}/works/{workId}", api.RemoveWorkFromOrder)

			//  Только модератор
			r.Group(func(r chi.Router) {
				r.Use(api.RequireModerator)
				r.Put("/publishing-orders/{id}/moderate", api.ModerateOrder)
			})
		})
	})

	log.Println("Сервер запущен: http://localhost:8080")
	log.Println("API:            http://localhost:8080/api")
	log.Println("Swagger UI:     http://localhost:8080/swagger/")
	log.Println("Minio консоль:  http://localhost:9001")

	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal("Ошибка сервера:", err)
	}
}

//  SSR: GET /works ─

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

	cartCount, orderID := getCartInfo()

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
	if err := tmpl.Execute(w, data); err != nil {
		log.Println("Ошибка шаблона:", err)
	}
}

//  SSR: GET /works/{id}

func workDetailHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var dbWork models.Work
	res := db.DB.Where("id = ? AND status = ?", id, models.WorkStatusActive).First(&dbWork)
	if res.Error != nil {
		http.NotFound(w, r)
		return
	}

	cartCount, orderID := getCartInfo()

	data := struct {
		Work           Work
		MinioURL       string
		CartCount      int
		CurrentOrderID int
	}{
		Work:           toViewWork(dbWork),
		MinioURL:       minioURL,
		CartCount:      cartCount,
		CurrentOrderID: orderID,
	}

	tmpl := template.Must(template.ParseFiles("templates/work_detail.html"))
	if err := tmpl.Execute(w, data); err != nil {
		log.Println("Ошибка шаблона:", err)
	}
}

//  SSR: GET /publishing-orders/{id}

func orderDetailHandler(w http.ResponseWriter, r *http.Request) {
	uid, role, authed := api.CredentialsFromRequest(r)
	if !authed {
		http.Error(w, "требуется авторизация", http.StatusUnauthorized)
		return
	}

	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var dbOrder models.PublishingOrder
	res := db.DB.Where("id = ? AND status != ?", id, models.StatusDeleted).
		Preload("Works.Work").
		First(&dbOrder)
	if res.Error != nil {
		http.NotFound(w, r)
		return
	}

	if !api.OrderVisibleToUser(&dbOrder, uid, role) {
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
	if err := tmpl.Execute(w, data); err != nil {
		log.Println("Ошибка шаблона:", err)
	}
}

//  SSR: POST /publishing-orders/add-work

func addWorkToOrderHandler(w http.ResponseWriter, r *http.Request) {
	workID, err := strconv.Atoi(r.FormValue("work_id"))
	if err != nil || workID == 0 {
		http.Redirect(w, r, "/works", http.StatusSeeOther)
		return
	}

	var order models.PublishingOrder
	res := db.DB.Where("creator_id = ? AND status = ?", creatorID, models.StatusDraft).
		First(&order)

	if res.Error != nil {
		order = models.PublishingOrder{
			Status:    models.StatusDraft,
			CreatorID: creatorID,
		}
		db.DB.Create(&order) //nolint:errcheck
	}

	var existing models.OrderWork
	check := db.DB.Where("order_id = ? AND work_id = ?", order.ID, workID).
		First(&existing)

	if check.Error != nil {
		orderWork := models.OrderWork{
			OrderID:  order.ID,
			WorkID:   uint(workID),
			Quantity: 1,
		}
		db.DB.Create(&orderWork) //nolint:errcheck
	}

	http.Redirect(w, r, "/works", http.StatusSeeOther)
}

//  SSR: POST /publishing-orders/{id}/delete

func deleteOrderHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id == 0 {
		http.Redirect(w, r, "/works", http.StatusSeeOther)
		return
	}

	db.DB.Exec( //nolint:errcheck
		"UPDATE publishing_orders SET status = ? WHERE id = ? AND creator_id = ? AND status = ?",
		models.StatusDeleted, id, creatorID, models.StatusDraft,
	)

	http.Redirect(w, r, "/works", http.StatusSeeOther)
}

//  SSR: POST /publishing-orders/{id}/update-work

func updateWorkQuantityHandler(w http.ResponseWriter, r *http.Request) {
	orderID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || orderID == 0 {
		http.Redirect(w, r, "/works", http.StatusSeeOther)
		return
	}

	workID, err := strconv.Atoi(r.FormValue("work_id"))
	if err != nil || workID == 0 {
		http.Redirect(w, r, fmt.Sprintf("/publishing-orders/%d", orderID), http.StatusSeeOther)
		return
	}

	delta, _ := strconv.Atoi(r.FormValue("delta"))

	var ow models.OrderWork
	res := db.DB.Where("order_id = ? AND work_id = ?", orderID, workID).First(&ow)
	if res.Error == nil {
		newQty := ow.Quantity + delta
		if newQty <= 0 {
			db.DB.Delete(&ow) //nolint:errcheck
		} else {
			db.DB.Model(&ow).Update("quantity", newQty) //nolint:errcheck
		}
	}

	http.Redirect(w, r, fmt.Sprintf("/publishing-orders/%d", orderID), http.StatusSeeOther)
}
