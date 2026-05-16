package handlers

import (
	"bytes"
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

func TestCartItemSerializationRoundTrip(t *testing.T) {
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

	// add item without SubmissionID/AnswerID (should be omitted in stored JSON)
	noIDs := dto.RequestCartObject{Item: dto.CartItem{
		SurveyID:   uuid.New(),
		QuestionID: uuid.New(),
		Note:       "noids",
	}}
	b, _ := json.Marshal(noIDs)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cart/items", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	handler := auth.AuthMiddleware(http.HandlerFunc(defHandler.AddToCart))
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for add, got %d", rec.Code)
	}

	// add item with SubmissionID and AnswerID set
	subID := uuid.New()
	ansID := uuid.New()
	withIDs := dto.RequestCartObject{Item: dto.CartItem{
		SurveyID:     uuid.New(),
		QuestionID:   uuid.New(),
		SubmissionID: &subID,
		AnswerID:     &ansID,
		Note:         "withids",
	}}
	b2, _ := json.Marshal(withIDs)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/cart/items", bytes.NewReader(b2))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	handler = auth.AuthMiddleware(http.HandlerFunc(defHandler.AddToCart))
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for add with ids, got %d", rec.Code)
	}

	// inspect stored raw JSON via cache.GetItems
	items, err := redisCache.GetItems(userID.String(), 10, 0)
	if err != nil {
		t.Fatalf("GetItems failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items stored, got %d", len(items))
	}
	// newest first -> withIDs is first
	if !containsSubstring(items[0], "withids") {
		t.Fatalf("expected first item to be withids, got %s", items[0])
	}
	if !containsSubstring(items[1], "noids") {
		t.Fatalf("expected second item to be noids, got %s", items[1])
	}
	// ensure no 'submission_id' in stored JSON for the noids item
	if containsSubstring(items[1], "submission_id") {
		t.Fatalf("expected no submission_id in stored JSON for noids, got %s", items[1])
	}

	// GET /cart and verify round-trip types
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/cart", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler = auth.AuthMiddleware(http.HandlerFunc(defHandler.GetCart))
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for get cart, got %d", rec.Code)
	}
	var resp dto.ResponseCart
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode cart response: %v", err)
	}
	if len(resp.Cart) != 2 {
		t.Fatalf("expected 2 items from get cart, got %d", len(resp.Cart))
	}
	// newest first
	if resp.Cart[0].Note != "withids" || resp.Cart[1].Note != "noids" {
		t.Fatalf("unexpected notes order: %+v", resp.Cart)
	}
	if resp.Cart[1].SubmissionID != nil {
		t.Fatalf("expected nil SubmissionID for noids, got %v", resp.Cart[1].SubmissionID)
	}
	if resp.Cart[0].SubmissionID == nil || resp.Cart[0].AnswerID == nil {
		t.Fatalf("expected non-nil IDs for withids, got %+v", resp.Cart[0])
	}
}

func containsSubstring(s, sub string) bool {
	return bytes.Contains([]byte(s), []byte(sub))
}
