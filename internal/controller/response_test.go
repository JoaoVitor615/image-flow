package controller_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JoaoVitor615/image-flow/internal/controller"
)

func TestSendSuccess(t *testing.T) {
	rec := httptest.NewRecorder()
	payload := map[string]string{"foo": "bar"}

	controller.SendSuccess(rec, http.StatusOK, payload)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %s", contentType)
	}

	var res controller.APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !res.Success {
		t.Fatalf("expected success true, got false")
	}

	dataMap, ok := res.Data.(map[string]any)
	if !ok || dataMap["foo"] != "bar" {
		t.Fatalf("unexpected data: %v", res.Data)
	}
}

func TestSendError(t *testing.T) {
	rec := httptest.NewRecorder()

	controller.SendError(rec, http.StatusBadRequest, "something went wrong")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var res controller.APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if res.Success {
		t.Fatalf("expected success false, got true")
	}

	if res.Error != "something went wrong" {
		t.Fatalf("expected error message 'something went wrong', got '%s'", res.Error)
	}
}
