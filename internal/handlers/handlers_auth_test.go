package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"example.com/m/internal/auth"
	"example.com/m/internal/cache"
	"example.com/m/internal/dto"
	"example.com/m/internal/testutil"
	"github.com/google/uuid"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, cleanup := testutil.SetupTestDB(t)
	t.Cleanup(cleanup)
	return db
}

func initAuthForTest(t *testing.T) { testutil.InitAuthForTest(t) }

func createTestToken(t *testing.T, claims auth.AccessClaims) string {
	return testutil.CreateTestToken(t, claims)
}

type surveyFixture struct {
	surveyID uuid.UUID
	q1ID     uuid.UUID
	q2ID     uuid.UUID
	choiceID uuid.UUID
}

func createSurveyFixture(t *testing.T, db *sql.DB) surveyFixture {
	t.Helper()
	f := testutil.CreateSurveyFixture(t, db)
	return surveyFixture{
		surveyID: f.SurveyID,
		q1ID:     f.Q1ID,
		q2ID:     f.Q2ID,
		choiceID: f.ChoiceID,
	}
}

func createSubmission(t *testing.T, db *sql.DB, fixture surveyFixture, userID uuid.UUID, submittedAt time.Time, isPublic bool) {
	t.Helper()
	f := testutil.SurveyFixture{
		SurveyID: fixture.surveyID,
		Q1ID:     fixture.q1ID,
		Q2ID:     fixture.q2ID,
		ChoiceID: fixture.choiceID,
	}
	testutil.CreateSubmission(t, db, f, userID, submittedAt, isPublic)
}

func addURLParam(req *http.Request, key, value string) *http.Request {
	return testutil.AddURLParam(req, key, value)
}

func setupCartTestCache(t *testing.T) *cache.RedisCache {
	t.Helper()
	rc, cleanup := testutil.SetupMiniredisCache(t)
	t.Cleanup(cleanup)
	return rc
}

func TestCreateSubmissionSuccess(t *testing.T) {
	initAuthForTest(t)
	db := setupTestDB(t)
	fixture := createSurveyFixture(t, db)

	defHandler := &Handler{DB: db}
	userID := uuid.New()
	token := createTestToken(t, auth.AccessClaims{Email: "user@example.com", UserID: userID.String(), Role: "user"})

	request := dto.RequestCreateSubmission{
		Answers: []dto.RequestCreateAnswer{
			{
				QuestionID:   fixture.q1ID,
				ChoiceID:     &fixture.choiceID,
				TextResponse: "",
			},
			{
				QuestionID:   fixture.q2ID,
				TextResponse: "Hello",
			},
		},
	}
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/survey/{surveyId}/submissions", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req = addURLParam(req, "surveyId", fixture.surveyID.String())

	handler := auth.AuthMiddleware(http.HandlerFunc(defHandler.CreateSubmission))
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, recorder.Code)
	}

	var resp dto.ResponseSubmission
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.UserID != userID {
		t.Fatalf("expected user_id %s, got %s", userID, resp.UserID)
	}
	if resp.SurveyID != fixture.surveyID {
		t.Fatalf("expected survey_id %s, got %s", fixture.surveyID, resp.SurveyID)
	}
}

func TestGetSubmissionsBySurveyFiltersNonAdmin(t *testing.T) {
	initAuthForTest(t)
	db := setupTestDB(t)
	fixture := createSurveyFixture(t, db)

	userID := uuid.New()
	otherUser := uuid.New()
	createSubmission(t, db, fixture, userID, time.Now().Add(-2*time.Hour), true)
	createSubmission(t, db, fixture, otherUser, time.Now().Add(-1*time.Hour), true)

	defHandler := &Handler{DB: db}
	token := createTestToken(t, auth.AccessClaims{Email: "user@example.com", UserID: userID.String(), Role: "user"})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/survey/{surveyId}/submissions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = addURLParam(req, "surveyId", fixture.surveyID.String())

	handler := auth.AuthMiddleware(http.HandlerFunc(defHandler.GetSubmissionsBySurvey))
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var resp []dto.ResponseSubmission
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 submission, got %d", len(resp))
	}
	if resp[0].UserID != userID {
		t.Fatalf("expected user_id %s, got %s", userID, resp[0].UserID)
	}
}

