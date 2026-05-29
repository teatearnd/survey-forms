package main

import (
	"log"
	"net/http"
	"os"

	"example.com/m/internal/auth"
	"example.com/m/internal/cache"
	"example.com/m/internal/config"
	"example.com/m/internal/handlers"
	"example.com/m/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf(".env not loaded: %v", err)
	}

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

	// redis init
	redisAddr := os.Getenv("REDIS_ADDRESS")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisPass := os.Getenv("REDIS_PASSWORD")
	redisDB := 0
	cacheClient := cache.NewRedisCache(redisAddr, redisPass, redisDB)
	if err := cacheClient.Ping(); err != nil {
		log.Printf("[REDIS IS DOWN] failed to ping redis: %v", err) // should it hard-fail?
	}
	cfg := config.LoadConfig()
	authInit := auth.Settings{
		Secret:   os.Getenv("JWT_SECRET"),
		Issuer:   os.Getenv("JWT_ISSUER"),
		Audience: os.Getenv("JWT_AUDIENCE"),
	}
	allowedDomains := config.ParseDomains(cfg.AllowedEmails)
	if err := auth.Init(authInit); err != nil {
		log.Fatalf("JWT init failed: %v", err)
	}
	if err := auth.ValidateConfig(); err != nil {
		log.Fatalf("JWT config invalid: %v", err)
	}
	def_handler := &handlers.Handler{DB: db, Cache: cacheClient, AllowedDomains: allowedDomains}

	r.Use(middleware.Logger)
	r.Get("/", handlers.DefaultHandler)
	r.Get("/surveys", def_handler.GetSurveys)
	r.Get("/survey/{surveyId}", def_handler.GetSingleSurvey)

	// auth
	r.Post("/login", def_handler.LoginHandler)
	r.Post("/register", def_handler.RegisterHandler)

	r.Group(func(r chi.Router) {
		r.Use(auth.AuthMiddleware)
		r.Post("/survey/{surveyId}/submissions", def_handler.CreateSubmission)
		r.Get("/survey/{surveyId}/submissions", def_handler.GetSubmissionsBySurvey)
		r.Get("/users/{userId}/submissions", def_handler.GetSubmissionsByUser)
		r.Get("/catalog/surveys/{surveyId}/submissions", def_handler.GetPublicSubmissionsBySurvey)
		r.Get("/catalog/questions/{questionId}/answers", def_handler.GetPublicAnswersByQuestion)
		r.Post("/survey", def_handler.CreateSurvey)
		r.Delete("/survey/{surveyId}", def_handler.DeleteSurvey)
		// Cart endpoints
		r.Post("/cart/items", def_handler.AddToCart)
		r.Get("/cart", def_handler.GetCart)
		r.Delete("/cart/items/{index}", def_handler.RemoveFromCart)
		r.Delete("/cart", def_handler.ClearCart)
	})

	log.Printf("starting server on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
