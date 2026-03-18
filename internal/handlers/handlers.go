package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"time"

	"example.com/m/internal/models"
	"example.com/m/internal/repository"
	"github.com/google/uuid"
)

type Handler struct {
	Mu     *sync.RWMutex
	TempDB *[]models.Survey
	DB     *sql.DB
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
	err := decoder.Decode(&called_survey)
	if err != nil {
		http.Error(w, "invalid json request", http.StatusBadRequest)
		return
	}
	found := false
	// temporary lookup at db (slice)
	h.Mu.Lock()
	defer h.Mu.Unlock()
	for i, j := range *h.TempDB {
		if called_survey.ID == j.ID {
			// found corresponding and deleting
			*h.TempDB = slices.Delete(*h.TempDB, i, i+1)
			found = true
			break
		}
	}
	if found == false {
		http.Error(w, "Couldn't find the survey", http.StatusNotFound)
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
	}
}

func (h *Handler) GetSurveys(w http.ResponseWriter, r *http.Request) {
	h.Mu.RLock()
	defer h.Mu.RUnlock()
	if len(*h.TempDB) == 0 {
		http.Error(w, "nothing to display", http.StatusNotFound)
		return
	}
	surveys := *h.TempDB

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err := json.NewEncoder(w).Encode(surveys)
	if err != nil {
		http.Error(w, "failed to encode", http.StatusInternalServerError)
	}
}
