package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"example.com/m/internal/auth"
	"example.com/m/internal/dto"
	"example.com/m/internal/models"
	"example.com/m/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	testSecret   = "test-secret"
	testIssuer   = "test-issuer"
	testAudience = "test-audience"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := repository.OpenDB_test()
	if err != nil {
		t.Fatalf("failed at db open: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		if err := os.Remove("./test.db"); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("failed to remove test db: %v", err)
		}
	})
	if err := repository.InitSchema(db); err != nil {
		t.Fatalf("failed at db initialization: %v", err)
	}
	return db
}

func initAuthForTest(t *testing.T) {
	t.Helper()
	if err := auth.Init(auth.Settings{Secret: testSecret, Issuer: testIssuer, Audience: testAudience}); err != nil {
		t.Fatalf("failed to init auth: %v", err)
	}
	if err := auth.ValidateConfig(); err != nil {
		t.Fatalf("invalid auth config: %v", err)
	}
}

func createTestToken(t *testing.T, claims auth.AccessClaims) string {
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

type surveyFixture struct {
	surveyID uuid.UUID
	q1ID     uuid.UUID
	q2ID     uuid.UUID
	choiceID uuid.UUID
}

func createSurveyFixture(t *testing.T, db *sql.DB) surveyFixture {
	t.Helper()
	fixture := surveyFixture{
		surveyID: uuid.New(),
		q1ID:     uuid.New(),
		q2ID:     uuid.New(),
		choiceID: uuid.New(),
	}

	survey := models.Survey{
		ID:          fixture.surveyID,
		Name:        "Survey One",
		Description: "Survey Desc",
		CreatedAt:   time.Now().UTC(),
		Questions_list: []models.Question{
			{
				ID:          fixture.q1ID,
				SurveyID:    fixture.surveyID,
				Description: "Pick one",
				Type:        models.MultipleChoice,
				IsMandatory: true,
				Choices: []models.Answer_choice{
					{
						ID:          fixture.choiceID,
						Description: "Option A",
					},
				},
			},
			{
				ID:          fixture.q2ID,
				SurveyID:    fixture.surveyID,
				Description: "Describe",
				Type:        models.TextBased,
				IsMandatory: true,
			},
		},
	}

	if _, err := repository.InsertSurvey(db, survey); err != nil {
		t.Fatalf("failed to insert survey: %v", err)
	}
	return fixture
}

func createSubmission(t *testing.T, db *sql.DB, fixture surveyFixture, userID uuid.UUID, submittedAt time.Time) {
	t.Helper()
	answers := []models.Answer{
		{
			ID:           uuid.New(),
			QuestionID:   fixture.q1ID,
			SubmissionID: uuid.New(),
			ChoiceID:     &fixture.choiceID,
			TextResponse: "",
		},
		{
			ID:           uuid.New(),
			QuestionID:   fixture.q2ID,
			SubmissionID: uuid.New(),
			TextResponse: "Some text",
		},
	}
	submissionID := uuid.New()
	for i := range answers {
		answers[i].SubmissionID = submissionID
	}

	sub := models.Submission{
		ID:       submissionID,
		SurveyID: fixture.surveyID,
		UserID:   userID,
		Time:     submittedAt,
		Answers:  answers,
	}

	if _, err := repository.InsertSubmission(db, sub); err != nil {
		t.Fatalf("failed to insert submission: %v", err)
	}
}

func addURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
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
	createSubmission(t, db, fixture, userID, time.Now().Add(-2*time.Hour))
	createSubmission(t, db, fixture, otherUser, time.Now().Add(-1*time.Hour))

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
	createSubmission(t, db, fixture, otherUser, time.Now().Add(-1*time.Hour))

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
