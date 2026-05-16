package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"example.com/m/internal/auth"
	"example.com/m/internal/cache"
	"example.com/m/internal/dto"
	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
)

func TestGetCartPaginationEdgeCasesAndEmpty(t *testing.T) {
	initAuthForTest(t)
	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer srv.Close()

	redisCache := cache.NewRedisCache(srv.Addr(), "", 0)
	if err := redisCache.Ping(); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
	defHandler := &Handler{Cache: redisCache}

	userID := uuid.New()
	token := createTestToken(t, auth.AccessClaims{Email: "u@e.com", UserID: userID.String(), Role: "user"})

	// ensure empty cart returns empty array
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/cart", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler := auth.AuthMiddleware(http.HandlerFunc(defHandler.GetCart))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for empty cart, got %d", rec.Code)
	}
	var resp dto.ResponseCart
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode empty cart: %v", err)
	}
	if len(resp.Cart) != 0 {
		t.Fatalf("expected 0 items for empty cart, got %d", len(resp.Cart))
	}

	// add two items
	for _, note := range []string{"a", "b"} {
		if err := redisCache.AddItem(userID.String(), `{"survey_id":"`+uuid.NewString()+`","question_id":"`+uuid.NewString()+`","note":"`+note+`"}`); err != nil {
			t.Fatalf("add failed: %v", err)
		}
	}

	// limit=0 should fallback to default (50) and return both
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/cart?limit=0", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler = auth.AuthMiddleware(http.HandlerFunc(defHandler.GetCart))
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for limit=0 fallback, got %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode cart: %v", err)
	}
	if len(resp.Cart) != 2 {
		t.Fatalf("expected 2 items with limit=0 fallback, got %d", len(resp.Cart))
	}

	// limit > 1000 fallback
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/cart?limit=1001", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler = auth.AuthMiddleware(http.HandlerFunc(defHandler.GetCart))
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for limit>1000 fallback, got %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode cart: %v", err)
	}
	if len(resp.Cart) != 2 {
		t.Fatalf("expected 2 items with limit>1000 fallback, got %d", len(resp.Cart))
	}

	// large offset beyond length -> empty
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/cart?offset=1000", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler = auth.AuthMiddleware(http.HandlerFunc(defHandler.GetCart))
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for large offset, got %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode cart: %v", err)
	}
	if len(resp.Cart) != 0 {
		t.Fatalf("expected 0 items for large offset, got %d", len(resp.Cart))
	}
}
