package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDefaultHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	handler := http.HandlerFunc(DefaultHandler)
	handler.ServeHTTP(recorder, req)

	if status := recorder.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v, want %v", status, http.StatusOK)
	}

	expected := `"There is nothing here."` + "\n"
	if recorder.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v, want %v", recorder.Body.String(), expected)
	}
}
