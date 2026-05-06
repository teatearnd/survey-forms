package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"example.com/m/internal/auth"
	"example.com/m/internal/dto"
	"example.com/m/internal/models"
	"example.com/m/internal/repository"
	"example.com/m/internal/validations"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	DB *sql.DB
}

func DefaultHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := "There is nothing here."
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) CreateSurvey(w http.ResponseWriter, r *http.Request) {
	new_survey := dto.RequestCreateSurvey{} // dto
	decoder := json.NewDecoder(r.Body)

	decoder.DisallowUnknownFields()
	err := validations.DecodeStrict(decoder, &new_survey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dtoResponse := dto.ToSurvey(new_survey)
	err = validations.ValidateSurveyAdding(dtoResponse)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res, err := repository.InsertSurvey(h.DB, dtoResponse)
	if err != nil {
		log.Printf("CreateSurvey: insert failed: %v", err)
		http.Error(w, "failed on db inserting", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	response := map[string]any{
		"message": "survey successfully created",
		"survey":  res,
	}
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) DeleteSurvey(w http.ResponseWriter, r *http.Request) {
	survey := chi.URLParam(r, "surveyId")
	err := validations.ValidateUuid(survey)
	if err != nil {
		http.Error(w, "bad uuid", http.StatusBadRequest)
		return
	}

	err = repository.DeleteSurveyByID(h.DB, survey)
	if err != nil {
		if errors.Is(err, repository.ErrSurveyNotFound) {
			http.Error(w, "survey not found", http.StatusNotFound)
			return
		}
		log.Printf("DeleteSurvey: failed on DeleteSurveyByID: %v", err)
		http.Error(w, "failed to delete a survey", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := map[string]any{
		"message":    "successfully deleted the survey",
		"deleted_id": survey,
	}
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, "failed to encode a response", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) GetSurveys(w http.ResponseWriter, r *http.Request) {
	res, err := repository.ListSurveys(h.DB)
	if err != nil {
		log.Printf("GetSurveys: parsing failed: %v", err)
		http.Error(w, "failed to list surveys", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		http.Error(w, "failed to encode", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) GetSingleSurvey(w http.ResponseWriter, r *http.Request) {
	survey := chi.URLParam(r, "surveyId")
	err := validations.ValidateUuid(survey)
	if err != nil {
		http.Error(w, "bad uuid", http.StatusBadRequest)
		return
	}

	res, err := repository.RetrieveSurvey(h.DB, survey)

	if err != nil {
		if errors.Is(err, repository.ErrSurveyNotFound) {
			http.Error(w, "survey not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed while interacting with the database", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		http.Error(w, "failed to encode a response", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) CreateSubmission(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := validations.ValidateUuid(claims.UserID); err != nil {
		http.Error(w, "invalid token subject", http.StatusUnauthorized)
		return
	}

	surveyID := chi.URLParam(r, "surveyId")
	if err := validations.ValidateUuid(surveyID); err != nil {
		http.Error(w, "bad uuid", http.StatusBadRequest)
		return
	}

	var req dto.RequestCreateSubmission
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := validations.DecodeStrict(decoder, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	exists, err := repository.SurveyExists(h.DB, surveyID)
	if err != nil {
		log.Printf("CreateSubmission: failed on SurveyExists: %v", err)
		http.Error(w, "failed to validate survey", http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, "survey not found", http.StatusNotFound)
		return
	}

	meta, err := repository.GetSurveyQuestionMeta(h.DB, surveyID)
	if err != nil {
		log.Printf("CreateSubmission: failed on GetSurveyQuestionMeta: %v", err)
		http.Error(w, "failed to validate submission", http.StatusInternalServerError)
		return
	}
	if err := validations.ValidateSubmissionRequest(req, meta); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	submissionID := uuid.New()
	surveyUUID, _ := uuid.Parse(surveyID)
	userUUID, _ := uuid.Parse(claims.UserID)

	sub := models.Submission{
		ID:       submissionID,
		SurveyID: surveyUUID,
		UserID:   userUUID,
		Time:     time.Now().UTC(),
		Answers:  []models.Answer{},
	}
	for _, ans := range req.Answers {
		a := models.Answer{
			ID:           uuid.New(),
			QuestionID:   ans.QuestionID,
			SubmissionID: submissionID,
			ChoiceID:     ans.ChoiceID,
			TextResponse: ans.TextResponse,
		}
		sub.Answers = append(sub.Answers, a)
	}

	created, err := repository.InsertSubmission(h.DB, sub)
	if err != nil {
		log.Printf("CreateSubmission: failed on InsertSubmission: %v", err)
		http.Error(w, "failed to create submission", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(toSubmissionResponse(created)); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) GetSubmissionsBySurvey(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	surveyID := chi.URLParam(r, "surveyId")
	if err := validations.ValidateUuid(surveyID); err != nil {
		http.Error(w, "bad uuid", http.StatusBadRequest)
		return
	}

	var userFilter *string
	if strings.TrimSpace(strings.ToLower(claims.Role)) != "admin" {
		userFilter = &claims.UserID
	}

	res, err := repository.ListSubmissionsBySurvey(h.DB, surveyID, userFilter)
	if err != nil {
		log.Printf("GetSubmissionsBySurvey: failed on ListSubmissionsBySurvey: %v", err)
		http.Error(w, "failed to list submissions", http.StatusInternalServerError)
		return
	}

	response := make([]dto.ResponseSubmission, 0, len(res))
	for _, sub := range res {
		response = append(response, toSubmissionResponse(sub))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) GetSubmissionsByUser(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	userID := chi.URLParam(r, "userId")
	if err := validations.ValidateUuid(userID); err != nil {
		http.Error(w, "bad uuid", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(strings.ToLower(claims.Role)) != "admin" && claims.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	res, err := repository.ListSubmissionsByUser(h.DB, userID)
	if err != nil {
		log.Printf("GetSubmissionsByUser: failed on ListSubmissionsByUser: %v", err)
		http.Error(w, "failed to list submissions", http.StatusInternalServerError)
		return
	}

	response := make([]dto.ResponseSubmission, 0, len(res))
	for _, sub := range res {
		response = append(response, toSubmissionResponse(sub))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

func toSubmissionResponse(sub models.Submission) dto.ResponseSubmission {
	resp := dto.ResponseSubmission{
		ID:          sub.ID,
		SurveyID:    sub.SurveyID,
		UserID:      sub.UserID,
		SubmittedAt: sub.Time,
		Answers:     make([]dto.ResponseAnswer, 0, len(sub.Answers)),
	}

	for _, ans := range sub.Answers {
		resp.Answers = append(resp.Answers, dto.ResponseAnswer{
			ID:           ans.ID,
			QuestionID:   ans.QuestionID,
			ChoiceID:     ans.ChoiceID,
			TextResponse: ans.TextResponse,
		})
	}

	return resp
}
