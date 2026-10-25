package controller

import (
	"encoding/json"
	"net/http"
)

// APIResponse represents the standard envelope for all API responses
type APIResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// SendJSON encodes and writes an APIResponse with the given HTTP status code
func SendJSON(w http.ResponseWriter, statusCode int, payload APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

// SendSuccess writes a successful response with optional data payload
func SendSuccess(w http.ResponseWriter, statusCode int, data any) {
	SendJSON(w, statusCode, APIResponse{
		Success: true,
		Data:    data,
	})
}

// SendError writes an error response with the specified status code and error message
func SendError(w http.ResponseWriter, statusCode int, message string) {
	SendJSON(w, statusCode, APIResponse{
		Success: false,
		Error:   message,
	})
}
