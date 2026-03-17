package main

import (
	"fmt"
	"net/http"
	"sync"

	"example.com/m/internal/handlers"
	"example.com/m/internal/models"
)

var tempDB = []models.Survey{}
var def_handler = handlers.Handler{TempDB: &tempDB, Mu: &sync.RWMutex{}}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", def_handler.DefaultHandler)
	mux.HandleFunc("POST /surveys", def_handler.CreateSurvey)
	mux.HandleFunc("DELETE /surveys", def_handler.DeleteSurvey)

	fmt.Printf("Server should be running on 8080 port now.\n")
	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		panic(err)
	}
}
