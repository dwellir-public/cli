package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListKeys(t *testing.T) {
	keys := []APIKey{
		{APIKey: "abc-123", Name: "test-key", Enabled: true},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/user/apikeys" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(keys)
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	ka := NewKeysAPI(client)
	result, err := ka.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 key, got %d", len(result))
	}
	if result[0].Name != "test-key" {
		t.Errorf("expected test-key, got %s", result[0].Name)
	}
}

func TestCreateKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var input CreateKeyInput
		json.NewDecoder(r.Body).Decode(&input)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(APIKey{APIKey: "new-key", Name: input.Name, Enabled: true})
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	ka := NewKeysAPI(client)
	result, err := ka.Create(CreateKeyInput{Name: "my-key"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "my-key" {
		t.Errorf("expected my-key, got %s", result.Name)
	}
}
