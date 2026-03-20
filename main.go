package main

import (
	"log"
	"net/http"
	"os"

	"example.com/m/internal/handlers"
	"example.com/m/internal/repository"
)

func main() {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable must be set")
	}

	mux := http.NewServeMux()
	db, err := repository.OpenDB()
	if err != nil {
		log.Fatalf("failed at db open: %v", err)
	}
	defer db.Close()
	err = repository.InitSchema(db)
	if err != nil {
		log.Fatalf("failed at db initialization: %v", err)
	}

	def_handler := &handlers.Handler{
		DB:        db,
		JWTSecret: []byte(jwtSecret),
	}

	mux.HandleFunc("/", def_handler.DefaultHandler)

	// Auth routes (public)
	mux.HandleFunc("/users/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		def_handler.Register(w, r)
	})

	mux.HandleFunc("/users/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		def_handler.Login(w, r)
	})

	// Single path for /surveys (protected)
	mux.HandleFunc("/surveys", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			def_handler.AuthMiddleware(def_handler.CreateSurvey)(w, r)
		case http.MethodGet:
			def_handler.AuthMiddleware(def_handler.GetSurveys)(w, r)
		case http.MethodDelete:
			def_handler.AuthMiddleware(def_handler.DeleteSurvey)(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/surveys/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			def_handler.AuthMiddleware(def_handler.CreateSurvey)(w, r)
		case http.MethodGet:
			def_handler.AuthMiddleware(def_handler.GetSurveys)(w, r)
		case http.MethodDelete:
			def_handler.AuthMiddleware(def_handler.DeleteSurvey)(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	log.Printf("starting server on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}


