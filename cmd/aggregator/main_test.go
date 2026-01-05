package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleAudit(t *testing.T) {
	// Test valid POST
	auditData := `{"timestamp":"2023-01-01T00:00:00Z","operation":"ReadFile","path":"/etc/passwd"}`
	req := httptest.NewRequest(http.MethodPost, "/audit", bytes.NewBufferString(auditData))
	w := httptest.NewRecorder()

	handleAudit(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandleAuditInvalidMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/audit", nil)
	w := httptest.NewRecorder()

	handleAudit(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleAuditInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/audit", bytes.NewBufferString("invalid json"))
	w := httptest.NewRecorder()

	handleAudit(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}
