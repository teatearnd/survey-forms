package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"example.com/m/internal/dto"
	"example.com/m/internal/models"
	"example.com/m/internal/repository"
	"github.com/google/uuid"
)

func TestDefaultHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	handler := http.HandlerFunc(DefaultHandler)
	handler.ServeHTTP(recorder, req)

	if status := recorder.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v, want %v", status, http.StatusOK)
	}

	expected := `"There is nothing here."` + "\n"
	if recorder.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v, want %v", recorder.Body.String(), expected)
	}
}

func TestCreateSurvey(t *testing.T) {
	dbPath := "./test.db"
	db, err := repository.OpenDB_test()
	if err != nil {
		t.Fatalf("failed at db open: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		if err := os.Remove(dbPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("failed to delete remove test db %s: %v", dbPath, err)
		}
	})
	err = repository.InitSchema(db)
	if err != nil {
		t.Fatalf("failed at db initialization: %v", err)
	}
	def_handler := &Handler{DB: db}
	recorder := httptest.NewRecorder()
	request := dto.RequestCreateSurvey{Name: "Survey Name", Description: "Survey Description", Questions_list: []dto.RequestCreateQuestion{
		{
			Description: "Normal Description",
			Type:        0,
			IsMandatory: true,
			Choices: []models.Answer_choice{
				{
					Description: "Answer Choice description",
				},
			},
		},
		{
			Description: "Second question",
			Type:        1,
			IsMandatory: false,
		},
	}}
	decoder, err := json.Marshal(request) // marshal the request into json and put into decoder
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/survey", bytes.NewReader(decoder))
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")
	handler := http.HandlerFunc(def_handler.CreateSurvey)
	handler.ServeHTTP(recorder, req)

	if status := recorder.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v, want %v", status, http.StatusCreated)
	}
	expected := map[string]any{
		"message": "survey successfully created",
	}
	response := map[string]any{}
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("failed while unmarshalling a response: %v", err)
	}
	if response["message"] != expected["message"] {
		t.Errorf("response returned an unexpected result, got %v, want %v", response["message"], expected["message"])
	}
	if response["survey"] == nil {
		t.Errorf("no survey is present in response")
	}

}

func TestPublicCatalogEndpoints(t *testing.T) {
	db := setupTestDB(t)
	fixture := createSurveyFixture(t, db)
	defHandler := &Handler{DB: db}

	userID := uuid.New()
	otherUser := uuid.New()
	createSubmission(t, db, fixture, userID, time.Now().Add(-2*time.Hour), true)
	createSubmission(t, db, fixture, otherUser, time.Now().Add(-1*time.Hour), false)

	// Public submissions by survey
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/catalog/surveys/{surveyId}/submissions", nil)
	req = addURLParam(req, "surveyId", fixture.surveyID.String())
	handler := http.HandlerFunc(defHandler.GetPublicSubmissionsBySurvey)
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var subs []dto.ResponseCatalogSubmission
	if err := json.Unmarshal(recorder.Body.Bytes(), &subs); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 public submission, got %d", len(subs))
	}

	// Public answers by question
	recorder = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/catalog/questions/{questionId}/answers", nil)
	req = addURLParam(req, "questionId", fixture.q1ID.String())
	handler = http.HandlerFunc(defHandler.GetPublicAnswersByQuestion)
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var answers []dto.ResponseCatalogQuestionAnswer
	if err := json.Unmarshal(recorder.Body.Bytes(), &answers); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(answers) != 1 {
		t.Fatalf("expected 1 public answer, got %d", len(answers))
	}
	if answers[0].QuestionID != fixture.q1ID {
		t.Fatalf("expected question_id %s, got %s", fixture.q1ID, answers[0].QuestionID)
	}
}
