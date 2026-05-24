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

	"publishing-backend/config"
	"publishing-backend/db"
	"publishing-backend/models"

	"github.com/go-chi/chi/v5"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	worksRedisKey = "api:works:all"
	worksCacheTTL = 60 * time.Second
)

var (
	minioBucket    = config.MinioBucket()
	minioEndpoint  = config.MinioEndpoint()
	minioAccessKey = config.MinioAccessKey()
	minioSecretKey = config.MinioSecretKey()
	minioPublicURL = config.MinioPublicURL()
	minioUseSSL    = config.MinioUseSSL()
)

func newMinioClient() (*minio.Client, error) {
	return minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioAccessKey, minioSecretKey, ""),
		Secure: minioUseSSL,
	})
}

func generateFileName(original string) string {
	ext := filepath.Ext(original)
	return fmt.Sprintf("file-%d%s", time.Now().UnixNano(), ext)
}

func uploadWorkFile(r *http.Request, mc *minio.Client, fieldName string) *string {
	if mc == nil {
		return nil
	}
	file, header, err := r.FormFile(fieldName)
	if err != nil {
		return nil
	}
	defer file.Close() //nolint:errcheck

	key := generateFileName(header.Filename)
	if _, uploadErr := mc.PutObject(
		context.Background(), minioBucket, key,
		file, header.Size,
		minio.PutObjectOptions{ContentType: header.Header.Get("Content-Type")},
	); uploadErr != nil {
		return nil
	}
	return &key
}

func formHasField(r *http.Request, key string) bool {
	if r.MultipartForm == nil {
		return false
	}
	_, ok := r.MultipartForm.Value[key]
	return ok
}

func applyWorkTextFields(r *http.Request, work *models.Work, partial bool) error {
	if !partial {
		work.Description = r.FormValue("description")
		work.WorkType = r.FormValue("work_type")
		work.Unit = r.FormValue("unit")
		work.ParamDeadline = r.FormValue("param_deadline")
		work.ParamQuantity = r.FormValue("param_quantity")
		work.ParamUnit = r.FormValue("param_unit")
		work.ParamFormat = r.FormValue("param_format")
		work.Tag1 = r.FormValue("tag1")
		work.Tag2 = r.FormValue("tag2")
		work.Tag3 = r.FormValue("tag3")
		return nil
	}

	if formHasField(r, "name") {
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			return fmt.Errorf("поле name не может быть пустым")
		}
		work.Name = name
	}
	if v := strings.TrimSpace(r.FormValue("price_rub")); v != "" {
		var priceRub int
		if _, err := fmt.Sscan(v, &priceRub); err != nil {
			return fmt.Errorf("price_rub должен быть числом")
		}
		work.PriceRub = priceRub
	}
	if v := strings.TrimSpace(r.FormValue("description")); v != "" {
		work.Description = v
	}
	if v := strings.TrimSpace(r.FormValue("work_type")); v != "" {
		work.WorkType = v
	}
	if v := strings.TrimSpace(r.FormValue("unit")); v != "" {
		work.Unit = v
	}
	if v := strings.TrimSpace(r.FormValue("param_deadline")); v != "" {
		work.ParamDeadline = v
	}
	if v := strings.TrimSpace(r.FormValue("param_quantity")); v != "" {
		work.ParamQuantity = v
	}
	if v := strings.TrimSpace(r.FormValue("param_unit")); v != "" {
		work.ParamUnit = v
	}
	if v := strings.TrimSpace(r.FormValue("param_format")); v != "" {
		work.ParamFormat = v
	}
	if v := strings.TrimSpace(r.FormValue("tag1")); v != "" {
		work.Tag1 = v
	}
	if v := strings.TrimSpace(r.FormValue("tag2")); v != "" {
		work.Tag2 = v
	}
	if v := strings.TrimSpace(r.FormValue("tag3")); v != "" {
		work.Tag3 = v
	}
	return nil
}

// парсинг булевых значений из HTML-форм
func formTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func attachWorkMedia(r *http.Request, work *models.Work, mc *minio.Client) {
	if key := uploadWorkFile(r, mc, "image"); key != nil {
		work.ImageKey = key
	}
	if key := uploadWorkFile(r, mc, "video"); key != nil {
		work.VideoKey = key
	}
}

