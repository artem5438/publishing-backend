package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"publishing-backend/api"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
)

func newTestRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(api.AuthMiddleware)

	okHandler := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	r.Get("/api/services", okHandler)
	r.Get("/api/services/{id}", okHandler)
	r.Post("/api/services", okHandler)
	r.Get("/api/publishing-orders", okHandler)

	return r
}

func createBearerToken(t *testing.T, secret string) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":    42,
		"user_login": "tester",
		"user_role":  "creator",
		"exp":        time.Now().Add(time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return tokenString
}

func TestAuthMiddleware_AllowsPublicServicesWithoutToken(t *testing.T) {
	router := newTestRouter()

	testCases := []struct {
		name string
		path string
	}{
		{name: "services list", path: "/api/services"},
		{name: "service card", path: "/api/services/123"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
			}
		})
	}
}

func TestAuthMiddleware_RejectsProtectedEndpointsWithoutToken(t *testing.T) {
	router := newTestRouter()

	testCases := []struct {
		name   string
		method string
		path   string
	}{
		{name: "create service", method: http.MethodPost, path: "/api/services"},
		{name: "get orders", method: http.MethodGet, path: "/api/publishing-orders"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
			}
		})
	}
}

func TestAuthMiddleware_AllowsProtectedEndpointsWithValidJWT(t *testing.T) {
	const secret = "test-secret"
	t.Setenv("JWT_SECRET", secret)

	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/publishing-orders", nil)
	req.Header.Set("Authorization", "Bearer "+createBearerToken(t, secret))
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
