package main

import (
	"log"
	"net/http"

	"example.com/m/internal/handlers"
	"example.com/m/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	r := chi.NewRouter()
	db, err := repository.OpenDB()
	if err != nil {
		log.Fatalf("failed at db open: %v", err)
	}
	defer db.Close()
	err = repository.InitSchema(db)
	if err != nil {
		log.Fatalf("failed at db initialization: %v", err)
	}

	def_handler := &handlers.Handler{DB: db}

	r.Use(middleware.Logger)
	r.Get("/", handlers.DefaultHandler)
	r.Get("/surveys", def_handler.GetSurveys)
	r.Post("/surveys", def_handler.CreateSurvey)
	r.Get("/survey/{surveyId}", def_handler.GetSingleSurvey)
	r.Delete("/survey/{surveyId}", def_handler.DeleteSurvey)

	log.Printf("starting server on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
