package main

import (
	"log"
	"net/http"
	"sync"

	"example.com/m/internal/handlers"
	"example.com/m/internal/models"
	"example.com/m/internal/repository"
)

var tempDB = []models.Survey{}

func main() {
	mux := http.NewServeMux()
	db, err := repository.OpenDB()
	if err != nil {
		panic(err)
	}
	defer db.Close()
	err = repository.InitSchema(db)
	if err != nil {
		log.Fatalf("failed at db initialization: %v", err)
	}

	def_handler := &handlers.Handler{Mu: &sync.RWMutex{}, DB: db, TempDB: &tempDB}

	mux.HandleFunc("/", def_handler.DefaultHandler)

	// Single path for /surveys; dispatch by HTTP method inside handler
	mux.HandleFunc("/surveys", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			def_handler.CreateSurvey(w, r)
		case http.MethodGet:
			def_handler.GetSurveys(w, r)
		case http.MethodDelete:
			def_handler.DeleteSurvey(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	log.Printf("starting server on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
