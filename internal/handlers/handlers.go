package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"example.com/m/internal/auth"
	"example.com/m/internal/cache"
	"example.com/m/internal/dto"
	"example.com/m/internal/models"
	"example.com/m/internal/repository"
	"example.com/m/internal/validations"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	DB    *sql.DB
	Cache *cache.RedisCache
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
		IsPublic: true,
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

func (h *Handler) GetPublicSubmissionsBySurvey(w http.ResponseWriter, r *http.Request) {
	surveyID := chi.URLParam(r, "surveyId")
	if err := validations.ValidateUuid(surveyID); err != nil {
		http.Error(w, "bad uuid", http.StatusBadRequest)
		return
	}

	limit, offset := parsePaginationParams(r)

	exists, err := repository.SurveyExists(h.DB, surveyID)
	if err != nil {
		log.Printf("GetPublicSubmissionsBySurvey: failed on SurveyExists: %v", err)
		http.Error(w, "failed to validate survey", http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, "survey not found", http.StatusNotFound)
		return
	}

	res, err := repository.ListPublicSubmissionsBySurvey(h.DB, surveyID, limit, offset)
	if err != nil {
		log.Printf("GetPublicSubmissionsBySurvey: failed on ListPublicSubmissionsBySurvey: %v", err)
		http.Error(w, "failed to list submissions", http.StatusInternalServerError)
		return
	}

	response := make([]dto.ResponseCatalogSubmission, 0, len(res))
	for _, sub := range res {
		response = append(response, toCatalogSubmissionResponse(sub))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) GetPublicAnswersByQuestion(w http.ResponseWriter, r *http.Request) {
	questionID := chi.URLParam(r, "questionId")
	if err := validations.ValidateUuid(questionID); err != nil {
		http.Error(w, "bad uuid", http.StatusBadRequest)
		return
	}

	limit, offset := parsePaginationParams(r)

	exists, err := repository.QuestionExists(h.DB, questionID)
	if err != nil {
		log.Printf("GetPublicAnswersByQuestion: failed on QuestionExists: %v", err)
		http.Error(w, "failed to validate question", http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, "question not found", http.StatusNotFound)
		return
	}

	res, err := repository.ListPublicAnswersByQuestion(h.DB, questionID, limit, offset)
	if err != nil {
		log.Printf("GetPublicAnswersByQuestion: failed on ListPublicAnswersByQuestion: %v", err)
		http.Error(w, "failed to list answers", http.StatusInternalServerError)
		return
	}

	response := make([]dto.ResponseCatalogQuestionAnswer, 0, len(res))
	for _, ans := range res {
		response = append(response, toCatalogQuestionAnswerResponse(ans))
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

func toCatalogSubmissionResponse(sub models.Submission) dto.ResponseCatalogSubmission {
	resp := dto.ResponseCatalogSubmission{
		ID:          sub.ID,
		SurveyID:    sub.SurveyID,
		SubmittedAt: sub.Time,
		Answers:     make([]dto.ResponseCatalogAnswer, 0, len(sub.Answers)),
	}

	for _, ans := range sub.Answers {
		resp.Answers = append(resp.Answers, dto.ResponseCatalogAnswer{
			ID:           ans.ID,
			QuestionID:   ans.QuestionID,
			ChoiceID:     ans.ChoiceID,
			TextResponse: ans.TextResponse,
		})
	}

	return resp
}

func toCatalogQuestionAnswerResponse(ans models.CatalogAnswer) dto.ResponseCatalogQuestionAnswer {
	return dto.ResponseCatalogQuestionAnswer{
		ID:           ans.ID,
		QuestionID:   ans.QuestionID,
		ChoiceID:     ans.ChoiceID,
		TextResponse: ans.TextResponse,
		SurveyID:     ans.SurveyID,
		SubmittedAt:  ans.SubmittedAt,
	}
}

func parsePaginationParams(r *http.Request) (limit, offset int) {
	limit = 50
	offset = 0

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	return limit, offset
}

// Cart Handlers
func (h *Handler) AddToCart(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := validations.ValidateUuid(claims.UserID); err != nil {
		http.Error(w, "invalid token subject", http.StatusUnauthorized)
		return
	}

	var req dto.RequestCartObject
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := validations.DecodeStrict(dec, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	b, err := json.Marshal(req.Item)
	if err != nil {
		http.Error(w, "failed to marshal payload", http.StatusInternalServerError)
		return
	}
	if err := h.Cache.AddItem(claims.UserID, string(b)); err != nil {
		log.Printf("AddToCart: cache add failed: %v", err)
		http.Error(w, "failed to add to cart", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) GetCart(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := validations.ValidateUuid(claims.UserID); err != nil {
		http.Error(w, "invalid token subject", http.StatusUnauthorized)
		return
	}

	limit, offset := parsePaginationParams(r)
	items, err := h.Cache.GetItems(claims.UserID, limit, offset)
	if err != nil {
		log.Printf("GetCart: failed to get items: %v", err)
		http.Error(w, "failed to get cart", http.StatusInternalServerError)
		return
	}

	resp := dto.ResponseCart{Cart: make([]dto.CartItem, 0, len(items))}
	for _, it := range items {
		var m dto.CartItem
		if err := json.Unmarshal([]byte(it), &m); err != nil {
			log.Printf("GetCart: failed to decode cart item: %v", err)
			http.Error(w, "failed to decode cart item", http.StatusInternalServerError)
			return
		}
		resp.Cart = append(resp.Cart, m)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) RemoveFromCart(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := validations.ValidateUuid(claims.UserID); err != nil {
		http.Error(w, "invalid token subject", http.StatusUnauthorized)
		return
	}

	idxStr := chi.URLParam(r, "index")
	idx, err := strconv.Atoi(idxStr)
	if err != nil || idx < 0 {
		http.Error(w, "bad index", http.StatusBadRequest)
		return
	}

	if err := h.Cache.RemoveItemByIndex(claims.UserID, idx); err != nil {
		log.Printf("RemoveFromCart: failed: %v", err)
		http.Error(w, "failed to remove item", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) ClearCart(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := validations.ValidateUuid(claims.UserID); err != nil {
		http.Error(w, "invalid token subject", http.StatusUnauthorized)
		return
	}

	if err := h.Cache.ClearCart(claims.UserID); err != nil {
		log.Printf("ClearCart: failed: %v", err)
		http.Error(w, "failed to clear cart", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