// applyWorkMediaOnUpdate обрабатывает remove_image/remove_video и опциональную замену файлов.
func applyWorkMediaOnUpdate(r *http.Request, work *models.Work, mc *minio.Client) {
	if formHasField(r, "remove_image") && formTruthy(r.FormValue("remove_image")) {
		work.ImageKey = nil
	}
	if formHasField(r, "remove_video") && formTruthy(r.FormValue("remove_video")) {
		work.VideoKey = nil
	}
	attachWorkMedia(r, work, mc)
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
		imageURL = minioPublicURL + "/" + *m.ImageKey
	}
	videoURL := ""
	if m.VideoKey != nil && *m.VideoKey != "" {
		videoURL = minioPublicURL + "/" + *m.VideoKey
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
// @Failure 401 {object} map[string]string
// @Security CookieAuth
// @Router /works [get]
func GetWorks(w http.ResponseWriter, r *http.Request) { // Получаем услуги из базы данных
	query := strings.ToLower(r.URL.Query().Get("query"))
	minPrice := r.URL.Query().Get("minPrice")
	maxPrice := r.URL.Query().Get("maxPrice")
	workType := r.URL.Query().Get("workType")

	ctx := context.Background()

	// Кешируем только если нет ни одного фильтра (Cache-Aside).
	hasFilters := query != "" || minPrice != "" || maxPrice != "" || workType != ""
	if !hasFilters {
		if cached, ok := CacheGet(ctx, worksRedisKey); ok {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(http.StatusOK)
			if _, err := fmt.Fprint(w, cached); err != nil {
				Logger.Error(EventCacheError,
					"event", EventCacheError,
					"cache_key", worksRedisKey,
					"result", "error",
					"error", err.Error(),
				)
				http.Error(w, "write error", http.StatusInternalServerError)
			}
			return
		}
	} else {
		w.Header().Set("X-Cache", "BYPASS")
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
	// Кешируем только если нет ни одного фильтра (Cache-Aside).
	if !hasFilters {
		if jsonBytes, err := json.Marshal(result); err == nil {
			CacheSet(ctx, worksRedisKey, string(jsonBytes), worksCacheTTL)
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
// @Failure     401 {object} map[string]string
// @Failure     404 {object} map[string]string
// @Security    CookieAuth
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

	mc, _ := newMinioClient()
	attachWorkMedia(r, &work, mc)

	// Создаем услугу в базе данных
	if err := db.DB.Create(&work).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка создания услуги")
		return
	}
	// Удаляем кэш при создании услуги (Delete on write).
	CacheInvalidate(context.Background(), worksRedisKey)

	writeJSON(w, http.StatusCreated, toWorkResponse(work))
}

// UpdateWork godoc
// @Summary     Редактировать услугу
// @Description Обновляет активную услугу. multipart/form-data, как CreateWork; пустые текстовые поля не меняют значение.
// @Tags        works
// @Accept      mpfd
// @Produce     json
// @Param       id            path int    true  "ID услуги"
// @Param       name          formData string false "Название"
// @Param       price_rub     formData int    false "Цена в рублях"
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
// @Param       remove_image  formData string false "true — убрать фото"
// @Param       remove_video  formData string false "true — убрать видео"
// @Success     200 {object} WorkResponse
// @Failure     400 {object} map[string]string
// @Failure     404 {object} map[string]string
// @Security    CookieAuth
// @Router      /works/{id} [put]
func UpdateWork(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "ошибка разбора формы")
		return
	}

	var work models.Work
	if db.DB.Where("id = ? AND status = ?", id, models.WorkStatusActive).First(&work).Error != nil {
		writeError(w, http.StatusNotFound, "услуга не найдена")
		return
	}

	if err := applyWorkTextFields(r, &work, true); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	mc, _ := newMinioClient()
	applyWorkMediaOnUpdate(r, &work, mc)

	if err := db.DB.Save(&work).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка обновления услуги")
		return
	}

	CacheInvalidate(context.Background(), worksRedisKey)
	writeJSON(w, http.StatusOK, toWorkResponse(work))
}

// DeleteWork godoc
// @Summary     Удалить услугу
// @Description Мягкое удаление: status = deleted
// @Tags        works
// @Produce     json
// @Param       id path int true "ID услуги"
// @Success     200 {object} map[string]string
// @Failure     404 {object} map[string]string
// @Security    CookieAuth
// @Router      /works/{id} [delete]
func DeleteWork(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var work models.Work
	if db.DB.Where("id = ? AND status = ?", id, models.WorkStatusActive).First(&work).Error != nil {
		writeError(w, http.StatusNotFound, "услуга не найдена")
		return
	}

	if err := db.DB.Model(&work).Update("status", models.WorkStatusDeleted).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка удаления услуги")
		return
	}

	CacheInvalidate(context.Background(), worksRedisKey)
	writeJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}
