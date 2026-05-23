//go:build unit || integration

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"publishing-backend/db"
	"publishing-backend/models"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	integrationEnvOnce sync.Once
	integrationEnvErr  error
	integrationGormDB  *gorm.DB
	integrationRedis   *redis.Client
	integrationStop    func()
)

func TestMain(m *testing.M) {
	InitLogger()
	_ = os.Setenv("JWT_SECRET", "test-jwt-secret")
	code := m.Run()
	if integrationStop != nil {
		integrationStop()
	}
	os.Exit(code)
}

// поднимаем 2 контейнера и получаем строки подключения
func startIntegrationEnv() error {
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
	)
	if err != nil {
		return err
	}

	redisContainer, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		_ = testcontainers.TerminateContainer(pgContainer)
		return err
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return err
	}

	redisURI, err := redisContainer.ConnectionString(ctx)
	if err != nil {
		return err
	}

	opts, err := redis.ParseURL(redisURI)
	if err != nil {
		return err
	}

	testDB, err := openGormWithRetry(connStr, 20, 500*time.Millisecond)
	if err != nil {
		return err
	}
	if err := testDB.AutoMigrate(
		&models.User{},
		&models.Work{},
		&models.PublishingOrder{},
		&models.OrderWork{},
	); err != nil {
		return err
	}

	integrationGormDB = testDB
	integrationRedis = redis.NewClient(opts)
	integrationStop = func() {
		if integrationRedis != nil {
			_ = integrationRedis.Close()
		}
		_ = testcontainers.TerminateContainer(pgContainer)
		_ = testcontainers.TerminateContainer(redisContainer)
	}
	return nil
}

// открываем соединение с базой данных с помощью GORM
func openGormWithRetry(connStr string, attempts int, delay time.Duration) (*gorm.DB, error) {
	var lastErr error
	for i := 0; i < attempts; i++ {
		testDB, err := gorm.Open(gormpostgres.Open(connStr), &gorm.Config{})
		if err == nil {
			sqlDB, err := testDB.DB()
			if err == nil {
				err = sqlDB.Ping()
			}
			if err == nil {
				return testDB, nil
			}
			lastErr = err
		} else {
			lastErr = err
		}
		time.Sleep(delay)
	}
	return nil, lastErr
}

// очищаем таблицы
func resetIntegrationTables(t *testing.T) {
	t.Helper()
	require.NoError(t, integrationGormDB.Exec(
		`TRUNCATE TABLE order_works, publishing_orders, works, users RESTART IDENTITY CASCADE`,
	).Error)
}

// настраиваем тестовое окружение
func setupTestEnv(t *testing.T) *gorm.DB {
	t.Helper()

	integrationEnvOnce.Do(func() {
		integrationEnvErr = startIntegrationEnv()
	})
	require.NoError(t, integrationEnvErr)

	resetIntegrationTables(t)

	oldDB := db.DB
	oldRedis := db.Redis
	db.DB = integrationGormDB
	db.Redis = integrationRedis

	t.Cleanup(func() {
		db.DB = oldDB
		db.Redis = oldRedis
	})

	return integrationGormDB
}

// монтируем маршруты API
func mountAPIRoutes(r chi.Router) {
	r.Route("/api", func(r chi.Router) {
		r.Use(AuthMiddleware)

		r.Post("/auth/register", Register)
		r.Post("/auth/login", Login)
		r.Post("/auth/logout", Logout)

		r.Group(func(r chi.Router) {
			r.Use(RequireAuth)

			r.Get("/works", GetWorks)
			r.Get("/works/{id}", GetWork)
			r.Post("/works", CreateWork)
			r.Put("/works/{id}", UpdateWork)
			r.Delete("/works/{id}", DeleteWork)
			r.Get("/auth/me", GetMe)
			r.Put("/auth/profile", UpdateProfile)

			r.Get("/publishing-orders", GetOrders)
			r.Get("/publishing-orders/cart", GetCart)
			r.Get("/publishing-orders/{id}", GetOrder)
			r.Put("/publishing-orders/{id}", UpdateOrder)
			r.Put("/publishing-orders/{id}/submit", SubmitOrder)
			r.Delete("/publishing-orders/{id}", DeleteOrder)

			r.Post("/publishing-orders/cart/works", AddWorkToOrder)
			r.Put("/publishing-orders/{id}/works/{workId}", UpdateOrderWork)
			r.Delete("/publishing-orders/{id}/works/{workId}", RemoveWorkFromOrder)

			r.Group(func(r chi.Router) {
				r.Use(RequireModerator)
				r.Put("/publishing-orders/{id}/moderate", ModerateOrder)
			})
		})
	})
}

// создаем тестовый сервер
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	r := chi.NewRouter()
	r.Use(LoggingMiddleware)
	mountAPIRoutes(r)
	return httptest.NewServer(r)
}

// структура для хранения ответа от сервера
type httpResponse struct {
	Status  int
	Body    []byte
	Cookies []*http.Cookie
}

// делаем запрос к серверу
func makeRequest(t *testing.T, server *httptest.Server, method, path string, body []byte, cookies []*http.Cookie) httpResponse {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, server.URL+path, bodyReader)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return httpResponse{
		Status:  resp.StatusCode,
		Body:    data,
		Cookies: resp.Cookies(),
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}

func parseJSONBody(t *testing.T, data []byte, dest any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(data, dest))
}

func mergeCookies(existing []*http.Cookie, newOnes []*http.Cookie) []*http.Cookie {
	byName := make(map[string]*http.Cookie)
	for _, c := range existing {
		byName[c.Name] = c
	}
	for _, c := range newOnes {
		byName[c.Name] = c
	}
	out := make([]*http.Cookie, 0, len(byName))
	for _, c := range byName {
		out = append(out, c)
	}
	return out
}

func uniqueLogin(prefix string) string {
	return prefix + "_" + time.Now().Format("150405.000000")
}
