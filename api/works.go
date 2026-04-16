package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"publishing-backend/db"
	"publishing-backend/models"

	"github.com/go-chi/chi/v5"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	minioBucket   = "publishing-media"
	minioEndpoint = "localhost:9000"
	minioAccess   = "minioadmin"
	minioSecret   = "minioadmin123"
	minioBaseURL  = "http://localhost:9000/publishing-media"
	worksRedisKey = "api:works:all"
)

func newMinioClient() (*minio.Client, error) {
	return minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioAccess, minioSecret, ""),
		Secure: false,
	})
}

func generateFileName(original string) string {
	ext := filepath.Ext(original)
	return fmt.Sprintf("file-%d%s", time.Now().UnixNano(), ext)
}

type WorkResponse struct {
	ID            uint     `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	PriceRub      int      `json:"price_rub"`
	WorkType      string   `json:"work_type"`
	Unit          string   `json:"unit"`
	ImageURL      string   `json:"image_url,omitempty"`
	VideoURL      string   `json:"video_url,omitempty"`
	Tags          []string `json:"tags"`
	ParamDeadline string   `json:"param_deadline"`
	ParamQuantity string   `json:"param_quantity"`
	ParamUnit     string   `json:"param_unit"`
	ParamFormat   string   `json:"param_format"`
}

func toWorkResponse(m models.Work) WorkResponse {
	imageURL := ""
	if m.ImageKey != nil && *m.ImageKey != "" {
		imageURL = minioBaseURL + "/" + *m.ImageKey
	}
	videoURL := ""
	if m.VideoKey != nil && *m.VideoKey != "" {
		videoURL = minioBaseURL + "/" + *m.VideoKey
	}
	tags := []string{}
	if m.Tag1 != "" {
		tags = append(tags, m.Tag1)
	}
	if m.Tag2 != "" {
		tags = append(tags, m.Tag2)
	}
	if m.Tag3 != "" {
		tags = append(tags, m.Tag3)
	}
	return WorkResponse{
		ID:            m.ID,
		Name:          m.Name,
		Description:   m.Description,
		PriceRub:      m.PriceRub,
		WorkType:      m.WorkType,
		Unit:          m.Unit,
		ImageURL:      imageURL,
		VideoURL:      videoURL,
		Tags:          tags,
		ParamDeadline: m.ParamDeadline,
		ParamQuantity: m.ParamQuantity,
		ParamUnit:     m.ParamUnit,
		ParamFormat:   m.ParamFormat,
	}
}

// GetWorks godoc
// @Summary Список услуг
// @Description Возвращает активные услуги. Поддерживает фильтры. Без фильтров кешируется в Redis 60s.
// @Tags works
// @Produce json
// @Param query query string false "Поиск по названию"
// @Param minPrice query int false "Минимальная цена"
// @Param maxPrice query int false "Максимальная цена"
// @Param workType query string false "Тип работы"
// @Success 200 {array} WorkResponse
// @Security CookieAuth
// @Router /works [get]
func GetWorks(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(r.URL.Query().Get("query"))
	minPrice := r.URL.Query().Get("minPrice")
	maxPrice := r.URL.Query().Get("maxPrice")
	workType := r.URL.Query().Get("workType")

	ctx := context.Background()

	// Кешируем только если нет ни одного фильтра
	hasFilters := query != "" || minPrice != "" || maxPrice != "" || workType != ""
	if !hasFilters {
		if cached, err := db.Redis.Get(ctx, worksRedisKey).Result(); err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(http.StatusOK)
			if _, err := fmt.Fprint(w, cached); err != nil {
				http.Error(w, "write error", http.StatusInternalServerError)
			}
			return
		}
	}

	tx := db.DB.Where("status = ?", models.WorkStatusActive)

	if query != "" {
		tx = tx.Where("LOWER(name) LIKE ?", "%"+query+"%")
	}
	if minPrice != "" {
		if v, err := strconv.Atoi(minPrice); err == nil {
			tx = tx.Where("price_rub >= ?", v)
		}
	}
	if maxPrice != "" {
		if v, err := strconv.Atoi(maxPrice); err == nil {
			tx = tx.Where("price_rub <= ?", v)
		}
	}
	if workType != "" {
		tx = tx.Where("work_type = ?", workType)
	}

	var dbWorks []models.Work
	tx.Find(&dbWorks)

	result := make([]WorkResponse, 0, len(dbWorks))
	for _, item := range dbWorks {
		result = append(result, toWorkResponse(item))
	}

	// Кешируем только чистый список без фильтров
	if !hasFilters {
		if jsonBytes, err := json.Marshal(result); err == nil {
			_ = db.Redis.Set(ctx, worksRedisKey, string(jsonBytes), 60*time.Second).Err()
		}
		w.Header().Set("X-Cache", "MISS")
	}

	writeJSON(w, http.StatusOK, result)
}

// GetWork godoc
// @Summary     Одна услуга
// @Description Возвращает услугу по ID
// @Tags        works
// @Produce     json
// @Param       id path int true "ID услуги"
// @Success     200 {object} WorkResponse
// @Failure     404 {object} map[string]string
// @Router      /works/{id} [get]
func GetWork(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var work models.Work
	if db.DB.Where("id = ? AND status = ?", id, models.WorkStatusActive).First(&work).Error != nil {
		writeError(w, http.StatusNotFound, "услуга не найдена")
		return
	}

	writeJSON(w, http.StatusOK, toWorkResponse(work))
}

// CreateWork godoc
// @Summary     Создать услугу
// @Description Создаёт новую услугу. Принимает multipart/form-data с файлами image и video.
// @Tags        works
// @Accept      mpfd
// @Produce     json
// @Param       name          formData string true  "Название"
// @Param       price_rub     formData int    true  "Цена в рублях"
// @Param       description   formData string false "Описание"
// @Param       work_type     formData string false "Тип работы"
// @Param       unit          formData string false "Единица"
// @Param       param_deadline formData string false "Срок"
// @Param       param_quantity formData string false "Количество"
// @Param       param_unit    formData string false "Единица параметра"
// @Param       param_format  formData string false "Формат"
// @Param       tag1          formData string false "Тег 1"
// @Param       tag2          formData string false "Тег 2"
// @Param       tag3          formData string false "Тег 3"
// @Param       image         formData file   false "Изображение"
// @Param       video         formData file   false "Видео"
// @Success     201 {object} WorkResponse
// @Failure     400 {object} map[string]string
// @Router      /works [post]
func CreateWork(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "ошибка разбора формы")
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		writeError(w, http.StatusBadRequest, "поле name обязательно")
		return
	}

	var priceRub int
	if _, err := fmt.Sscan(r.FormValue("price_rub"), &priceRub); err != nil {
		writeError(w, http.StatusBadRequest, "price_rub должен быть числом")
		return
	}

	work := models.Work{
		Name:          name,
		Description:   r.FormValue("description"),
		PriceRub:      priceRub,
		WorkType:      r.FormValue("work_type"),
		Unit:          r.FormValue("unit"),
		ParamDeadline: r.FormValue("param_deadline"),
		ParamQuantity: r.FormValue("param_quantity"),
		ParamUnit:     r.FormValue("param_unit"),
		ParamFormat:   r.FormValue("param_format"),
		Tag1:          r.FormValue("tag1"),
		Tag2:          r.FormValue("tag2"),
		Tag3:          r.FormValue("tag3"),
		Status:        models.WorkStatusActive,
	}

	mc, minioErr := newMinioClient()

	if imageFile, imageHeader, err := r.FormFile("image"); err == nil {
		defer imageFile.Close() //nolint:errcheck
		if minioErr == nil {
			key := generateFileName(imageHeader.Filename)
			if _, uploadErr := mc.PutObject(
				context.Background(), minioBucket, key,
				imageFile, imageHeader.Size,
				minio.PutObjectOptions{ContentType: imageHeader.Header.Get("Content-Type")},
			); uploadErr == nil {
				work.ImageKey = &key
			}
		}
	}

	if videoFile, videoHeader, err := r.FormFile("video"); err == nil {
		defer videoFile.Close() //nolint:errcheck
		if minioErr == nil {
			key := generateFileName(videoHeader.Filename)
			if _, uploadErr := mc.PutObject(
				context.Background(), minioBucket, key,
				videoFile, videoHeader.Size,
				minio.PutObjectOptions{ContentType: videoHeader.Header.Get("Content-Type")},
			); uploadErr == nil {
				work.VideoKey = &key
			}
		}
	}

	if err := db.DB.Create(&work).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка создания услуги")
		return
	}

	db.Redis.Del(context.Background(), worksRedisKey) //nolint:errcheck

	writeJSON(w, http.StatusCreated, toWorkResponse(work))
}
