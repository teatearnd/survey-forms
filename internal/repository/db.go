package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"example.com/m/internal/dto"
	"example.com/m/internal/models"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

const initSchema = `
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS surveys (
	owner_id TEXT NOT NULL,
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
	description TEXT,
	created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS questions (
    id TEXT PRIMARY KEY,
    survey_id TEXT,
    description TEXT,
	type TEXT,
	is_mandatory BOOL,
    FOREIGN KEY(survey_id) REFERENCES surveys(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS choices (
	id TEXT PRIMARY KEY,
	question_id TEXT,
	description TEXT,
	FOREIGN KEY(question_id) REFERENCES questions(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS submissions (
	id TEXT PRIMARY KEY,
	survey_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	is_public BOOLEAN NOT NULL DEFAULT true,
	submitted_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(survey_id) REFERENCES surveys(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS answers (
	id TEXT PRIMARY KEY,
	submission_id TEXT NOT NULL,
	question_id TEXT NOT NULL,
	choice_id TEXT,
	text_response TEXT,
	FOREIGN KEY(submission_id) REFERENCES submissions(id) ON DELETE CASCADE,
	FOREIGN KEY(question_id) REFERENCES questions(id) ON DELETE CASCADE,
	FOREIGN KEY(choice_id) REFERENCES choices(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),	
		password_hash TEXT NOT NULL,
		email VARCHAR(255) UNIQUE NOT NULL,
		role TEXT NOT NULL DEFAULT 'user',
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		);

CREATE INDEX IF NOT EXISTS idx_submissions_survey_id ON submissions(survey_id);
CREATE INDEX IF NOT EXISTS idx_submissions_user_id ON submissions(user_id);
CREATE INDEX IF NOT EXISTS idx_submissions_public_survey ON submissions(survey_id, is_public);
CREATE INDEX IF NOT EXISTS idx_answers_submission_id ON answers(submission_id);
`

func OpenDB() (*sql.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to the db: %w", err)
	}
	// maxopenconns, maxidleconns?
	if err = db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping the db: %w", err)
	}
	log.Printf("established connection to db")
	return db, nil
}

func InitSchema(db *sql.DB) error {
	_, err := db.Exec(initSchema)
	if err != nil {
		return fmt.Errorf("failed to initialize tables %w", err)
	}
	return nil
}

func CreateUser(h *sql.DB, cred dto.UserRegistration) error {
	query := `INSERT INTO users (email, password_hash) VALUES ($1, $2)`
	_, err := h.Exec(query, cred.Email, cred.Password)
	if err != nil {
		return fmt.Errorf("failed to insert a user: %w", err)
	}
	return nil
}

func FindUserByEmail(h *sql.DB, email string) (string, error) {
	query := `SELECT password_hash FROM users WHERE email = $1`
	var hash string
	err := h.QueryRow(query, email).Scan(&hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("user not found")
		}
		return "", err
	}
	return hash, nil
}

func FindUserForLogin(h *sql.DB, email string) (string, string, string, error) {
	query := `SELECT id, role, password_hash FROM users WHERE email = $1`
	var id string
	var role string
	var hash string
	err := h.QueryRow(query, email).Scan(&id, &role, &hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", "", fmt.Errorf("user not found")
		}
		return "", "", "", err
	}
	return id, role, hash, nil
}

//

func InsertSurvey(h *sql.DB, survey models.Survey) (models.Survey, error) {
	tx, err := h.Begin()
	if err != nil {
		return models.Survey{}, err
	}
	defer tx.Rollback()

	const inserting_surveys = `
	INSERT INTO surveys(owner_id, id, name, description, created_at)
	VALUES ($1, $2, $3, $4, $5);
	`
	const inserting_questions = `
	INSERT INTO questions(id, survey_id, description, type, is_mandatory)
	VALUES ($1, $2, $3, $4, $5);
	`
	const inserting_choices = `
	INSERT INTO choices(id, question_id, description)
	VALUES ($1, $2, $3); `

	_, err = tx.Exec(inserting_surveys, survey.OwnerID, survey.ID.String(), survey.Name, survey.Description, survey.CreatedAt)
	if err != nil {
		return models.Survey{}, fmt.Errorf("failed at inserting surveys %s into the db: %w", survey.ID, err)
	}
	for _, j := range survey.Questions_list {
		_, err = tx.Exec(inserting_questions, j.ID.String(), j.SurveyID.String(), j.Description, j.Type, j.IsMandatory)
		if err != nil {
			return models.Survey{}, fmt.Errorf("failed while inserting question %s %w", j.ID, err)
		}
		for _, c := range j.Choices {
			_, err = tx.Exec(inserting_choices, c.ID.String(), j.ID.String(), c.Description)
			if err != nil {
				return models.Survey{}, fmt.Errorf("failed while inserting answer-choices: %w", err)
			}
		}
	}

	created := models.Survey{
		OwnerID:        survey.OwnerID,
		ID:             survey.ID,
		Name:           survey.Name,
		Description:    survey.Description,
		Questions_list: survey.Questions_list,
		CreatedAt:      survey.CreatedAt,
	}

	if err = tx.Commit(); err != nil {
		return models.Survey{}, fmt.Errorf("transaction commit failed: %w", err)
	}

	return created, nil
}

