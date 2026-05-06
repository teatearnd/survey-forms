package validations

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"example.com/m/internal/dto"
	"example.com/m/internal/models"
	"github.com/google/uuid"
)

func ValidateSurveyAdding(s models.Survey) error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("survey name cannot be empty")
	}
	if len(s.Questions_list) == 0 {
		return fmt.Errorf("no questions found")
	}

	for i, q := range s.Questions_list {
		if strings.TrimSpace(q.Description) == "" {
			return fmt.Errorf("questions_list[%d] has no description", i)
		}
		if q.Type != models.MultipleChoice && q.Type != models.TextBased {
			return fmt.Errorf("questions_list[%d] has an incorrect question type", i)
		}
		if q.Type == models.MultipleChoice && len(q.Choices) == 0 {
			return fmt.Errorf("questions_list[%d] with property MultipleChoice, but no choices present", i)
		}
		if q.Type == models.TextBased && len(q.Choices) > 0 {
			return fmt.Errorf("questions_list[%d] with property TextBased is not allowed to have choices", i)
		}

		for j, choice := range q.Choices {
			if strings.TrimSpace(choice.Description) == "" {
				return fmt.Errorf("choice %d is empty at questions_list[%d]", j, i)
			}
		}
	}
	return nil
}

func ValidateUuid(id string) error {
	if err := uuid.Validate(id); err != nil {
		return fmt.Errorf("failed on validating an ID: %s", id)
	}
	return nil
}

// Instead of checking with decode.More() we check the next non-whitespace character with Token() to find the trailing data
// Use this function when decoding requests
func DecodeStrict(decoder *json.Decoder, v any) error {
	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("failed while decoding the request: %w", err)
	}
	_, err := decoder.Token()
	if err != nil {
		if err != io.EOF {
			return fmt.Errorf("unexpected trailing data after JSON request: %w", err)
		}
	}
	return nil
}

func ValidateSubmissionRequest(req dto.RequestCreateSubmission, meta map[uuid.UUID]models.QuestionMeta) error {
	if len(req.Answers) == 0 {
		return fmt.Errorf("no answers provided")
	}

	answered := make(map[uuid.UUID]struct{}, len(req.Answers))
	for i, ans := range req.Answers {
		if ans.QuestionID == uuid.Nil {
			return fmt.Errorf("answers[%d] has an empty question_id", i)
		}
		qm, ok := meta[ans.QuestionID]
		if !ok {
			return fmt.Errorf("answers[%d] references an unknown question", i)
		}
		if _, exists := answered[ans.QuestionID]; exists {
			return fmt.Errorf("answers[%d] duplicates a question", i)
		}

		switch qm.Type {
		case models.MultipleChoice:
			if ans.ChoiceID == nil {
				return fmt.Errorf("answers[%d] missing choice_id for multiple choice", i)
			}
			if _, ok := qm.ChoiceIDs[*ans.ChoiceID]; !ok {
				return fmt.Errorf("answers[%d] has invalid choice_id", i)
			}
			if strings.TrimSpace(ans.TextResponse) != "" {
				return fmt.Errorf("answers[%d] must not include text_response for multiple choice", i)
			}
		case models.TextBased:
			if ans.ChoiceID != nil {
				return fmt.Errorf("answers[%d] must not include choice_id for text questions", i)
			}
			if strings.TrimSpace(ans.TextResponse) == "" {
				return fmt.Errorf("answers[%d] missing text_response", i)
			}
		default:
			return fmt.Errorf("answers[%d] has invalid question type", i)
		}

		answered[ans.QuestionID] = struct{}{}
	}

	for id, q := range meta {
		if q.IsMandatory {
			if _, ok := answered[id]; !ok {
				return fmt.Errorf("missing mandatory answer for question %s", id)
			}
		}
	}

	return nil
}
