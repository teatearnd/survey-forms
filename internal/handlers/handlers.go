package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"example.com/m/internal/dto"
	"example.com/m/internal/repository"
	"example.com/m/internal/validations"
	"github.com/go-chi/chi/v5"
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
