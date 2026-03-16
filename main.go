package main

import (
	"fmt"
	"net/http"
)

func defaultHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello World!")
}

// Creates a new survey using a struct Survey
func createSurvey(w http.ResponseWriter, r *http.Request) {

}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", defaultHandler)

	http.ListenAndServe(":8080", mux)
	fmt.Printf("Server should be running on 8080 port now.\n")
}
