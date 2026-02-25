package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer token, got: %s", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/v3/chains" {
			t.Errorf("expected /v3/chains, got: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]map[string]string{{"name": "Ethereum"}})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	var result []map[string]string
	err := client.Get("/v3/chains", nil, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0]["name"] != "Ethereum" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestClientPost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got: %s", r.Method)
		}
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	var result map[string]bool
	err := client.Post("/v4/organization/analytics", map[string]string{"interval": "day"}, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result["ok"] {
		t.Error("expected ok: true")
	}
}

func TestClientUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"detail": "Not authenticated"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "bad-token")
	var result map[string]string
	err := client.Get("/v4/user", nil, &result)
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got: %T", err)
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("expected 401, got: %d", apiErr.StatusCode)
	}
}

func TestClientTokenRefresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Dwellir-Refreshed-Token", "new-token-456")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	var refreshedToken string
	client := NewClient(server.URL, "old-token")
	client.OnTokenRefresh = func(newToken string) {
		refreshedToken = newToken
	}

	var result map[string]string
	client.Get("/v4/user", nil, &result)
	if refreshedToken != "new-token-456" {
		t.Errorf("expected refreshed token, got: '%s'", refreshedToken)
	}
}
