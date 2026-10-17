package response

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

func WriteResponse(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("write response", "error", err)
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
