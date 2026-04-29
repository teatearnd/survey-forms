package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"example.com/m/internal/dto"
	"example.com/m/internal/models"
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
);`

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

	_, err = db.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		return fmt.Errorf("failed to turn on fkeys at %w", err)
	}
	return nil
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
