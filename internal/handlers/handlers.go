package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"example.com/m/internal/models"
	"example.com/m/internal/repository"
	"github.com/google/uuid"
)

type Handler struct {
	DB *sql.DB
}

func (h *Handler) DefaultHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello World!")
}

func (h *Handler) CreateSurvey(w http.ResponseWriter, r *http.Request) {
	currentDate := time.Now()
	new_survey := models.Survey{}
	decoder := json.NewDecoder(r.Body)

	decoder.DisallowUnknownFields()
	err := decoder.Decode(&new_survey)
	if err != nil {
		http.Error(w, "invalid JSON request", http.StatusBadRequest)
		return
	}
	new_survey.CreatedAt = currentDate

	err = models.ValidateSurveyAdding(new_survey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = repository.InsertSurvey(h.DB, &new_survey)
	if err != nil {
		log.Printf("CreateSurvey: insert failed: %v", err)
		http.Error(w, "failed on db inserting", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	response := map[string]any{
		"message": "survey successfully created",
		"survey":  new_survey,
	}
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) DeleteSurvey(w http.ResponseWriter, r *http.Request) {
	type delete struct {
		ID uuid.UUID `json:"id"`
	}
	called_survey := delete{}
	decoder := json.NewDecoder(r.Body)

	decoder.DisallowUnknownFields()
	err := decoder.Decode(&called_survey)
	if err != nil {
		http.Error(w, "invalid json request", http.StatusBadRequest)
		return
	}

	err = repository.DeleteSurveyByID(h.DB, called_survey.ID)
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
		"deleted_id": called_survey.ID,
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
	}
}
