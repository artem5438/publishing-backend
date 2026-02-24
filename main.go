package main

import (
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"publishing-backend/db"
)

// Модели данных
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

type PublishingOrder struct {
	ID         int
	Items      []OrderItem
	ResultText string
}

type OrderItem struct {
	WorkID   int
	WorkName string
	PriceRub int
	Quantity int
	ImageKey string
}

// Данные в памяти (лаба 1: без БД)
var works = []Work{
	{
		ID: 1, Name: "Цифровая печать", PriceRub: 5000,
		Description: "Цифровая печать — оперативный способ выпуска книги тиражом от 1 до 500 экземпляров. Идеально для презентационных, авторских и корпоративных изданий. Печать на оборудовании с разрешением 1200 dpi. Стоимость рассчитывается по тиражу и формату.",
		ImageKey:    "print-digital.jpg", VideoKey: "print-process.mp4",
		Tags:   []string{"Быстрый срок", "Малый тираж", "Высокое разрешение"},
		Params: WorkParams{Deadline: "от 2 дней", Quantity: "1–500 экз.", Unit: "за экземпляр", Format: "A4, A5, A6"},
	},
	{
		ID: 2, Name: "Офсетная печать", PriceRub: 15000,
		Description: "Офсетная печать — стандарт для крупных тиражей от 1000 экземпляров. Обеспечивает стабильное воспроизведение цвета и высокую скорость производства. Подходит для книг, журналов и каталогов массового распространения.",
		ImageKey:    "print-offset.jpg", VideoKey: "",
		Tags:   []string{"Крупный тираж", "Низкая цена за экз.", "Стабильный цвет"},
		Params: WorkParams{Deadline: "от 10 дней", Quantity: "от 1000 экз.", Unit: "за тираж", Format: "A4, A5, 70×100"},
	},
	{
		ID: 3, Name: "Мягкий переплёт", PriceRub: 800,
		Description: "Скрепление блока на термоклей или скобу с мягкой обложкой. Экономичный вариант для брошюр, учебных пособий и малотиражных изданий. Обложка печатается на плотной бумаге с возможностью ламинации.",
		ImageKey:    "soft-cover.jpg", VideoKey: "",
		Tags:   []string{"Экономично", "Лёгкий вес", "Быстро"},
		Params: WorkParams{Deadline: "от 1 дня", Quantity: "от 10 экз.", Unit: "за экземпляр", Format: "A4, A5, A6"},
	},
	{
		ID: 4, Name: "Твёрдый переплёт", PriceRub: 2500,
		Description: "Переплёт в твёрдую обложку с тиснением или суперобложкой. Обеспечивает долговечность и представительный вид издания. Применяется для деловых книг, монографий и подарочных изданий.",
		ImageKey:    "hard-cover.jpg", VideoKey: "",
		Tags:   []string{"Долговечность", "Премиум вид", "Тиснение"},
		Params: WorkParams{Deadline: "от 5 дней", Quantity: "от 50 экз.", Unit: "за экземпляр", Format: "A4, A5, 60×84"},
	},
	{
		ID: 5, Name: "Вёрстка", PriceRub: 3000,
		Description: "Профессиональная вёрстка в Adobe InDesign по издательским стандартам. Включает расстановку переносов, работу с иллюстрациями, создание оглавления и колонтитулов. Результат — готовый PDF для передачи в печать.",
		ImageKey:    "layout.jpg", VideoKey: "",
		Tags:   []string{"InDesign", "PDF для печати", "По ГОСТ"},
		Params: WorkParams{Deadline: "от 3 дней", Quantity: "любой объём", Unit: "за полосу", Format: "любой"},
	},
	{
		ID: 6, Name: "Корректура", PriceRub: 1500,
		Description: "Вычитка текста на орфографию, пунктуацию и стилистику. Корректор работает с оригинал-макетом и возвращает исправленный файл с пометками. Обязательный этап перед сдачей рукописи в производство.",
		ImageKey:    "proofreading.jpg", VideoKey: "layout-demo.mp4",
		Tags:   []string{"Орфография", "Пунктуация", "Стилистика"},
		Params: WorkParams{Deadline: "от 2 дней", Quantity: "любой объём", Unit: "за 1000 знаков", Format: "Word / PDF"},
	},
	{
		ID: 7, Name: "Дизайн обложки", PriceRub: 4000,
		Description: "Разработка уникального дизайна обложки с учётом жанра и целевой аудитории книги. Включает 3 концепции на выбор, правки и подготовку финального файла для печати в CMYK.",
		ImageKey:    "cover-design.jpg", VideoKey: "",
		Tags:   []string{"3 концепции", "CMYK", "Уникальный стиль"},
		Params: WorkParams{Deadline: "от 5 дней", Quantity: "1 обложка", Unit: "за проект", Format: "любой формат"},
	},
	{
		ID: 8, Name: "Присвоение ISBN", PriceRub: 1000,
		Description: "Оформление и присвоение международного стандартного книжного номера ISBN и индекса ББК. Услуга необходима для реализации книги через магазины и библиотеки. Занимает до 14 рабочих дней через Российскую книжную палату.",
		ImageKey:    "isbn.jpg", VideoKey: "",
		Tags:   []string{"Официально", "Для продажи", "Библиотеки"},
		Params: WorkParams{Deadline: "до 14 дней", Quantity: "1 издание", Unit: "за издание", Format: "—"},
	},
}

var currentOrder = PublishingOrder{
	ID: 1,
	Items: []OrderItem{
		{WorkID: 1, WorkName: "Цифровая печать", PriceRub: 5000, Quantity: 1, ImageKey: "print-digital.jpg"},
		{WorkID: 5, WorkName: "Вёрстка", PriceRub: 3000, Quantity: 1, ImageKey: "layout.jpg"},
	},
	ResultText: "Ориентировочная стоимость: 8 000 ₽",
}

const minioURL = "http://localhost:9000/publishing-media"

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

	log.Println("Сервер запущен: http://localhost:8080")
	log.Println("Minio консоль: http://localhost:9001")
	http.ListenAndServe(":8080", r)
}

func worksListHandler(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(r.URL.Query().Get("query"))

	filtered := []Work{}
	for _, work := range works {
		if query == "" || strings.Contains(strings.ToLower(work.Name), query) {
			filtered = append(filtered, work)
		}
	}

	data := struct {
		Works          []Work
		Query          string
		MinioURL       string
		CartCount      int
		CurrentOrderID int
	}{
		Works:          filtered,
		Query:          query,
		MinioURL:       minioURL,
		CartCount:      len(currentOrder.Items),
		CurrentOrderID: currentOrder.ID,
	}

	tmpl := template.Must(template.ParseFiles("templates/works_list.html"))
	tmpl.Execute(w, data)
}

func workDetailHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)

	var found *Work
	for i := range works {
		if works[i].ID == id {
			found = &works[i]
			break
		}
	}

	if found == nil {
		http.NotFound(w, r)
		return
	}

	data := struct {
		Work     Work
		MinioURL string
	}{
		Work:     *found,
		MinioURL: minioURL,
	}

	tmpl := template.Must(template.ParseFiles("templates/work_detail.html"))
	tmpl.Execute(w, data)
}

func orderDetailHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)

	if currentOrder.ID != id {
		http.NotFound(w, r)
		return
	}

	data := struct {
		Order    PublishingOrder
		MinioURL string
	}{
		Order:    currentOrder,
		MinioURL: minioURL,
	}

	tmpl := template.Must(template.ParseFiles("templates/order_detail.html"))
	tmpl.Execute(w, data)
}
