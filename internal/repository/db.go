package repository

import (
	"database/sql"
	"fmt"
	"log"

	"example.com/m/internal/models"
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
    content TEXT,
    FOREIGN KEY(survey_id) REFERENCES surveys(id)
);`

func OpenDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "./my.db")
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		db.Close()
		return nil, err
	}

	log.Printf("established connection to db")
	return db, nil
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

func InsertSurvey(h *sql.DB, survey *models.Survey) error {
	tx, err := h.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	const inserting_surveys = `
	INSERT INTO surveys(id, title, created_at)
	VALUES (?, ?, ?);
	`
	const inserting_questions = `
	INSERT INTO questions(id, survey_id, content)
	VALUES (?, ?, ?);
	`

	_, err = tx.Exec(inserting_surveys, survey.ID, survey.Name, survey.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed at inserting surveys %s into the db: %w", survey.ID, err)
	}
	for _, j := range survey.Questions_list {
		_, err = tx.Exec(inserting_questions, j.ID, j.SurveyID, j.Description)
		if err != nil {
			return fmt.Errorf("failed while inserting question %s %v", j.ID, err)
		}
	}
	return tx.Commit()
}
