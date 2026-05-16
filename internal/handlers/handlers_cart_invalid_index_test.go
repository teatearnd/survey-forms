package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"example.com/m/internal/auth"
	"example.com/m/internal/cache"
	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
)

func TestRemoveFromCartInvalidIndexParsing(t *testing.T) {
	initAuthForTest(t)
	redisCache := setupSimpleCartCache(t)
	defHandler := &Handler{Cache: redisCache}

	userID := uuid.New()
	token := createTestToken(t, auth.AccessClaims{Email: "u@e.com", UserID: userID.String(), Role: "user"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/cart/items/abc", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = addURLParam(req, "index", "abc")

	handler := auth.AuthMiddleware(http.HandlerFunc(defHandler.RemoveFromCart))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-integer index, got %d", rec.Code)
	}
}

func TestRemoveFromCartNegativeIndex(t *testing.T) {
	initAuthForTest(t)
	redisCache := setupSimpleCartCache(t)
	defHandler := &Handler{Cache: redisCache}

	userID := uuid.New()
	token := createTestToken(t, auth.AccessClaims{Email: "u@e.com", UserID: userID.String(), Role: "user"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/cart/items/-1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = addURLParam(req, "index", "-1")

	handler := auth.AuthMiddleware(http.HandlerFunc(defHandler.RemoveFromCart))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for negative index, got %d", rec.Code)
	}
}

func TestRemoveFromCartOutOfRangeIndexReturnsServerError(t *testing.T) {
	initAuthForTest(t)
	// create real miniredis and then close after adding one item
	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	redisCache := cache.NewRedisCache(srv.Addr(), "", 0)
	if err := redisCache.Ping(); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
	// add one item manually
	if err := redisCache.AddItem("user-x", `{"survey_id":"`+uuid.NewString()+`","question_id":"`+uuid.NewString()+`"}`); err != nil {
		srv.Close()
		t.Fatalf("add failed: %v", err)
	}
	// leave server closed to simulate out-of-range LSET on later call
	srv.Close()

	defHandler := &Handler{Cache: redisCache}
	userID := uuid.New()
	token := createTestToken(t, auth.AccessClaims{Email: "u@e.com", UserID: userID.String(), Role: "user"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/cart/items/5", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = addURLParam(req, "index", "5")

	handler := auth.AuthMiddleware(http.HandlerFunc(defHandler.RemoveFromCart))
	handler.ServeHTTP(rec, req)

	// because redis is closed, the handler should return 500 for backend failure
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for redis failure on removal, got %d", rec.Code)
	}
}
