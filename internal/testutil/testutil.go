package testutil

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"testing"
	"time"

	"example.com/m/internal/auth"
	"example.com/m/internal/cache"
	"example.com/m/internal/models"
	"example.com/m/internal/repository"
	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	testSecret   = "test-secret"
	testIssuer   = "test-issuer"
	testAudience = "test-audience"
)

type SurveyFixture struct {
	SurveyID uuid.UUID
	Q1ID     uuid.UUID
	Q2ID     uuid.UUID
	ChoiceID uuid.UUID
}

// SetupTestDB opens a test sqlite DB, initializes schema and returns cleanup func
func SetupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	db, err := repository.OpenDB_test()
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := repository.InitSchema(db); err != nil {
		_ = db.Close()
		t.Fatalf("failed to init schema: %v", err)
	}
	cleanup := func() {
		_ = db.Close()
		_ = os.Remove("./test.db")
	}
	return db, cleanup
}

// CreateSurveyFixture inserts a minimal survey with two questions and returns ids
func CreateSurveyFixture(t *testing.T, db *sql.DB) SurveyFixture {
	t.Helper()
	fixture := SurveyFixture{
		SurveyID: uuid.New(),
		Q1ID:     uuid.New(),
		Q2ID:     uuid.New(),
		ChoiceID: uuid.New(),
	}

	survey := models.Survey{
		ID:          fixture.SurveyID,
		Name:        "Survey One",
		Description: "Survey Desc",
		CreatedAt:   time.Now().UTC(),
		Questions_list: []models.Question{
			{
				ID:          fixture.Q1ID,
				SurveyID:    fixture.SurveyID,
				Description: "Pick one",
				Type:        models.MultipleChoice,
				IsMandatory: true,
				Choices: []models.Answer_choice{
					{
						ID:          fixture.ChoiceID,
						Description: "Option A",
					},
				},
			},
			{
				ID:          fixture.Q2ID,
				SurveyID:    fixture.SurveyID,
				Description: "Describe",
				Type:        models.TextBased,
				IsMandatory: true,
			},
		},
	}

	if _, err := repository.InsertSurvey(db, survey); err != nil {
		t.Fatalf("failed to insert survey fixture: %v", err)
	}
	return fixture
}

// CreateSubmission inserts a submission for the given fixture and user
func CreateSubmission(t *testing.T, db *sql.DB, fixture SurveyFixture, userID uuid.UUID, submittedAt time.Time, isPublic bool) {
	t.Helper()
	answers := []models.Answer{
		{
			ID:           uuid.New(),
			QuestionID:   fixture.Q1ID,
			SubmissionID: uuid.New(),
			ChoiceID:     &fixture.ChoiceID,
			TextResponse: "",
		},
		{
			ID:           uuid.New(),
			QuestionID:   fixture.Q2ID,
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
		SurveyID: fixture.SurveyID,
		UserID:   userID,
		IsPublic: isPublic,
		Time:     submittedAt,
		Answers:  answers,
	}
	if _, err := repository.InsertSubmission(db, sub); err != nil {
		t.Fatalf("failed to insert submission: %v", err)
	}
}

// InitAuthForTest initializes auth package with test settings
func InitAuthForTest(t *testing.T) {
	t.Helper()
	if err := auth.Init(auth.Settings{Secret: testSecret, Issuer: testIssuer, Audience: testAudience}); err != nil {
		t.Fatalf("failed to init auth: %v", err)
	}
	if err := auth.ValidateConfig(); err != nil {
		t.Fatalf("invalid auth config: %v", err)
	}
}

// CreateTestToken signs and returns a test JWT for the given claims
func CreateTestToken(t *testing.T, claims auth.AccessClaims) string {
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

// SetupMiniredisCache starts a miniredis server and returns a RedisCache and cleanup
func SetupMiniredisCache(t *testing.T) (*cache.RedisCache, func()) {
	t.Helper()
	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	redisCache := cache.NewRedisCache(srv.Addr(), "", 0)
	if err := redisCache.Ping(); err != nil {
		srv.Close()
		t.Fatalf("failed to ping miniredis: %v", err)
	}
	cleanup := func() {
		srv.Close()
	}
	return redisCache, cleanup
}

// AddURLParam injects chi route context param into request
func AddURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
