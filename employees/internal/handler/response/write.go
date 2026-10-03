package response

import (
	"encoding/json"
	"log"
	"net/http"
)

func WriteHeader(w http.ResponseWriter, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
}

func WriteResponse(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}

func WriteError(w http.ResponseWriter, code int, msg string) {
	WriteResponse(w, code, ErrorResponse{
		Code:    code,
		Message: msg,
	})
}

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