func CheckOwnership(h *sql.DB, userID string, surveyID string) error {
	var ownershipID string
	const checkOwnership = `
	SELECT owner_id FROM surveys WHERE id = $1;
	`

	err := h.QueryRow(checkOwnership, surveyID).Scan(&ownershipID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrSurveyNotFound
		}
		return fmt.Errorf("failed to find the survey: %w", err)
	}

	if ownershipID != userID {
		return ErrNotOwner
	}

	return nil
}

var ErrSurveyNotFound = errors.New("survey not found")
var ErrNotOwner = errors.New("user is not the owner of the survey")

func DeleteSurveyByID(h *sql.DB, id string) error {
	const deleteSurvey = `
	DELETE FROM surveys
	WHERE id = $1;
	`
	tx, err := h.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(deleteSurvey, id)
	if err != nil {
		return fmt.Errorf("failed at deleting survey %s: %w", id, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed reading delete result: %w", err)
	}
	if affected == 0 {
		return ErrSurveyNotFound
	}

	return tx.Commit()
}

func ListSurveys(h *sql.DB) ([]dto.ResponseGetSurveys, error) {
	const searchSurvey = `
	SELECT owner_id, id, name, description, created_at FROM surveys;
	`
	rows, err := h.Query(searchSurvey)
	if err != nil {
		return nil, fmt.Errorf("failed when parsing surveys: %w", err)
	}
	defer rows.Close()

	res := []dto.ResponseGetSurveys{}
	for rows.Next() {
		var temp models.Survey
		err = rows.Scan(&temp.OwnerID, &temp.ID, &temp.Name, &temp.Description, &temp.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed when preparing results: %w", err)
		}
		response := dto.GetSurveys(temp)
		res = append(res, response)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("iteration error on reading surveys: %w", err)
	}

	return res, nil
}

func RetrieveSurvey(h *sql.DB, id string) (dto.RequestSurvey, error) {
	const searchSurvey = `
	SELECT owner_id, id, name, description, created_at FROM surveys
	WHERE id = $1;
	`
	const searchQuestion = `
	SELECT description, type, is_mandatory, id FROM questions
	WHERE survey_id = $1;
	`
	const searchOptions = `
	SELECT id, description FROM choices
	WHERE question_id = $1;
	`
	res := models.Survey{}
	err := h.QueryRow(searchSurvey, id).Scan(
		&res.OwnerID,
		&res.ID,
		&res.Name,
		&res.Description,
		&res.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dto.RequestSurvey{}, ErrSurveyNotFound
		}
		return dto.RequestSurvey{}, fmt.Errorf("failed when parsing a survey: %w", err)
	}
	response := dto.RequestSurvey{
		OwnerID:        res.OwnerID,
		ID:             res.ID,
		Name:           res.Name,
		Description:    res.Description,
		CreatedAt:      res.CreatedAt,
		Questions_list: []dto.RequestQuestion{},
	}

	rows, err := h.Query(searchQuestion, res.ID)
	if err != nil {
		return dto.RequestSurvey{}, fmt.Errorf("failed to read results: %w", err)
	}
	questions := []models.Question{}
	for rows.Next() {
		question := models.Question{}
		err = rows.Scan(&question.Description, &question.Type, &question.IsMandatory, &question.ID)
		if err != nil {
			return dto.RequestSurvey{}, fmt.Errorf("failed to read questions: %w", err)
		}
		questions = append(questions, question)
	}

	if err = rows.Err(); err != nil {
		return dto.RequestSurvey{}, fmt.Errorf("iteration error: %w", err)
	}
	rows.Close()

	for _, question := range questions {
		choices := []models.Answer_choice{}

		cRows, err := h.Query(searchOptions, question.ID)
		if err != nil {
			return dto.RequestSurvey{}, fmt.Errorf("failed reading options: %w", err)
		}
		for cRows.Next() {
			choice := models.Answer_choice{}
			err = cRows.Scan(&choice.ID, &choice.Description)
			if err != nil {
				cRows.Close()
				return dto.RequestSurvey{}, fmt.Errorf("failed on reading options: %w", err)
			}
			choices = append(choices, choice)
		}
		if err = cRows.Err(); err != nil {
			cRows.Close()
			return dto.RequestSurvey{}, fmt.Errorf("iteration error on questions: %w", err)
		}
		cRows.Close()

		dto_question := dto.RequestQuestion{
			Description: question.Description,
			Type:        question.Type,
			Choices:     choices,
			IsMandatory: question.IsMandatory,
		}
		response.Questions_list = append(response.Questions_list, dto_question)
	}

	return response, nil
}