func TestGetSubmissionsByUserForbidden(t *testing.T) {
	initAuthForTest(t)
	db := setupTestDB(t)
	fixture := createSurveyFixture(t, db)

	userID := uuid.New()
	otherUser := uuid.New()
	createSubmission(t, db, fixture, otherUser, time.Now().Add(-1*time.Hour), true)

	defHandler := &Handler{DB: db}
	token := createTestToken(t, auth.AccessClaims{Email: "user@example.com", UserID: userID.String(), Role: "user"})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/{userId}/submissions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = addURLParam(req, "userId", otherUser.String())

	handler := auth.AuthMiddleware(http.HandlerFunc(defHandler.GetSubmissionsByUser))
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, recorder.Code)
	}
}

func TestGetSurveysAndGetSingleSurvey(t *testing.T) {
	db := setupTestDB(t)
	fixture := createSurveyFixture(t, db)
	defHandler := &Handler{DB: db}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/surveys", nil)
	handler := http.HandlerFunc(defHandler.GetSurveys)
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var list []dto.ResponseGetSurveys
	if err := json.Unmarshal(recorder.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 survey, got %d", len(list))
	}

	recorder = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/survey/{surveyId}", nil)
	req = addURLParam(req, "surveyId", fixture.surveyID.String())
	handler = http.HandlerFunc(defHandler.GetSingleSurvey)
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var single dto.RequestSurvey
	if err := json.Unmarshal(recorder.Body.Bytes(), &single); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if single.ID != fixture.surveyID {
		t.Fatalf("expected survey_id %s, got %s", fixture.surveyID, single.ID)
	}
}

func TestDeleteSurvey(t *testing.T) {
	db := setupTestDB(t)
	fixture := createSurveyFixture(t, db)
	defHandler := &Handler{DB: db}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/survey/{surveyId}", nil)
	req = addURLParam(req, "surveyId", fixture.surveyID.String())
	handler := http.HandlerFunc(defHandler.DeleteSurvey)
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["deleted_id"] != fixture.surveyID.String() {
		t.Fatalf("expected deleted_id %s, got %v", fixture.surveyID, resp["deleted_id"])
	}
}

func TestCartAddGetRemoveClear(t *testing.T) {
	initAuthForTest(t)
	redisCache := setupCartTestCache(t)
	defHandler := &Handler{Cache: redisCache}

	userID := uuid.New()
	token := createTestToken(t, auth.AccessClaims{Email: "user@example.com", UserID: userID.String(), Role: "user"})

	// Add two items
	addPayload := func(value string) []byte {
		body := dto.RequestCartObject{Item: dto.CartItem{
			SurveyID:   uuid.New(),
			QuestionID: uuid.New(),
			Note:       value,
		}}
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal cart payload: %v", err)
		}
		return payload
	}

	for _, value := range []string{"first", "second"} {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/cart/items", bytes.NewReader(addPayload(value)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		handler := auth.AuthMiddleware(http.HandlerFunc(defHandler.AddToCart))
		handler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, recorder.Code)
		}
	}

	// Get cart with pagination
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/cart?limit=1&offset=0", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler := auth.AuthMiddleware(http.HandlerFunc(defHandler.GetCart))
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var cartResp dto.ResponseCart
	if err := json.Unmarshal(recorder.Body.Bytes(), &cartResp); err != nil {
		t.Fatalf("failed to decode cart response: %v", err)
	}
	if len(cartResp.Cart) != 1 {
		t.Fatalf("expected 1 cart item, got %d", len(cartResp.Cart))
	}
	if cartResp.Cart[0].Note != "second" {
		t.Fatalf("expected newest item first, got %#v", cartResp.Cart[0])
	}

	// Remove newest item (index 0)
	recorder = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/cart/items/0", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = addURLParam(req, "index", "0")
	handler = auth.AuthMiddleware(http.HandlerFunc(defHandler.RemoveFromCart))
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	// Clear remaining cart
	recorder = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/cart", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler = auth.AuthMiddleware(http.HandlerFunc(defHandler.ClearCart))
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	if count, err := redisCache.Len(userID.String()); err != nil {
		t.Fatalf("Len failed: %v", err)
	} else if count != 0 {
		t.Fatalf("expected empty cart, got %d items", count)
	}
}

func TestCartUnauthorized(t *testing.T) {
	initAuthForTest(t)
	redisCache := setupCartTestCache(t)
	defHandler := &Handler{Cache: redisCache}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cart/items", bytes.NewReader([]byte(`{"item":{"item":"x"}}`)))
	req.Header.Set("Content-Type", "application/json")
	handler := auth.AuthMiddleware(http.HandlerFunc(defHandler.AddToCart))
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
}
