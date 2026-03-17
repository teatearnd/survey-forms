package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"sync"

	"example.com/m/internal/models"
	"github.com/google/uuid"
)

type Handler struct {
	Mu     *sync.RWMutex
	TempDB *[]models.Survey
}

func (h *Handler) DefaultHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello World!")
}

// Creates a new survey using a struct Survey
func (h *Handler) CreateSurvey(w http.ResponseWriter, r *http.Request) {
	new_survey := models.Survey{}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	err := decoder.Decode(&new_survey)
	if err != nil {
		http.Error(w, "invalid JSON request", http.StatusBadRequest)
		return
	}
	err = models.ValidateSurveyAdding(new_survey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.Mu.Lock()
	*h.TempDB = append(*h.TempDB, new_survey)
	h.Mu.Unlock()

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
		"message":    "succesfully deleted the survey",
		"deleted_id": called_survey.ID,
	}
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, "failed to encode a response", http.StatusInternalServerError)
	}
}
