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

func TestHandlersWhenRedisUnavailable(t *testing.T) {
	initAuthForTest(t)
	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	addr := srv.Addr()

	rc := cache.NewRedisCache(addr, "", 0)
	if err := rc.Ping(); err != nil {
		t.Fatalf("ping failed: %v", err)
	}

	// close server to simulate failure
	srv.Close()

	handler := &Handler{Cache: rc}
	userID := uuid.New()
	token := createTestToken(t, auth.AccessClaims{Email: "u@e.com", UserID: userID.String(), Role: "user"})

	// AddToCart should return 500 when redis down — send valid payload so decoding doesn't fail
	rec := httptest.NewRecorder()
	payload := []byte(`{"item":{"survey_id":"` + uuid.NewString() + `","question_id":"` + uuid.NewString() + `"}}`)
	req := httptest.NewRequest(http.MethodPost, "/cart/items", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	h := auth.AuthMiddleware(http.HandlerFunc(handler.AddToCart))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for AddToCart when redis down, got %d", rec.Code)
	}

	// GetCart should return 500
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/cart", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	h = auth.AuthMiddleware(http.HandlerFunc(handler.GetCart))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for GetCart when redis down, got %d", rec.Code)
	}

	// RemoveFromCart should return 500 (use index 0)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/cart/items/0", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = addURLParam(req, "index", "0")
	h = auth.AuthMiddleware(http.HandlerFunc(handler.RemoveFromCart))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for RemoveFromCart when redis down, got %d", rec.Code)
	}

	// ClearCart should return 500
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/cart", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	h = auth.AuthMiddleware(http.HandlerFunc(handler.ClearCart))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for ClearCart when redis down, got %d", rec.Code)
	}
}
