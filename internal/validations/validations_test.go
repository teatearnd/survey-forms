package validations

import (
	"testing"

	"example.com/m/internal/models"
)

func TestValidateSurvey(t *testing.T) {
	baseSurvey := models.Survey{
		Name:        "Normal name",
		Description: "Normal description",
		Questions_list: []models.Question{
			{
				Description: "Normal Description of a Question",
				Type:        1,
				IsMandatory: true,
			},
		},
	}

	tests := map[string]struct {
		mutate  func(*models.Survey)
		wantErr string
	}{
		"valid survey": {
			mutate:  func(s *models.Survey) {},
			wantErr: "",
		},
		"survey name is blank": {
			mutate: func(s *models.Survey) {
				s.Name = " "
			},
			wantErr: "survey name cannot be empty",
		},
		"no question survey": {
			mutate: func(s *models.Survey) {
				s.Questions_list = []models.Question{}
			},
			wantErr: "no questions found",
		},
		"question description is blank": {
			mutate: func(s *models.Survey) {
				s.Questions_list = []models.Question{
					{
						Description: " ",
					},
				}
			},
			wantErr: "questions_list[0] has no description",
		},
		"question type is not of type 0 or 1": {
			mutate: func(s *models.Survey) {
				s.Questions_list = []models.Question{
					{
						Description: "Normal Description of a question",
						Type:        3,
					},
				}
			},
			wantErr: "questions_list[0] has an incorrect question type",
		},
		"question is multiple choice but has no choices": {
			mutate: func(s *models.Survey) {
				s.Questions_list = []models.Question{
					{
						Description: "Normal Description of a question",
						Type:        0,
						Choices:     []models.Answer_choice{},
					},
				}
			},
			wantErr: "questions_list[0] with property MultipleChoice, but no choices present",
		},
		"question is text based but choices are present": {
			mutate: func(s *models.Survey) {
				s.Questions_list = []models.Question{
					{
						Description: "Normal Description of a question",
						Type:        1,
						Choices: []models.Answer_choice{
							{
								Description: "Answer Choice",
							},
						},
					},
				}
			},
			wantErr: "questions_list[0] with property TextBased is not allowed to have choices",
		},
		"question is multiple choice but description of a choice is blank": {
			mutate: func(s *models.Survey) {
				s.Questions_list = []models.Question{
					{
						Description: "Normal Description of a question",
						Type:        0,
						Choices: []models.Answer_choice{
							{
								Description: "Next question will be blank",
							},
							{
								Description: " ",
							},
						},
					},
				}
			},
			wantErr: "choice 1 is empty at questions_list[0]",
		},
	}

	for caseName, testcase := range tests {
		t.Run(caseName, func(t *testing.T) {
			s := baseSurvey
			testcase.mutate(&s)

			err := ValidateSurveyAdding(s)

			if testcase.wantErr == "" {
				if err != nil {
					t.Fatalf("got err %v, expected a success", err)
				}
				return
			}

			if err.Error() != testcase.wantErr {
				t.Fatalf("got err %v, want %q", err, testcase.wantErr)
			}
		})
	}
}
