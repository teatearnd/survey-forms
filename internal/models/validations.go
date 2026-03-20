package models

import (
	"fmt"
	"strings"
)

func ValidateSurveyAdding(s Survey) error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if len(s.Questions_list) == 0 {
		return fmt.Errorf("no questions found")
	}

	for i, q := range s.Questions_list {
		if strings.TrimSpace(q.Description) == "" {
			return fmt.Errorf("questions_list[%d] has no description", i)
		}
		if q.Type != MultipleChoice && q.Type != TextBased {
			return fmt.Errorf("questions_list[%d] has an incorrect question type", i)
		}
		if q.Type == MultipleChoice && len(q.Choices) == 0 {
			return fmt.Errorf("questions_list[%d] with property MultipleChoice, but no choices present", i)
		}
		if q.Type == TextBased && len(q.Choices) > 0 {
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