func SurveyExists(h *sql.DB, id string) (bool, error) {
	const query = `
	SELECT 1 FROM surveys
	WHERE id = $1
	LIMIT 1;
	`
	var exists int
	if err := h.QueryRow(query, id).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check survey existence: %w", err)
	}
	return true, nil
}

func GetSurveyQuestionMeta(h *sql.DB, surveyID string) (map[uuid.UUID]models.QuestionMeta, error) {
	const queryQuestions = `
	SELECT id, type, is_mandatory FROM questions
	WHERE survey_id = $1;
	`
	const queryChoices = `
	SELECT id FROM choices
	WHERE question_id = $1;
	`
	rows, err := h.Query(queryQuestions, surveyID)
	if err != nil {
		return nil, fmt.Errorf("failed to read survey questions: %w", err)
	}
	res := make(map[uuid.UUID]models.QuestionMeta)
	questionIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var idStr string
		var qType models.QuestionType
		var isMandatory bool
		if err := rows.Scan(&idStr, &qType, &isMandatory); err != nil {
			return nil, fmt.Errorf("failed to scan questions: %w", err)
		}
		qid, err := uuid.Parse(idStr)
		if err != nil {
			return nil, fmt.Errorf("invalid question id in db: %w", err)
		}
		meta := models.QuestionMeta{
			ID:          qid,
			Type:        qType,
			IsMandatory: isMandatory,
			ChoiceIDs:   map[uuid.UUID]struct{}{},
		}
		res[qid] = meta
		questionIDs = append(questionIDs, qid)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iteration error on questions: %w", err)
	}
	rows.Close()

	for _, qid := range questionIDs {
		cRows, err := h.Query(queryChoices, qid.String())
		if err != nil {
			return nil, fmt.Errorf("failed to read choices: %w", err)
		}
		for cRows.Next() {
			var choiceIDStr string
			if err := cRows.Scan(&choiceIDStr); err != nil {
				cRows.Close()
				return nil, fmt.Errorf("failed to scan choices: %w", err)
			}
			cid, err := uuid.Parse(choiceIDStr)
			if err != nil {
				cRows.Close()
				return nil, fmt.Errorf("invalid choice id in db: %w", err)
			}
			meta := res[qid]
			meta.ChoiceIDs[cid] = struct{}{}
			res[qid] = meta
		}
		if err := cRows.Err(); err != nil {
			cRows.Close()
			return nil, fmt.Errorf("iteration error on choices: %w", err)
		}
		cRows.Close()
	}

	return res, nil
}

