package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"example.com/m/internal/dto"
	"example.com/m/internal/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

const initSchema = `
CREATE TABLE IF NOT EXISTS surveys (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
	description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
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
	is_public BOOL NOT NULL DEFAULT 1,
	submitted_at DATETIME DEFAULT CURRENT_TIMESTAMP,
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

CREATE INDEX IF NOT EXISTS idx_submissions_survey_id ON submissions(survey_id);
CREATE INDEX IF NOT EXISTS idx_submissions_user_id ON submissions(user_id);
CREATE INDEX IF NOT EXISTS idx_submissions_public_survey ON submissions(survey_id, is_public);
CREATE INDEX IF NOT EXISTS idx_answers_submission_id ON answers(submission_id);
`

func OpenDB() (*sql.DB, error) {
	db, err := sqlx.Connect("sqlite3", "./my.db?_foreign_keys=1")
	if err != nil {
		fmt.Println(err)
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(10)
	db.SetConnMaxIdleTime(time.Second * 30)

	err = db.Ping()
	if err != nil {
		db.Close()
		return nil, err
	}

	log.Printf("established connection to db")
	return db.DB, nil
}

func InitSchema(db *sql.DB) error {
	_, err := db.Exec(initSchema)
	if err != nil {
		return fmt.Errorf("failed to initialize tables %w", err)
	}

	if err := ensureSubmissionPublicColumn(db); err != nil {
		return err
	}

	_, err = db.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		return fmt.Errorf("failed to turn on fkeys at %w", err)
	}
	return nil
}

func ensureSubmissionPublicColumn(db *sql.DB) error {
	_, err := db.Exec("ALTER TABLE submissions ADD COLUMN is_public BOOL NOT NULL DEFAULT 1;")
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "duplicate column name") || strings.Contains(msg, "duplicate column") {
		return nil
	}
	return fmt.Errorf("failed to add submissions.is_public column: %w", err)
}

