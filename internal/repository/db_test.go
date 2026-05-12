package repository

import (
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"example.com/m/internal/models"
	"github.com/google/uuid"
)

type surveyFixture struct {
	surveyID uuid.UUID
	q1ID     uuid.UUID
	q2ID     uuid.UUID
	choiceID uuid.UUID
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := OpenDB_test()
	if err != nil {
		t.Fatalf("failed at db open: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		if err := os.Remove("./test.db"); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("failed to remove test db: %v", err)
		}
	})
	if err := InitSchema(db); err != nil {
		t.Fatalf("failed at db initialization: %v", err)
	}
	return db
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

	if _, err := InsertSurvey(db, survey); err != nil {
		t.Fatalf("failed to insert survey: %v", err)
	}
	return fixture
}

func createSubmission(t *testing.T, db *sql.DB, fixture surveyFixture, userID uuid.UUID, submittedAt time.Time, isPublic bool) {
	t.Helper()
	submissionID := uuid.New()
	answers := []models.Answer{
		{
			ID:           uuid.New(),
			QuestionID:   fixture.q1ID,
			SubmissionID: submissionID,
			ChoiceID:     &fixture.choiceID,
			TextResponse: "",
		},
		{
			ID:           uuid.New(),
			QuestionID:   fixture.q2ID,
			SubmissionID: submissionID,
			TextResponse: "Some text",
		},
	}

	sub := models.Submission{
		ID:       submissionID,
		SurveyID: fixture.surveyID,
		UserID:   userID,
		IsPublic: isPublic,
		Time:     submittedAt,
		Answers:  answers,
	}

	if _, err := InsertSubmission(db, sub); err != nil {
		t.Fatalf("failed to insert submission: %v", err)
	}
}

func TestSurveyLifecycle(t *testing.T) {
	db := setupTestDB(t)
	fixture := createSurveyFixture(t, db)

	exists, err := SurveyExists(db, fixture.surveyID.String())
	if err != nil {
		t.Fatalf("SurveyExists failed: %v", err)
	}
	if !exists {
		t.Fatalf("expected survey to exist")
	}

	survey, err := RetrieveSurvey(db, fixture.surveyID.String())
	if err != nil {
		t.Fatalf("RetrieveSurvey failed: %v", err)
	}
	if survey.ID != fixture.surveyID {
		t.Fatalf("expected survey ID %s, got %s", fixture.surveyID, survey.ID)
	}
	if len(survey.Questions_list) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(survey.Questions_list))
	}

	if err := DeleteSurveyByID(db, fixture.surveyID.String()); err != nil {
		t.Fatalf("DeleteSurveyByID failed: %v", err)
	}
	if err := DeleteSurveyByID(db, fixture.surveyID.String()); !errors.Is(err, ErrSurveyNotFound) {
		t.Fatalf("expected ErrSurveyNotFound, got %v", err)
	}
}

func TestSubmissionQueries(t *testing.T) {
	db := setupTestDB(t)
	fixture := createSurveyFixture(t, db)

	userID := uuid.New()
	otherUser := uuid.New()
	createSubmission(t, db, fixture, userID, time.Now().Add(-2*time.Hour), true)
	createSubmission(t, db, fixture, otherUser, time.Now().Add(-1*time.Hour), true)

	allSubs, err := ListSubmissionsBySurvey(db, fixture.surveyID.String(), nil)
	if err != nil {
		t.Fatalf("ListSubmissionsBySurvey failed: %v", err)
	}
	if len(allSubs) != 2 {
		t.Fatalf("expected 2 submissions, got %d", len(allSubs))
	}

	userFilter := userID.String()
	filtered, err := ListSubmissionsBySurvey(db, fixture.surveyID.String(), &userFilter)
	if err != nil {
		t.Fatalf("ListSubmissionsBySurvey filtered failed: %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered submission, got %d", len(filtered))
	}
	if filtered[0].UserID != userID {
		t.Fatalf("expected user_id %s, got %s", userID, filtered[0].UserID)
	}

	byUser, err := ListSubmissionsByUser(db, otherUser.String())
	if err != nil {
		t.Fatalf("ListSubmissionsByUser failed: %v", err)
	}
	if len(byUser) != 1 {
		t.Fatalf("expected 1 submission by user, got %d", len(byUser))
	}
	if byUser[0].UserID != otherUser {
		t.Fatalf("expected user_id %s, got %s", otherUser, byUser[0].UserID)
	}
}

func TestPublicCatalogQueries(t *testing.T) {
	db := setupTestDB(t)
	fixture := createSurveyFixture(t, db)

	userID := uuid.New()
	otherUser := uuid.New()
	createSubmission(t, db, fixture, userID, time.Now().Add(-2*time.Hour), true)
	createSubmission(t, db, fixture, otherUser, time.Now().Add(-1*time.Hour), false)

	publicSubs, err := ListPublicSubmissionsBySurvey(db, fixture.surveyID.String(), 50, 0)
	if err != nil {
		t.Fatalf("ListPublicSubmissionsBySurvey failed: %v", err)
	}
	if len(publicSubs) != 1 {
		t.Fatalf("expected 1 public submission, got %d", len(publicSubs))
	}

	publicAnswers, err := ListPublicAnswersByQuestion(db, fixture.q1ID.String(), 50, 0)
	if err != nil {
		t.Fatalf("ListPublicAnswersByQuestion failed: %v", err)
	}
	if len(publicAnswers) != 1 {
		t.Fatalf("expected 1 public answer for question, got %d", len(publicAnswers))
	}
	if publicAnswers[0].QuestionID != fixture.q1ID {
		t.Fatalf("expected question_id %s, got %s", fixture.q1ID, publicAnswers[0].QuestionID)
	}
}
