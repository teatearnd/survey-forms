package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"example.com/m/internal/dto"
	"example.com/m/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RequestRegisterUser
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid JSON request", http.StatusBadRequest)
		return
	}
	if decoder.More() {
		http.Error(w, "multiple json objects/trailing junk", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password are required", http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Register: bcrypt failed: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	user := dto.ToUser(req)
	user.PasswordHash = string(hash)

	created, err := repository.InsertUser(h.DB, user)
	if err != nil {
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			http.Error(w, "username already exists", http.StatusConflict)
			return
		}
		log.Printf("Register: insert failed: %v", err)
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	response := map[string]any{
		"message":  "user successfully registered",
		"id":       created.ID,
		"username": created.Username,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Register: encode failed: %v", err)
	}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.RequestLoginUser
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid JSON request", http.StatusBadRequest)
		return
	}
	if decoder.More() {
		http.Error(w, "multiple json objects/trailing junk", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password are required", http.StatusBadRequest)
		return
	}

	user, err := repository.FindUserByUsername(h.DB, req.Username)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		log.Printf("Login: find user failed: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	claims := jwt.MapClaims{
		"sub": user.ID.String(),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(h.JWTSecret)
	if err != nil {
		log.Printf("Login: token signing failed: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(dto.ResponseLoginUser{Token: signed}); err != nil {
		log.Printf("Login: encode failed: %v", err)
	}
}