func InsertSubmission(h *sql.DB, submission models.Submission) (models.Submission, error) {
	tx, err := h.Begin()
	if err != nil {
		return models.Submission{}, err
	}
	defer tx.Rollback()

	const insertSubmission = `
	INSERT INTO submissions(id, survey_id, user_id, is_public, submitted_at)
	VALUES ($1, $2, $3, $4, $5);
	`
	const insertAnswer = `
	INSERT INTO answers(id, submission_id, question_id, choice_id, text_response)
	VALUES ($1, $2, $3, $4, $5);
	`

	_, err = tx.Exec(
		insertSubmission,
		submission.ID.String(),
		submission.SurveyID.String(),
		submission.UserID.String(),
		submission.IsPublic,
		submission.Time,
	)
	if err != nil {
		return models.Submission{}, fmt.Errorf("failed to insert submission: %w", err)
	}

	for _, ans := range submission.Answers {
		var choiceID any
		if ans.ChoiceID != nil {
			choiceID = ans.ChoiceID.String()
		}
		_, err = tx.Exec(insertAnswer, ans.ID.String(), submission.ID.String(), ans.QuestionID.String(), choiceID, ans.TextResponse)
		if err != nil {
			return models.Submission{}, fmt.Errorf("failed to insert answer: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return models.Submission{}, fmt.Errorf("failed to commit submission transaction: %w", err)
	}
	return submission, nil
}

func ListSubmissionsBySurvey(h *sql.DB, surveyID string, userID *string) ([]models.Submission, error) {
	query := `
	SELECT id, survey_id, user_id, submitted_at
	FROM submissions
	WHERE survey_id = $1
	`
	args := []any{surveyID}
	if userID != nil {
		query += " AND user_id = $2"
		args = append(args, *userID)
	}
	query += " ORDER BY submitted_at DESC;"

	rows, err := h.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query submissions: %w", err)
	}
	res := []models.Submission{}
	for rows.Next() {
		var sub models.Submission
		var idStr, surveyIDStr, userIDStr string
		if err := rows.Scan(&idStr, &surveyIDStr, &userIDStr, &sub.Time); err != nil {
			return nil, fmt.Errorf("failed to scan submissions: %w", err)
		}
		if sub.ID, err = uuid.Parse(idStr); err != nil {
			return nil, fmt.Errorf("invalid submission id: %w", err)
		}
		if sub.SurveyID, err = uuid.Parse(surveyIDStr); err != nil {
			return nil, fmt.Errorf("invalid survey id in submission: %w", err)
		}
		if sub.UserID, err = uuid.Parse(userIDStr); err != nil {
			return nil, fmt.Errorf("invalid user id in submission: %w", err)
		}

		res = append(res, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iteration error on submissions: %w", err)
	}
	rows.Close()

	for i := range res {
		answers, err := listAnswersBySubmission(h, res[i].ID.String())
		if err != nil {
			return nil, err
		}
		res[i].Answers = answers
	}

	return res, nil
}

func ListPublicSubmissionsBySurvey(h *sql.DB, surveyID string, limit, offset int) ([]models.Submission, error) {
	const query = `
	SELECT id, survey_id, user_id, submitted_at
	FROM submissions
	WHERE survey_id = $1 AND is_public = true
	ORDER BY submitted_at DESC
	LIMIT $2 OFFSET $3;
	`
	rows, err := h.Query(query, surveyID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query public submissions: %w", err)
	}
	res := []models.Submission{}
	for rows.Next() {
		var sub models.Submission
		var idStr, surveyIDStr, userIDStr string
		if err := rows.Scan(&idStr, &surveyIDStr, &userIDStr, &sub.Time); err != nil {
			return nil, fmt.Errorf("failed to scan submissions: %w", err)
		}
		if sub.ID, err = uuid.Parse(idStr); err != nil {
			return nil, fmt.Errorf("invalid submission id: %w", err)
		}
		if sub.SurveyID, err = uuid.Parse(surveyIDStr); err != nil {
			return nil, fmt.Errorf("invalid survey id in submission: %w", err)
		}
		if sub.UserID, err = uuid.Parse(userIDStr); err != nil {
			return nil, fmt.Errorf("invalid user id in submission: %w", err)
		}

		res = append(res, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iteration error on submissions: %w", err)
	}
	rows.Close()

	for i := range res {
		answers, err := listAnswersBySubmission(h, res[i].ID.String())
		if err != nil {
			return nil, err
		}
		res[i].Answers = answers
	}

	return res, nil
}

func ListSubmissionsByUser(h *sql.DB, userID string) ([]models.Submission, error) {
	query := `
	SELECT id, survey_id, user_id, submitted_at
	FROM submissions
	WHERE user_id = $1
	ORDER BY submitted_at DESC;
	`
	rows, err := h.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query submissions by user: %w", err)
	}
	res := []models.Submission{}
	for rows.Next() {
		var sub models.Submission
		var idStr, surveyIDStr, userIDStr string
		if err := rows.Scan(&idStr, &surveyIDStr, &userIDStr, &sub.Time); err != nil {
			return nil, fmt.Errorf("failed to scan submissions: %w", err)
		}
		if sub.ID, err = uuid.Parse(idStr); err != nil {
			return nil, fmt.Errorf("invalid submission id: %w", err)
		}
		if sub.SurveyID, err = uuid.Parse(surveyIDStr); err != nil {
			return nil, fmt.Errorf("invalid survey id in submission: %w", err)
		}
		if sub.UserID, err = uuid.Parse(userIDStr); err != nil {
			return nil, fmt.Errorf("invalid user id in submission: %w", err)
		}

		res = append(res, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iteration error on submissions: %w", err)
	}
	rows.Close()

	for i := range res {
		answers, err := listAnswersBySubmission(h, res[i].ID.String())
		if err != nil {
			return nil, err
		}
		res[i].Answers = answers
	}

	return res, nil
}

func ListPublicAnswersByQuestion(h *sql.DB, questionID string, limit, offset int) ([]models.CatalogAnswer, error) {
	const query = `
	SELECT a.id, a.question_id, a.choice_id, a.text_response, s.survey_id, s.submitted_at
	FROM answers a
	JOIN submissions s ON s.id = a.submission_id
	WHERE a.question_id = $1 AND s.is_public = true
	ORDER BY s.submitted_at DESC
	LIMIT $2 OFFSET $3;
	`
	rows, err := h.Query(query, questionID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query public answers: %w", err)
	}
	defer rows.Close()

	res := []models.CatalogAnswer{}
	for rows.Next() {
		var ans models.CatalogAnswer
		var idStr, questionIDStr, surveyIDStr string
		var choiceIDStr sql.NullString
		if err := rows.Scan(&idStr, &questionIDStr, &choiceIDStr, &ans.TextResponse, &surveyIDStr, &ans.SubmittedAt); err != nil {
			return nil, fmt.Errorf("failed to scan public answers: %w", err)
		}
		var parseErr error
		if ans.ID, parseErr = uuid.Parse(idStr); parseErr != nil {
			return nil, fmt.Errorf("invalid answer id: %w", parseErr)
		}
		if ans.QuestionID, parseErr = uuid.Parse(questionIDStr); parseErr != nil {
			return nil, fmt.Errorf("invalid question id: %w", parseErr)
		}
		if ans.SurveyID, parseErr = uuid.Parse(surveyIDStr); parseErr != nil {
			return nil, fmt.Errorf("invalid survey id: %w", parseErr)
		}
		if choiceIDStr.Valid {
			choiceID, parseErr := uuid.Parse(choiceIDStr.String)
			if parseErr != nil {
				return nil, fmt.Errorf("invalid choice id: %w", parseErr)
			}
			ans.ChoiceID = &choiceID
		}
		res = append(res, ans)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iteration error on public answers: %w", err)
	}

	return res, nil
}

func listAnswersBySubmission(h *sql.DB, submissionID string) ([]models.Answer, error) {
	const query = `
	SELECT id, question_id, choice_id, text_response
	FROM answers
	WHERE submission_id = $1;
	`
	rows, err := h.Query(query, submissionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query answers: %w", err)
	}
	defer rows.Close()

	answers := []models.Answer{}
	for rows.Next() {
		var ans models.Answer
		var idStr, questionIDStr string
		var choiceIDStr sql.NullString
		if err := rows.Scan(&idStr, &questionIDStr, &choiceIDStr, &ans.TextResponse); err != nil {
			return nil, fmt.Errorf("failed to scan answers: %w", err)
		}
		var parseErr error
		if ans.ID, parseErr = uuid.Parse(idStr); parseErr != nil {
			return nil, fmt.Errorf("invalid answer id: %w", parseErr)
		}
		if ans.QuestionID, parseErr = uuid.Parse(questionIDStr); parseErr != nil {
			return nil, fmt.Errorf("invalid question id: %w", parseErr)
		}
		if choiceIDStr.Valid {
			choiceID, parseErr := uuid.Parse(choiceIDStr.String)
			if parseErr != nil {
				return nil, fmt.Errorf("invalid choice id: %w", parseErr)
			}
			ans.ChoiceID = &choiceID
		}
		answers = append(answers, ans)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iteration error on answers: %w", err)
	}
	return answers, nil
}

func QuestionExists(h *sql.DB, id string) (bool, error) {
	const query = `
	SELECT 1 FROM questions
	WHERE id = $1
	LIMIT 1;
	`
	var exists int
	if err := h.QueryRow(query, id).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check question existence: %w", err)
	}
	return true, nil
}

// Testing environment
func OpenDB_test() (*sql.DB, error) {
	dsn := os.Getenv("DATABASE_URL_TEST")
	if dsn == "" {
		dsn = "postgres://survey_app:survey_pass@localhost:5432/survey_forms_test?sslmode=disable"
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		fmt.Println(err)
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db.SetMaxIdleConns(1)
	db.SetMaxOpenConns(1)
	db.SetConnMaxIdleTime(time.Second * 30)

	err = db.Ping()
	if err != nil {
		db.Close()
		return nil, err
	}

	log.Printf("established connection to db")
	return db, nil
}
