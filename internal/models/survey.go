package models

import (
	"time"

	"github.com/google/uuid"
)

// Foundation
type Survey struct {
	ID             uuid.UUID  `json:"id"`
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	Questions_list []Question `json:"questions_list"`
	CreatedAt      time.Time  `json:"created_at"`
}

type QuestionType int

const (
	MultipleChoice QuestionType = iota // 0
	TextBased                          // 1
)

// 4th member of a Survey struct
type Question struct {
	Description string          `json:"description"`
	Type        QuestionType    `json:"type"`
	Choices     []Answer_choice `json:"choices,omitempty"`
	IsMandatory bool            `json:"is_mandatory"`
	ID          uuid.UUID       `json:"id"`
	SurveyID    uuid.UUID       `json:"survey_id"`
}

// 3rd member of a Question struct
type Answer_choice struct {
	ID          uuid.UUID `json:"id"`
	Description string    `json:"description"`
}

type Submission struct {
	ID       uuid.UUID `json:"id"`
	SurveyID uuid.UUID `json:"survey_id"`
	UserID   uuid.UUID `json:"user_id"`
	Answers  []Answer  `json:"answers"`
	IsPublic bool      `json:"is_public"`
	Time     time.Time `json:"submitted_at"`
}

type Answer struct {
	ID           uuid.UUID  `json:"id"`
	QuestionID   uuid.UUID  `json:"question_id"`
	SubmissionID uuid.UUID  `json:"submission_id"`
	ChoiceID     *uuid.UUID `json:"choice_id,omitempty"`
	TextResponse string     `json:"text_response"`
}

type QuestionMeta struct {
	ID          uuid.UUID
	Type        QuestionType
	IsMandatory bool
	ChoiceIDs   map[uuid.UUID]struct{}
}

type CatalogAnswer struct {
	ID           uuid.UUID
	QuestionID   uuid.UUID
	ChoiceID     *uuid.UUID
	TextResponse string
	SurveyID     uuid.UUID
	SubmittedAt  time.Time
}
