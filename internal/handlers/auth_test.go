package handlers_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"example.com/m/internal/handlers"
	"example.com/m/internal/repository"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=1")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := repository.InitSchema(db); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}
	return db
}

func newTestHandler(t *testing.T) *handlers.Handler {
	t.Helper()
	return &handlers.Handler{
		DB:        setupTestDB(t),
		JWTSecret: []byte("test-jwt-secret"),
	}
}

func TestRegister_Success(t *testing.T) {
	h := newTestHandler(t)
	defer h.DB.Close()

	body := `{"username":"alice","password":"secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/users/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["username"] != "alice" {
		t.Errorf("expected username alice, got %v", resp["username"])
	}
}

func TestRegister_DuplicateUsername(t *testing.T) {
	h := newTestHandler(t)
	defer h.DB.Close()

	body := `{"username":"alice","password":"secret123"}`
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/users/register", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.Register(w, req)
		if i == 0 && w.Code != http.StatusCreated {
			t.Fatalf("first register: expected 201, got %d", w.Code)
		}
		if i == 1 && w.Code != http.StatusConflict {
			t.Fatalf("duplicate register: expected 409, got %d", w.Code)
		}
	}
}

func TestRegister_MissingFields(t *testing.T) {
	h := newTestHandler(t)
	defer h.DB.Close()

	tests := []struct {
		body string
	}{
		{`{"username":"alice"}`},
		{`{"password":"secret123"}`},
		{`{}`},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodPost, "/users/register", bytes.NewBufferString(tt.body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.Register(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("body %q: expected 400, got %d", tt.body, w.Code)
		}
	}
}

func TestLogin_Success(t *testing.T) {
	h := newTestHandler(t)
	defer h.DB.Close()

	// Register first
	regBody := `{"username":"bob","password":"pass1234"}`
	req := httptest.NewRequest(http.MethodPost, "/users/register", bytes.NewBufferString(regBody))
	req.Header.Set("Content-Type", "application/json")
	h.Register(httptest.NewRecorder(), req)

	// Login
	loginBody := `{"username":"bob","password":"pass1234"}`
	req2 := httptest.NewRequest(http.MethodPost, "/users/login", bytes.NewBufferString(loginBody))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	h.Login(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w2.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	token, ok := resp["token"].(string)
	if !ok || token == "" {
		t.Error("expected non-empty token in login response")
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	h := newTestHandler(t)
	defer h.DB.Close()

	// Register
	regBody := `{"username":"carol","password":"pass1234"}`
	req := httptest.NewRequest(http.MethodPost, "/users/register", bytes.NewBufferString(regBody))
	req.Header.Set("Content-Type", "application/json")
	h.Register(httptest.NewRecorder(), req)

	tests := []struct {
		body string
	}{
		{`{"username":"carol","password":"wrongpass"}`},
		{`{"username":"nonexistent","password":"pass1234"}`},
	}
	for _, tt := range tests {
		req2 := httptest.NewRequest(http.MethodPost, "/users/login", bytes.NewBufferString(tt.body))
		req2.Header.Set("Content-Type", "application/json")
		w2 := httptest.NewRecorder()
		h.Login(w2, req2)
		if w2.Code != http.StatusUnauthorized {
			t.Errorf("body %q: expected 401, got %d", tt.body, w2.Code)
		}
	}
}

func TestAuthMiddleware_NoToken(t *testing.T) {
	h := newTestHandler(t)
	defer h.DB.Close()

	protected := h.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/surveys", nil)
	w := httptest.NewRecorder()
	protected(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	h := newTestHandler(t)
	defer h.DB.Close()

	protected := h.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/surveys", nil)
	req.Header.Set("Authorization", "Bearer notavalidtoken")
	w := httptest.NewRecorder()
	protected(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

