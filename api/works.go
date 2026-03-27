package api

import (
	"context"
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

// ─── Minio ───────────────────────────────────────────────────────────────────

const (
	minioBucket   = "publishing-media"
	minioEndpoint = "localhost:9000"
	minioAccess   = "minioadmin"
	minioSecret   = "minioadmin"
	minioBaseURL  = "http://localhost:9000/publishing-media"
)

func newMinioClient() (*minio.Client, error) {
	return minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioAccess, minioSecret, ""),
		Secure: false,
	})
}

// generateFileName генерирует уникальное имя файла на латинице
func generateFileName(original string) string {
	ext := filepath.Ext(original)
	return fmt.Sprintf("file-%d%s", time.Now().UnixNano(), ext)
}

// ─── Response-структура услуги ────────────────────────────────────────────────

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

// ─── GET /api/works ───────────────────────────────────────────────────────────

func GetWorks(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(r.URL.Query().Get("query"))

	var dbWorks []models.Work
	tx := db.DB.Where("status = ?", models.WorkStatusActive)
	if query != "" {
		tx = tx.Where("LOWER(name) LIKE ?", "%"+query+"%")
	}
	tx.Find(&dbWorks)

	result := make([]WorkResponse, 0, len(dbWorks))
	for _, item := range dbWorks {
		result = append(result, toWorkResponse(item))
	}

	writeJSON(w, http.StatusOK, result)
}

// ─── GET /api/works/{id} ──────────────────────────────────────────────────────

func GetWork(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var work models.Work
	res := db.DB.Where("id = ? AND status = ?", id, models.WorkStatusActive).First(&work)
	if res.Error != nil {
		writeError(w, http.StatusNotFound, "услуга не найдена")
		return
	}

	writeJSON(w, http.StatusOK, toWorkResponse(work))
}

// ─── POST /api/works ──────────────────────────────────────────────────────────

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

	priceRub, err := strconv.Atoi(r.FormValue("price_rub"))
	if err != nil {
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

	// Загрузка изображения
	if imageFile, imageHeader, err := r.FormFile("image"); err == nil {
		defer imageFile.Close() //nolint:errcheck
		if minioErr == nil {
			key := generateFileName(imageHeader.Filename)
			_, uploadErr := mc.PutObject(
				context.Background(), minioBucket, key,
				imageFile, imageHeader.Size,
				minio.PutObjectOptions{ContentType: imageHeader.Header.Get("Content-Type")},
			)
			if uploadErr == nil {
				work.ImageKey = &key
			}
		}
	}

	// Загрузка видео
	if videoFile, videoHeader, err := r.FormFile("video"); err == nil {
		defer videoFile.Close() //nolint:errcheck
		if minioErr == nil {
			key := generateFileName(videoHeader.Filename)
			_, uploadErr := mc.PutObject(
				context.Background(), minioBucket, key,
				videoFile, videoHeader.Size,
				minio.PutObjectOptions{ContentType: videoHeader.Header.Get("Content-Type")},
			)
			if uploadErr == nil {
				work.VideoKey = &key
			}
		}
	}

	if res := db.DB.Create(&work); res.Error != nil {
		writeError(w, http.StatusInternalServerError, "ошибка создания услуги")
		return
	}

	writeJSON(w, http.StatusCreated, toWorkResponse(work))
}
