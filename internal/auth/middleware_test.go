package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testSecret   = "test-secret"
	testIssuer   = "test-issuer"
	testAudience = "test-audience"
)

func initAuthForTest(t *testing.T) {
	t.Helper()
	if err := Init(Settings{Secret: testSecret, Issuer: testIssuer, Audience: testAudience}); err != nil {
		t.Fatalf("failed to init auth: %v", err)
	}
	if err := ValidateConfig(); err != nil {
		t.Fatalf("invalid auth config: %v", err)
	}
}

func createTestToken(t *testing.T, claims AccessClaims) string {
	t.Helper()
	claims.RegisteredClaims = jwt.RegisteredClaims{
		Issuer:    testIssuer,
		Audience:  jwt.ClaimStrings{testAudience},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

func TestAuthMiddlewareMissingHeader(t *testing.T) {
	initAuthForTest(t)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "missing or invalid authorization header") {
		t.Fatalf("unexpected response: %s", recorder.Body.String())
	}
}

func TestAuthMiddlewareMissingToken(t *testing.T) {
	initAuthForTest(t)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer ")

	handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "missing token") {
		t.Fatalf("unexpected response: %s", recorder.Body.String())
	}
}

func TestAuthMiddlewareInvalidToken(t *testing.T) {
	initAuthForTest(t)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt")

	handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "invalid token") {
		t.Fatalf("unexpected response: %s", recorder.Body.String())
	}
}

func TestAuthMiddlewareSetsClaims(t *testing.T) {
	initAuthForTest(t)
	token := createTestToken(t, AccessClaims{Email: "test@example.com", UserID: "11111111-1111-1111-1111-111111111111", Role: "admin"})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetClaims(r)
		if !ok {
			http.Error(w, "missing claims", http.StatusUnauthorized)
			return
		}
		if claims.Email != "test@example.com" {
			http.Error(w, "wrong claims", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
}
