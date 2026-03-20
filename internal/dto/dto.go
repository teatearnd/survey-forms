package dto

import (
	"time"

	"example.com/m/internal/models"
	"github.com/google/uuid"
)

type RequestCreateSurvey struct {
	Name           string                  `json:"name"`
	Description    string                  `json:"description"`
	Questions_list []RequestCreateQuestion `json:"questions_list"`
}

type RequestCreateQuestion struct {
	Description string                `json:"description"`
	Type        models.QuestionType   `json:"type"`
	Choices     []RequestCreateChoice `json:"choices,omitempty"`
	IsMandatory bool                  `json:"is_mandatory"`
}

type RequestCreateChoice struct {
	Description string `json:"description"`
}

func ToSurvey(req RequestCreateSurvey) models.Survey {
	s := models.Survey{
		ID:          uuid.New(),
		Name:        req.Name,
		Description: req.Description,
		CreatedAt:   time.Now(),
	}

	for _, q := range req.Questions_list {
		mq := models.Question{
			ID:          uuid.New(),
			SurveyID:    s.ID,
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
		s.Questions_list = append(s.Questions_list, mq)
	}
	return s
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

// User auth DTOs

type RequestRegisterUser struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type RequestLoginUser struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type ResponseLoginUser struct {
	Token string `json:"token"`
}

func ToUser(req RequestRegisterUser) models.User {
	return models.User{
		ID:        uuid.New(),
		Username:  req.Username,
		CreatedAt: time.Now(),
	}
}
