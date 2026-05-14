package dto

import (
	"time"

	"example.com/m/internal/models"
	"github.com/google/uuid"
)

type RequestLookupSurvey struct {
	ID uuid.UUID `json:"id"`
}

type RequestCreateSurvey struct {
	Name           string                  `json:"name"`
	Description    string                  `json:"description"`
	Questions_list []RequestCreateQuestion `json:"questions_list"`
}

type RequestSurvey struct {
	ID             uuid.UUID         `json:"id"`
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	Questions_list []RequestQuestion `json:"questions_list"`
	CreatedAt      time.Time         `json:"created_at"`
}
type RequestQuestion struct {
	Description string                 `json:"description"`
	Type        models.QuestionType    `json:"type"`
	Choices     []models.Answer_choice `json:"choices,omitempty"`
	IsMandatory bool                   `json:"is_mandatory"`
}

type RequestCreateQuestion struct {
	Description string                 `json:"description"`
	Type        models.QuestionType    `json:"type"`
	Choices     []models.Answer_choice `json:"choices,omitempty"`
	IsMandatory bool                   `json:"is_mandatory"`
}

func ToSurvey(req RequestCreateSurvey) models.Survey {
	survey := models.Survey{
		ID:          uuid.New(),
		Name:        req.Name,
		Description: req.Description,
		CreatedAt:   time.Now().UTC(),
	}

	for _, q := range req.Questions_list {
		mq := models.Question{
			ID:          uuid.New(),
			SurveyID:    survey.ID,
			Description: q.Description,
			Type:        q.Type,
			IsMandatory: q.IsMandatory,
		}
		for _, c := range q.Choices {
			mq.Choices = append(mq.Choices, models.Answer_choice{
				ID:          uuid.New(),
				Description: c.Description,
			})
		}
		survey.Questions_list = append(survey.Questions_list, mq)
	}
	return survey
}

type ResponseGetSurveys struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

func GetSurveys(response models.Survey) ResponseGetSurveys {
	r := ResponseGetSurveys{
		ID:          response.ID,
		Name:        response.Name,
		Description: response.Description,
		CreatedAt:   response.CreatedAt,
	}
	return r
}

type RequestCreateSubmission struct {
	Answers []RequestCreateAnswer `json:"answers"`
}

type RequestCreateAnswer struct {
	QuestionID   uuid.UUID  `json:"question_id"`
	ChoiceID     *uuid.UUID `json:"choice_id,omitempty"`
	TextResponse string     `json:"text_response,omitempty"`
}

type ResponseSubmission struct {
	ID          uuid.UUID        `json:"id"`
	SurveyID    uuid.UUID        `json:"survey_id"`
	UserID      uuid.UUID        `json:"user_id"`
	Answers     []ResponseAnswer `json:"answers"`
	SubmittedAt time.Time        `json:"submitted_at"`
}

type ResponseAnswer struct {
	ID           uuid.UUID  `json:"id"`
	QuestionID   uuid.UUID  `json:"question_id"`
	ChoiceID     *uuid.UUID `json:"choice_id,omitempty"`
	TextResponse string     `json:"text_response,omitempty"`
}

type ResponseCatalogSubmission struct {
	ID          uuid.UUID               `json:"id"`
	SurveyID    uuid.UUID               `json:"survey_id"`
	Answers     []ResponseCatalogAnswer `json:"answers"`
	SubmittedAt time.Time               `json:"submitted_at"`
}

type ResponseCatalogAnswer struct {
	ID           uuid.UUID  `json:"id"`
	QuestionID   uuid.UUID  `json:"question_id"`
	ChoiceID     *uuid.UUID `json:"choice_id,omitempty"`
	TextResponse string     `json:"text_response,omitempty"`
}

type ResponseCatalogQuestionAnswer struct {
	ID           uuid.UUID  `json:"id"`
	QuestionID   uuid.UUID  `json:"question_id"`
	ChoiceID     *uuid.UUID `json:"choice_id,omitempty"`
	TextResponse string     `json:"text_response,omitempty"`
	SurveyID     uuid.UUID  `json:"survey_id"`
	SubmittedAt  time.Time  `json:"submitted_at"`
}

// to send the object to the cart
type RequestCartObject struct {
	// stored as-is (JSON) in Redis
	Item map[string]any `json:"item"`
}

// to receive an array of the cart
type ResponseCart struct {
	Cart []map[string]any `json:"cart"`
}
