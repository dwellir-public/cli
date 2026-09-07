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
		if err := json.NewEncoder(w).Encode([]map[string]string{{"name": "Ethereum"}}); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
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
		if err := json.NewEncoder(w).Encode(map[string]bool{"ok": true}); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
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
		if err := json.NewEncoder(w).Encode(map[string]string{"detail": "Not authenticated"}); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
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

// Marly's refresh middleware only advertises a new expiry for the token the
// client already holds; it never returns a replacement token value. The client
// must therefore keep sending the configured token and must not treat the
// expiry header as something to persist.
func TestClientKeepsConfiguredTokenWhenMarlyAdvertisesRefreshedExpiry(t *testing.T) {
	var seenAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		w.Header().Set("X-Dwellir-Refreshed-Token-Expiry", "2026-03-01T00:00:00+00:00")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "old-token")

	var result map[string]string
	if err := client.Get("/v4/user", nil, &result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seenAuth != "Bearer old-token" {
		t.Errorf("expected the configured token to be sent, got: %q", seenAuth)
	}
	if result["status"] != "ok" {
		t.Errorf("expected the response body to be decoded, got: %#v", result)
	}
}
