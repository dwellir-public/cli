package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewLoginMux_CallbackRejectsMissingToken(t *testing.T) {
	resultCh := make(chan *CallbackPayload, 1)
	errCh := make(chan error, 1)
	mux := newLoginMux("https://dashboard.dwellir.com", resultCh, errCh)

	payload := map[string]string{"org": "orderbook", "user": "elias"}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/callback", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	select {
	case <-resultCh:
		t.Fatal("unexpected callback result for missing token")
	default:
	}

	select {
	case <-errCh:
		// expected
	default:
		t.Fatal("expected callback error for missing token")
	}
}

func TestNewLoginMux_CallbackAcceptsValidToken(t *testing.T) {
	resultCh := make(chan *CallbackPayload, 1)
	errCh := make(chan error, 1)
	mux := newLoginMux("https://dashboard.dwellir.com", resultCh, errCh)

	payload := map[string]string{"token": "abc123", "org": "orderbook", "user": "elias"}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/callback", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	select {
	case got := <-resultCh:
		if got == nil || got.Token != "abc123" {
			t.Fatalf("unexpected callback payload: %#v", got)
		}
	default:
		t.Fatal("expected callback result for valid token")
	}

	select {
	case err := <-errCh:
		t.Fatalf("unexpected callback error: %v", err)
	default:
	}
}
