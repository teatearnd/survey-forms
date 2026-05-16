package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"example.com/m/internal/auth"
	"example.com/m/internal/cache"
	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
)

func setupSimpleCartCache(t *testing.T) *cache.RedisCache {
	t.Helper()
	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	t.Cleanup(srv.Close)

	redisCache := cache.NewRedisCache(srv.Addr(), "", 0)
	if err := redisCache.Ping(); err != nil {
		t.Fatalf("failed to ping redis: %v", err)
	}
	return redisCache
}

func TestAddToCartMalformedJSON(t *testing.T) {
	initAuthForTest(t)
	redisCache := setupSimpleCartCache(t)
	defHandler := &Handler{Cache: redisCache}

	userID := uuid.New()
	token := createTestToken(t, auth.AccessClaims{Email: "u@e.com", UserID: userID.String(), Role: "user"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cart/items", bytes.NewReader([]byte("{invalid-json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	handler := auth.AuthMiddleware(http.HandlerFunc(defHandler.AddToCart))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed JSON, got %d", rec.Code)
	}
}

func TestAddToCartUnknownFields(t *testing.T) {
	initAuthForTest(t)
	redisCache := setupSimpleCartCache(t)
	defHandler := &Handler{Cache: redisCache}

	userID := uuid.New()
	token := createTestToken(t, auth.AccessClaims{Email: "u@e.com", UserID: userID.String(), Role: "user"})

	// include unknown field 'unknown_field' inside item
	payload := []byte(`{"item":{"survey_id":"` + uuid.NewString() + `","question_id":"` + uuid.NewString() + `","unknown_field":"x"}}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cart/items", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	handler := auth.AuthMiddleware(http.HandlerFunc(defHandler.AddToCart))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown fields, got %d", rec.Code)
	}
}