func InsertSurvey(h *sql.DB, survey models.Survey) (models.Survey, error) {
	tx, err := h.Begin()
	if err != nil {
		return models.Survey{}, err
	}
	defer tx.Rollback()

	const inserting_surveys = `
	INSERT INTO surveys(id, name, description, created_at)
	VALUES (?, ?, ?, ?);
	`
	const inserting_questions = `
	INSERT INTO questions(id, survey_id, description, type, is_mandatory)
	VALUES (?, ?, ?, ?, ?);
	`
	const inserting_choices = `
	INSERT INTO choices(id, question_id, description)
	VALUES (?, ?, ?); `

	_, err = tx.Exec(inserting_surveys, survey.ID, survey.Name, survey.Description, survey.CreatedAt)
	if err != nil {
		return models.Survey{}, fmt.Errorf("failed at inserting surveys %s into the db: %w", survey.ID, err)
	}
	for _, j := range survey.Questions_list {
		_, err = tx.Exec(inserting_questions, j.ID, j.SurveyID, j.Description, j.Type, j.IsMandatory)
		if err != nil {
			return models.Survey{}, fmt.Errorf("failed while inserting question %s %w", j.ID, err)
		}
		for _, c := range j.Choices {
			_, err = tx.Exec(inserting_choices, c.ID, j.ID, c.Description)
			if err != nil {
				return models.Survey{}, fmt.Errorf("failed while inserting answer-choices: %w", err)
			}
		}
	}

	created := models.Survey{
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

var ErrSurveyNotFound = errors.New("survey not found")

func DeleteSurveyByID(h *sql.DB, id string) error {
	const deleteSurvey = `
	DELETE FROM surveys
	WHERE id = ?;
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
	SELECT id, name, description, created_at FROM surveys;
	`
	rows, err := h.Query(searchSurvey)
	if err != nil {
		return nil, fmt.Errorf("failed when parsing surveys: %w", err)
	}
	defer rows.Close()

	res := []dto.ResponseGetSurveys{}
	for rows.Next() {
		var temp models.Survey
		err = rows.Scan(&temp.ID, &temp.Name, &temp.Description, &temp.CreatedAt)
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
	SELECT id, name, description, created_at FROM surveys
	WHERE id = ?;
	`
	const searchQuestion = `
	SELECT description, type, is_mandatory, id FROM questions
	WHERE survey_id = ?;
	`
	const searchOptions = `
	SELECT id, description FROM choices
	WHERE question_id = ?
	`
	res := models.Survey{}
	err := h.QueryRow(searchSurvey, id).Scan(
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
	defer rows.Close()

	for rows.Next() {
		question := models.Question{}
		err = rows.Scan(&question.Description, &question.Type, &question.IsMandatory, &question.ID)
		if err != nil {
			return dto.RequestSurvey{}, fmt.Errorf("failed to read questions: %w", err)
		}

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

		// new dto with no survey_id present
		dto_question := dto.RequestQuestion{
			Description: question.Description,
			Type:        question.Type,
			Choices:     choices,
			IsMandatory: question.IsMandatory,
		}
		response.Questions_list = append(response.Questions_list, dto_question)
	}

	if err = rows.Err(); err != nil {
		return dto.RequestSurvey{}, fmt.Errorf("iteration error: %w", err)
	}

	return response, nil
}

func SurveyExists(h *sql.DB, id string) (bool, error) {
	const query = `
	SELECT 1 FROM surveys
	WHERE id = ?
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
	WHERE survey_id = ?;
	`
	const queryChoices = `
	SELECT id FROM choices
	WHERE question_id = ?;
	`
	rows, err := h.Query(queryQuestions, surveyID)
	if err != nil {
		return nil, fmt.Errorf("failed to read survey questions: %w", err)
	}
	defer rows.Close()

	res := make(map[uuid.UUID]models.QuestionMeta)
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
		cRows, err := h.Query(queryChoices, idStr)
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
			meta.ChoiceIDs[cid] = struct{}{}
		}
		if err := cRows.Err(); err != nil {
			cRows.Close()
			return nil, fmt.Errorf("iteration error on choices: %w", err)
		}
		cRows.Close()
		res[qid] = meta
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iteration error on questions: %w", err)
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
	VALUES (?, ?, ?, ?, ?);
	`
	const insertAnswer = `
	INSERT INTO answers(id, submission_id, question_id, choice_id, text_response)
	VALUES (?, ?, ?, ?, ?);
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
	WHERE survey_id = ?
	`
	args := []any{surveyID}
	if userID != nil {
		query += " AND user_id = ?"
		args = append(args, *userID)
	}
	query += " ORDER BY submitted_at DESC;"

	rows, err := h.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query submissions: %w", err)
	}
	defer rows.Close()

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

		answers, err := listAnswersBySubmission(h, idStr)
		if err != nil {
			return nil, err
		}
		sub.Answers = answers
		res = append(res, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iteration error on submissions: %w", err)
	}

	return res, nil
}

func ListPublicSubmissionsBySurvey(h *sql.DB, surveyID string) ([]models.Submission, error) {
	const query = `
	SELECT id, survey_id, user_id, submitted_at
	FROM submissions
	WHERE survey_id = ? AND is_public = 1
	ORDER BY submitted_at DESC;
	`
	rows, err := h.Query(query, surveyID)
	if err != nil {
		return nil, fmt.Errorf("failed to query public submissions: %w", err)
	}
	defer rows.Close()

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

		answers, err := listAnswersBySubmission(h, idStr)
		if err != nil {
			return nil, err
		}
		sub.Answers = answers
		res = append(res, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iteration error on submissions: %w", err)
	}

	return res, nil
}

func ListSubmissionsByUser(h *sql.DB, userID string) ([]models.Submission, error) {
	query := `
	SELECT id, survey_id, user_id, submitted_at
	FROM submissions
	WHERE user_id = ?
	ORDER BY submitted_at DESC;
	`
	rows, err := h.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query submissions by user: %w", err)
	}
	defer rows.Close()

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

		answers, err := listAnswersBySubmission(h, idStr)
		if err != nil {
			return nil, err
		}
		sub.Answers = answers
		res = append(res, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iteration error on submissions: %w", err)
	}

	return res, nil
}

func ListPublicAnswersByQuestion(h *sql.DB, questionID string) ([]models.CatalogAnswer, error) {
	const query = `
	SELECT a.id, a.question_id, a.choice_id, a.text_response, s.survey_id, s.submitted_at
	FROM answers a
	JOIN submissions s ON s.id = a.submission_id
	WHERE a.question_id = ? AND s.is_public = 1
	ORDER BY s.submitted_at DESC;
	`
	rows, err := h.Query(query, questionID)
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
	WHERE submission_id = ?;
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
	WHERE id = ?
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
	db, err := sqlx.Connect("sqlite3", "./test.db?_foreign_keys=1")
	if err != nil {
		fmt.Println(err)
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(10)
	db.SetConnMaxIdleTime(time.Second * 30)

	err = db.Ping()
	if err != nil {
		db.Close()
		return nil, err
	}

	log.Printf("established connection to db")
	return db.DB, nil
}
